/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

// Package reverse implements the agent side of the reverse channel: the
// agent — typically behind NAT/firewall and NOT directly reachable —
// dials OUT to the config manager's websocket endpoint and then serves
// the manager's HTTP requests THROUGH that connection (executed as
// loopback calls against the agent's own web port).
package reverse

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/cihub/seelog"
	"github.com/gorilla/websocket"

	"infini.sh/framework/core/api/websocket/reverse"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
	common "infini.sh/framework/modules/configs/common"
)

const (
	reconnectDelay  = 5 * time.Second
	maxMessageBytes = 8 * 1024 * 1024
	requestDeadline = 30 * time.Second
)

var (
	writeMu sync.Mutex
	started sync.Once
)

// Setup launches the reverse-channel loop (idempotent). The agent main
// calls this; it only dials out when configs.managed is on.
func Setup() {
	started.Do(func() {
		if !global.Env().SystemConfig.Configs.Managed {
			return
		}
		go run()
	})
}

func run() {
	for !global.ShuttingDown() {
		if err := connectAndServe(); err != nil && !global.ShuttingDown() {
			log.Debugf("agent reverse channel: %v (retrying in %v)", err, reconnectDelay)
		}
		if global.ShuttingDown() {
			return
		}
		time.Sleep(reconnectDelay)
	}
}

func connectAndServe() error {
	servers := global.Env().SystemConfig.Configs.Servers
	var lastErr error
	for _, server := range servers {
		conn, err := dial(server)
		if err != nil {
			lastErr = err
			continue
		}
		log.Debugf("agent reverse channel connected to [%s]", server)
		err = serve(conn)
		_ = conn.Close()
		log.Warnf("agent reverse channel disconnected from [%s]: %v", server, err)
		return err
	}
	if lastErr != nil {
		return lastErr
	}
	return nil
}

// dial opens the websocket to the manager's /ws endpoint, carrying the
// instance ID and the manager credential (same auth as sync).
func dial(server string) (*websocket.Conn, error) {
	wsURL, err := reverseURL(server)
	if err != nil {
		return nil, err
	}
	headers := http.Header{}
	headers.Set(reverse.HeaderPeerID, global.Env().SystemConfig.NodeConfig.ID)
	if tok := global.Env().SystemConfig.Configs.ManagerConfig.AccessToken.Get(); tok != "" {
		headers.Set("Authorization", "Bearer "+tok)
	} else if tok, _ := common.LoadTokenFromKeystore(common.ManagerTokenKeystoreKey); tok != "" {
		headers.Set("Authorization", "Bearer "+strings.TrimSpace(tok))
	}
	dialer := &websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.Dial(wsURL, headers)
	return conn, err
}

func reverseURL(server string) (string, error) {
	u, err := url.Parse(server)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	default:
		u.Scheme = "ws"
	}
	if !strings.HasSuffix(u.Path, "/ws") {
		u.Path = strings.TrimSuffix(u.Path, "/") + "/ws"
	}
	return u.String(), nil
}

// serve reads frames: session assignment → hello → request loop.
func serve(conn *websocket.Conn) error {
	conn.SetReadLimit(maxMessageBytes)
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		text := string(payload)
		// Hub wire format is "<MsgType> <payload>"; the proxied requests
		// arrive as "PRIVATE reverse_request {json}" — strip the type
		// prefix first, then dispatch on the command.
		parts := strings.SplitN(text, " ", 2)
		if len(parts) != 2 {
			continue
		}
		payload1 := parts[1]
		command := parts[0]
		if command == "PRIVATE" || command == "CONFIG" {
			// "PRIVATE reverse_request {json}" → command=reverse_request
			if sub := strings.SplitN(payload1, " ", 2); len(sub) == 2 && strings.HasPrefix(sub[0], "reverse_") {
				command, payload1 = sub[0], sub[1]
			}
		}
		switch command {
		case "CONFIG":
			if sid, ok := stripPrefix(payload1, "websocket-session-id:"); ok && sid != "" {
				hello := reverse.HelloMessage{
					SessionID: sid,
					PeerID:    global.Env().SystemConfig.NodeConfig.ID,
					Features:  []string{reverse.CompressionGzip},
				}
				if err := send(conn, reverse.FormatHelloCommand(hello)); err != nil {
					return err
				}
				log.Debugf("agent reverse channel hello sent for session [%s]", sid)
			}
		case reverse.FeaturesCommand:
			// manager answered our capability advertisement
			if fm, err := reverse.ParseFeaturePayload(payload1); err == nil {
				managerGzipSupported.Store(fm.Gzip)
			}
		case reverse.RequestCommand:
			go handleRequest(conn, payload1)
		}
	}
}

func stripPrefix(s, prefix string) (string, bool) {
	if strings.HasPrefix(s, prefix) {
		return strings.TrimSpace(s[len(prefix):]), true
	}
	return "", false
}

