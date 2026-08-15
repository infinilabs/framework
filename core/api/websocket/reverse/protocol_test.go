package reverse

import (
	"net/http"
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
