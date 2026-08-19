package reverse

import (
	"encoding/base64"
	"fmt"
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
	SessionID string `json:"session_id"`
	PeerID    string `json:"instance_id"`
}

type RequestMessage struct {
	RequestID   string      `json:"request_id"`
	PeerID      string      `json:"instance_id"`
	Method      string      `json:"method"`
	Path        string      `json:"path"`
	Body        string      `json:"body,omitempty"`
	Headers     http.Header `json:"headers,omitempty"`
	AccessToken string      `json:"access_token,omitempty"`
}

type ResponseMessage struct {
	RequestID string `json:"request_id"`
	PeerID    string `json:"instance_id"`
	Chunk     string `json:"chunk,omitempty"`
	// Status is the HTTP status of the locally executed request, set on the
	// Done frame. 0 is not a valid HTTP status (valid range is 100-599);
	// a Done frame without Status is only legal together with Error.
	Status int `json:"status,omitempty"`
	// Error reports an execution failure on the Done frame: the agent could
	// not produce an HTTP response at all, so there is no status to report.
	// Mutually exclusive with Status.
	Error string `json:"error,omitempty"`
	Done  bool   `json:"done,omitempty"`
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

// WriteChunkedResponse streams body back to the control plane in chunkBytes-sized
// base64 frames, followed by a final Done frame carrying status. status must be
// a real HTTP status code; if the request could not be executed at all, use
// WriteFailureResponse instead.
func WriteChunkedResponse(write func(payload string) error, requestID, peerID string, status int, body []byte, chunkBytes int) error {
	if status < 100 || status > 599 {
		return fmt.Errorf("status must be a valid HTTP status code (100-599), got %d", status)
	}
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

// WriteFailureResponse terminates the response stream for requestID with an
// execution failure: the agent could not produce an HTTP response at all, so
// there is no status code to report.
func WriteFailureResponse(write func(payload string) error, requestID, peerID string, cause error) error {
	msg := ResponseMessage{
		RequestID: requestID,
		PeerID:    peerID,
		Done:      true,
	}
	if cause != nil {
		msg.Error = cause.Error()
	}
	return write(FormatResponseCommand(msg))
}