func send(conn *websocket.Conn, payload string) error {
	writeMu.Lock()
	defer writeMu.Unlock()
	return conn.WriteMessage(websocket.TextMessage, []byte(payload))
}

// managerGzipSupported flips true once the manager answers our HELLO
// capability advertisement; until then responses stay uncompressed
// (mixed-version safe: an old manager ignores the unknown feature frame).
var managerGzipSupported atomic.Bool

// handleRequest executes one proxied HTTP request against the agent's
// own web port and streams the response back in chunks. Bodies over the
// compression threshold are gzipped when the manager advertised support,
// flagged via ResponseMessage.Compressed.
func handleRequest(conn *websocket.Conn, payload string) {
	reqMsg, err := reverse.ParseRequestPayload(payload)
	if err != nil {
		log.Debugf("agent reverse channel: bad request payload: %v", err)
		return
	}

	status, body := execute(reqMsg)

	compressed := false
	if managerGzipSupported.Load() {
		body, compressed = reverse.MaybeCompress(body)
	}

	resp := reverse.ResponseMessage{
		RequestID:  reqMsg.RequestID,
		PeerID:     reqMsg.PeerID,
		Compressed: compressed,
	}
	// chunk the body (base64) then a Done frame with the status
	for offset := 0; offset < len(body); offset += 64 * 1024 {
		end := offset + 64*1024
		if end > len(body) {
			end = len(body)
		}
		resp.Chunk = base64.StdEncoding.EncodeToString(body[offset:end])
		if err := send(conn, reverse.FormatResponseCommand(resp)); err != nil {
			log.Debugf("agent reverse channel: response chunk failed: %v", err)
			return
		}
	}
	resp.Chunk, resp.Done, resp.Status = "", true, status
	if len(body) == 0 {
		// still send one empty chunk so the manager assembles something
		resp.Chunk = ""
	}
	if err := send(conn, reverse.FormatResponseCommand(resp)); err != nil {
		log.Debugf("agent reverse channel: response done failed: %v", err)
	}
}

// execute performs the loopback HTTP call against the agent's web port.
func execute(reqMsg reverse.RequestMessage) (int, []byte) {
	target := localBaseURL() + reqMsg.Path
	var body io.Reader
	if b, err := reqMsg.BodyBytes(); err == nil && len(b) > 0 {
		body = strings.NewReader(string(b))
	}
	req, err := http.NewRequest(reqMsg.Method, target, body)
	if err != nil {
		return http.StatusBadRequest, []byte(err.Error())
	}
	reqMsg.ApplyHeaders(req)
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", util.ContentTypeJson)
	}
	// Authenticate the loopback call with the agent's own API token so
	// it passes the access_token realm.
	if tok, _ := common.LoadTokenFromKeystore("AGENT_API_ACCESS_TOKEN"); tok != "" {
		req.Header.Set("X-API-Token", strings.TrimSpace(tok))
	}
	client := &http.Client{Timeout: requestDeadline}
	resp, err := client.Do(req)
	if err != nil {
		return http.StatusBadGateway, []byte(err.Error())
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data
}

// localBaseURL derives the agent's own serving address.
func localBaseURL() string {
	// Prefer the web app port; some binaries (gateway) serve their API
	// under the `api:` section instead with an empty WebAppConfig network
	// — GetBindingAddr panics on that, so fall through to APIConfig.
	var schema, addr string
	web := global.Env().SystemConfig.WebAppConfig
	if web.NetworkConfig.Port > 0 || web.NetworkConfig.Binding != "" || web.NetworkConfig.Host != "" {
		schema = "http"
		if web.TLSConfig.TLSEnabled {
			schema = "https"
		}
		addr = web.NetworkConfig.GetBindingAddr()
	} else if api := global.Env().SystemConfig.APIConfig; api.Enabled && (api.NetworkConfig.Port > 0 || api.NetworkConfig.Binding != "" || api.NetworkConfig.Host != "") {
		schema = "http"
		if api.TLSConfig.TLSEnabled {
			schema = "https"
		}
		addr = api.NetworkConfig.GetBindingAddr()
	} else {
		return "http://127.0.0.1"
	}
	// Loopback preference: wildcard bindings (0.0.0.0/::) are reached via
	// 127.0.0.1, but a binding pinned to a specific interface IP (e.g. a
	// second instance on a LAN address) must keep that host — nothing
	// listens on the loopback rewrite for such bindings.
	if i := strings.LastIndex(addr, ":"); i > 0 {
		host := addr[:i]
		if host == "" || host == "0.0.0.0" || host == "::" {
			host = "127.0.0.1"
		}
		return schema + "://" + host + addr[i:]
	}
	return schema + "://127.0.0.1"
}
