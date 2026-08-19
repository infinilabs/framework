package reverse

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"infini.sh/framework/core/util"
)

func TestSessionManagerProxyRequestRoundTrip(t *testing.T) {
	manager := NewSessionManager(ManagerOptions{})
	manager.RegisterPendingSession("session-1", "peer-1")
	if err := manager.ActivateSession("session-1", "peer-1"); err != nil {
		t.Fatalf("activate session: %v", err)
	}

	headers := http.Header{}
	headers.Set("Authorization", "Bearer token-1")

	send := func(sessionID, payload string) error {
		if sessionID != "session-1" {
			t.Fatalf("unexpected session id: %s", sessionID)
		}
		if !strings.HasPrefix(payload, RequestCommand+" ") {
			t.Fatalf("unexpected payload: %s", payload)
		}
		msg, err := ParseRequestPayload(strings.TrimPrefix(payload, RequestCommand+" "))
		if err != nil {
			t.Fatalf("parse request payload: %v", err)
		}
		if msg.BearerToken() != "token-1" {
			t.Fatalf("unexpected bearer token: %s", msg.BearerToken())
		}
		return WriteChunkedResponse(func(responsePayload string) error {
			if !strings.HasPrefix(responsePayload, ResponseCommand+" ") {
				t.Fatalf("unexpected response payload: %s", responsePayload)
			}
			return manager.HandleResponsePayload(strings.TrimPrefix(responsePayload, ResponseCommand+" "))
		}, msg.RequestID, msg.PeerID, http.StatusOK, []byte(`{"ack":true}`), DefaultResponseChunkBytes)
	}

	var response map[string]bool
	req := &util.Request{Method: http.MethodGet, Path: "/stats"}
	res, err := manager.ProxyRequest("peer-1", req, headers, send, &response)
	if err != nil {
		t.Fatalf("proxy request: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", res.StatusCode)
	}
	if !response["ack"] {
		t.Fatal("expected response to unmarshal")
	}
}

func TestSessionManagerDisconnectFailsPendingRequest(t *testing.T) {
	manager := NewSessionManager(ManagerOptions{})
	manager.RegisterPendingSession("session-1", "peer-1")
	if err := manager.ActivateSession("session-1", "peer-1"); err != nil {
		t.Fatalf("activate session: %v", err)
	}

	send := func(sessionID, payload string) error {
		manager.OnDisconnect(sessionID)
		return nil
	}

	_, err := manager.ProxyRequest("peer-1", &util.Request{Method: http.MethodGet, Path: "/stats"}, nil, send, nil)
	if !IsRecoverableError(err) {
		t.Fatalf("expected recoverable disconnect error, got %v", err)
	}
}

// A Done frame carrying Error means the agent could not execute the request
// at all; the call must fail with the agent's error and must not be retried.
func TestSessionManagerExecutionFailure(t *testing.T) {
	manager := NewSessionManager(ManagerOptions{})
	manager.RegisterPendingSession("session-1", "peer-1")
	if err := manager.ActivateSession("session-1", "peer-1"); err != nil {
		t.Fatalf("activate session: %v", err)
	}

	send := func(sessionID, payload string) error {
		msg, err := ParseRequestPayload(strings.TrimPrefix(payload, RequestCommand+" "))
		if err != nil {
			t.Fatalf("parse request payload: %v", err)
		}
		return WriteFailureResponse(func(responsePayload string) error {
			return manager.HandleResponsePayload(strings.TrimPrefix(responsePayload, ResponseCommand+" "))
		}, msg.RequestID, msg.PeerID, errors.New("local handler blew up"))
	}

	_, err := manager.ProxyRequest("peer-1", &util.Request{Method: http.MethodGet, Path: "/stats"}, nil, send, nil)
	if err == nil || !strings.Contains(err.Error(), "local handler blew up") {
		t.Fatalf("expected execution failure, got %v", err)
	}
	if IsRecoverableError(err) {
		t.Fatal("execution failure must not be recoverable")
	}
}

// A Done frame with neither Status nor Error is an agent-side protocol
// violation and must fail loudly instead of defaulting to 200.
func TestSessionManagerRejectsDoneWithoutStatus(t *testing.T) {
	manager := NewSessionManager(ManagerOptions{})
	manager.RegisterPendingSession("session-1", "peer-1")
	if err := manager.ActivateSession("session-1", "peer-1"); err != nil {
		t.Fatalf("activate session: %v", err)
	}

	send := func(sessionID, payload string) error {
		msg, err := ParseRequestPayload(strings.TrimPrefix(payload, RequestCommand+" "))
		if err != nil {
			t.Fatalf("parse request payload: %v", err)
		}
		frame := FormatResponseCommand(ResponseMessage{RequestID: msg.RequestID, PeerID: msg.PeerID, Done: true})
		return manager.HandleResponsePayload(strings.TrimPrefix(frame, ResponseCommand+" "))
	}

	_, err := manager.ProxyRequest("peer-1", &util.Request{Method: http.MethodGet, Path: "/stats"}, nil, send, nil)
	if err == nil || !strings.Contains(err.Error(), "neither status nor error") {
		t.Fatalf("expected missing status error, got %v", err)
	}
}
