/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package server

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"infini.sh/framework/core/api"
	framework_ws "infini.sh/framework/core/api/websocket"
	"infini.sh/framework/core/api/websocket/reverse"
	log "infini.sh/framework/core/log"
	"infini.sh/framework/core/model"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
)

// ──────────────────────────────────────────────────────────────────────────
// Reverse channel hosting — managed instances are NOT directly reachable:
// agents sit behind NAT/firewalls and only dial OUT. The manager calls
// back THROUGH the agent-initiated websocket (see
// core/api/websocket/reverse): the agent connects to the manager's /ws
// endpoint with its instance ID in the peer header, HELLOs, and from
// then on the manager can ProxyRequest down that connection while the
// agent executes locally and streams the response back.
//
// This file wires the manager side: websocket callbacks + commands, and
// a ReverseProxyRequest helper for consumers (LogPilot's instance
// detail panel is the first).
// ──────────────────────────────────────────────────────────────────────────

var (
	reverseManager      *reverse.SessionManager
	reverseRegisterOnce sync.Once
)

// ReverseChannelReady reports whether the agent reverse channel is wired.
func ReverseChannelReady() bool { return reverseManager != nil }

// ReverseIsConnected reports whether the given instance holds an active
// reverse-channel session (i.e., the manager can reach it right now).
func ReverseIsConnected(instanceID string) bool {
	return reverseManager != nil && reverseManager.IsConnected(instanceID)
}

// ReverseProxyRequest performs a logical HTTP request against a managed
// instance THROUGH its reverse channel. Returns an error when the
// instance is not connected (the one-way deployment reality).
func ReverseProxyRequest(peerID string, req *util.Request) (*util.Result, error) {
	if reverseManager == nil {
		return nil, fmt.Errorf("reverse channel not enabled on this manager")
	}
	// The send callback pushes the wire frame down the instance's
	// websocket session.
	send := func(sessionID, payload string) error {
		return framework_ws.SendPrivateMessage(sessionID, payload)
	}
	return reverseManager.ProxyRequest(peerID, req, nil, send, nil)
}

// ReverseProxyRequestJSON is ReverseProxyRequest with a JSON response
// unmarshal convenience.
func ReverseProxyRequestJSON(peerID string, req *util.Request, out interface{}) (*util.Result, error) {
	if reverseManager == nil {
		return nil, fmt.Errorf("reverse channel not enabled on this manager")
	}
	send := func(sessionID, payload string) error {
		return framework_ws.SendPrivateMessage(sessionID, payload)
	}
	return reverseManager.ProxyRequest(peerID, req, nil, send, out)
}

// setupReverseChannel wires the websocket callbacks and commands. Called
// from Setup once.
func setupReverseChannel() {
	reverseRegisterOnce.Do(func() {
		reverseManager = reverse.NewSessionManager(reverse.ManagerOptions{})

		framework_ws.RegisterConnectCallback(onReverseConnect)
		framework_ws.RegisterDisconnectCallback(onReverseDisconnect)
		api.HandleWebSocketCommand(reverse.HelloCommand, "instance reverse hello", handleReverseHello)
		api.HandleWebSocketCommand(reverse.ResponseCommand, "instance reverse response", handleReverseResponse)

		log.Info("configs server: reverse channel ready (instances dial /ws; manager calls back through it)")
	})
}

// onReverseConnect validates the connecting instance and opens its
// pending session. The agent sends its instance ID in the peer header.
func onReverseConnect(sessionID string, w http.ResponseWriter, r *http.Request) error {
	instanceID := strings.TrimSpace(r.Header.Get(reverse.HeaderPeerID))
	if instanceID == "" {
		return nil // not a managed-instance connection
	}

	ctx := orm.NewContext().DirectAccess()
	inst := model.Instance{}
	inst.ID = instanceID
	exists, err := orm.GetV2(ctx, &inst)
	if err != nil && !isNotFound(err) {
		log.Warnf("configs server: reverse connect [%s] instance lookup failed: %v", instanceID, err)
		return err
	}
	if err != nil || !exists {
		log.Warnf("configs server: reverse connect rejected: instance %s is not registered", instanceID)
		return fmt.Errorf("instance %s is not registered", instanceID)
	}
	if loadInstanceStatus(ctx, instanceID) != StatusApproved {
		log.Warnf("configs server: reverse connect rejected: instance %s is not approved", instanceID)
		return fmt.Errorf("instance %s is not approved", instanceID)
	}
	// Credential check: the dial-out must authenticate like a sync would
	// (manager token / registered self token in the Authorization header
	// or token query param).
	presented := strings.TrimSpace(r.Header.Get("Authorization"))
	presented = strings.TrimPrefix(presented, "Bearer ")
	if presented == "" {
		presented = strings.TrimSpace(r.URL.Query().Get("token"))
	}
	if !matchesManagerToken(ctx, instanceID, presented) &&
		!matchesRegisteredAccessToken(ctx, instanceID, presented) {
		log.Warnf("configs server: reverse connect rejected: instance %s credential check failed (token presented: %v)", instanceID, presented != "")
		return fmt.Errorf("instance %s reverse channel credential rejected", instanceID)
	}

	reverseManager.RegisterPendingSession(sessionID, instanceID)
	return nil
}

func onReverseDisconnect(sessionID string) {
	if reverseManager != nil {
		reverseManager.OnDisconnect(sessionID)
	}
}

func handleReverseHello(c *framework_ws.WebsocketConnection, array []string) {
	if len(array) < 2 || reverseManager == nil {
		return
	}
	features, err := reverseManager.HandleHelloPayload(strings.Join(array[1:], " "))
	if err != nil {
		log.Warnf("configs server: reverse hello rejected: %v", err)
		return
	}
	log.Debugf("configs server: reverse hello accepted")

	// answer the peer's capability advertisement with our own, so both
	// sides compress only when the other end supports it (old peers ignore
	// this unknown command and stay on the uncompressed path)
	if len(features) > 0 {
		msg := reverse.FeatureMessage{Gzip: true}
		_ = c.WritePrivateMessage(reverse.FormatFeatureCommand(msg))
	}
}

func handleReverseResponse(c *framework_ws.WebsocketConnection, array []string) {
	if len(array) < 2 || reverseManager == nil {
		return
	}
	if err := reverseManager.HandleResponsePayload(strings.Join(array[1:], " ")); err != nil {
		log.Debugf("configs server: reverse response error: %v", err)
	}
}
