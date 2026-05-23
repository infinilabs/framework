package reverse

import (
	"encoding/base64"
	"net/http"
	"strings"

	"infini.sh/framework/core/util"
)

const (
	HeaderPeerID              = "X-INFINI-INSTANCE-ID"
	HelloCommand              = "reverse_hello"
	RequestCommand            = "reverse_request"
	ResponseCommand           = "reverse_response"
	DefaultResponseChunkBytes = 32 * 1024
)

type HelloMessage struct {
	SessionID string `json:"session_id"`  // Identifies the reverse websocket session being activated by the hello handshake.
	PeerID    string `json:"instance_id"` // Identifies which instance is attaching to that reverse session.
}

type RequestMessage struct {
	RequestID   string      `json:"request_id"`  // Correlates the reverse request with its response chunks.
	PeerID      string      `json:"instance_id"` // Identifies the target instance on the other side of the reverse channel.
	Method      string      `json:"method"`
	Path        string      `json:"path"`
	Body        string      `json:"body,omitempty"`
	Headers     http.Header `json:"headers,omitempty"`
	AccessToken string      `json:"access_token,omitempty"`
}

type ResponseMessage struct {
	RequestID string `json:"request_id"`  // Correlates the response chunks with the original reverse request.
	PeerID    string `json:"instance_id"` // Identifies which instance produced the reverse response.
	Chunk     string `json:"chunk,omitempty"`
	Status    int    `json:"status,omitempty"`
	Done      bool   `json:"done,omitempty"`
}

func ParseHelloPayload(payload string) (HelloMessage, error) {
	msg := HelloMessage{}
	return msg, util.FromJSONBytes([]byte(payload), &msg)
}

func ParseRequestPayload(payload string) (RequestMessage, error) {
	msg := RequestMessage{}
	return msg, util.FromJSONBytes([]byte(payload), &msg)
}

func ParseResponsePayload(payload string) (ResponseMessage, error) {
	msg := ResponseMessage{}
	return msg, util.FromJSONBytes([]byte(payload), &msg)
}

func FormatHelloCommand(msg HelloMessage) string {
	return HelloCommand + " " + string(util.MustToJSONBytes(msg))
}

func FormatRequestCommand(msg RequestMessage) string {
	return RequestCommand + " " + string(util.MustToJSONBytes(msg))
}

func FormatResponseCommand(msg ResponseMessage) string {
	return ResponseCommand + " " + string(util.MustToJSONBytes(msg))
}

func (m *RequestMessage) SetBody(body []byte) {
	if len(body) == 0 {
		m.Body = ""
		return
	}
	m.Body = base64.StdEncoding.EncodeToString(body)
}

func (m RequestMessage) BodyBytes() ([]byte, error) {
	if m.Body == "" {
		return nil, nil
	}
	return base64.StdEncoding.DecodeString(m.Body)
}

func (m RequestMessage) NormalizedHeaders() http.Header {
	headers := http.Header{}
	for key, values := range m.Headers {
		copied := append([]string(nil), values...)
		headers[key] = copied
	}
	if headers.Get("Authorization") == "" && strings.TrimSpace(m.AccessToken) != "" {
		headers.Set("Authorization", "Bearer "+strings.TrimSpace(m.AccessToken))
	}
	return headers
}

func (m RequestMessage) ApplyHeaders(req *http.Request) {
	if req == nil {
		return
	}
	if req.Header == nil {
		req.Header = http.Header{}
	}
	for key := range req.Header {
		req.Header.Del(key)
	}
	for key, values := range m.NormalizedHeaders() {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
}

func (m RequestMessage) BearerToken() string {
	value := strings.TrimSpace(m.NormalizedHeaders().Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(value), "bearer ") {
		return ""
	}
	return strings.TrimSpace(value[7:])
}

func (m *ResponseMessage) SetChunk(body []byte) {
	if len(body) == 0 {
		m.Chunk = ""
		return
	}
	m.Chunk = base64.StdEncoding.EncodeToString(body)
}

func (m ResponseMessage) ChunkBytes() ([]byte, error) {
	if m.Chunk == "" {
		return nil, nil
	}
	return base64.StdEncoding.DecodeString(m.Chunk)
}

func WriteChunkedResponse(write func(payload string) error, requestID, peerID string, status int, body []byte, chunkBytes int) error {
	if chunkBytes <= 0 {
		chunkBytes = DefaultResponseChunkBytes
	}
	for start := 0; start < len(body); start += chunkBytes {
		end := start + chunkBytes
		if end > len(body) {
			end = len(body)
		}
		msg := ResponseMessage{
			RequestID: requestID,
			PeerID:    peerID,
		}
		msg.SetChunk(body[start:end])
		if err := write(FormatResponseCommand(msg)); err != nil {
			return err
		}
	}

	done := ResponseMessage{
		RequestID: requestID,
		PeerID:    peerID,
		Status:    status,
		Done:      true,
	}
	return write(FormatResponseCommand(done))
}
