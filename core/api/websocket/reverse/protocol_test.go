package reverse

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestRequestMessageNormalizedHeadersFallsBackToLegacyAccessToken(t *testing.T) {
	msg := RequestMessage{
		AccessToken: "token-1",
	}

	headers := msg.NormalizedHeaders()
	if got := headers.Get("Authorization"); got != "Bearer token-1" {
		t.Fatalf("unexpected authorization header: %s", got)
	}
	if got := msg.BearerToken(); got != "token-1" {
		t.Fatalf("unexpected bearer token: %s", got)
	}
}

func TestRequestMessageApplyHeaders(t *testing.T) {
	msg := RequestMessage{
		Headers: http.Header{
			"Authorization": []string{"Bearer token-2"},
			"X-Test":        []string{"value"},
		},
	}
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	req.Header.Set("Existing", "old")

	msg.ApplyHeaders(req)

	if req.Header.Get("Existing") != "" {
		t.Fatal("expected old header to be removed")
	}
	if req.Header.Get("Authorization") != "Bearer token-2" {
		t.Fatalf("unexpected authorization header: %s", req.Header.Get("Authorization"))
	}
	if req.Header.Get("X-Test") != "value" {
		t.Fatalf("unexpected x-test header: %s", req.Header.Get("X-Test"))
	}
}

func TestWriteChunkedResponseRejectsInvalidStatus(t *testing.T) {
	write := func(payload string) error {
		t.Fatal("no frame should be written for an invalid status")
		return nil
	}
	if err := WriteChunkedResponse(write, "req-1", "peer-1", 0, []byte("body"), DefaultResponseChunkBytes); err == nil {
		t.Fatal("expected error for zero status")
	}
	if err := WriteChunkedResponse(write, "req-1", "peer-1", 99, []byte("body"), DefaultResponseChunkBytes); err == nil {
		t.Fatal("expected error for out-of-range status")
	}
	if err := WriteChunkedResponse(write, "req-1", "peer-1", 600, []byte("body"), DefaultResponseChunkBytes); err == nil {
		t.Fatal("expected error for out-of-range status")
	}
}

func TestWriteFailureResponse(t *testing.T) {
	var frames []string
	err := WriteFailureResponse(func(payload string) error {
		frames = append(frames, payload)
		return nil
	}, "req-1", "peer-1", errors.New("boom"))
	if err != nil {
		t.Fatalf("write failure response: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("expected exactly one frame, got %d", len(frames))
	}
	if !strings.HasPrefix(frames[0], ResponseCommand+" ") {
		t.Fatalf("unexpected payload: %s", frames[0])
	}
	msg, err := ParseResponsePayload(strings.TrimPrefix(frames[0], ResponseCommand+" "))
	if err != nil {
		t.Fatalf("parse response payload: %v", err)
	}
	if !msg.Done || msg.Error != "boom" || msg.Status != 0 || msg.Chunk != "" {
		t.Fatalf("unexpected failure frame: %+v", msg)
	}
}
