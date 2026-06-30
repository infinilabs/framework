package credential

import "testing"

func TestEncodeDecodeAccessToken(t *testing.T) {
	cred := &Credential{
		Name: "agent-token",
		Type: AccessToken,
		Payload: map[string]interface{}{
			AccessToken: map[string]interface{}{
				"access_token": "mock-token",
			},
		},
	}
	cred.SetSecret([]byte("12345678901234567890123456789012"))

	if err := cred.Encode(); err != nil {
		t.Fatalf("Encode() returned error: %v", err)
	}

	token, err := cred.DecodeAccessToken()
	if err != nil {
		t.Fatalf("DecodeAccessToken() returned error: %v", err)
	}

	if got := token.AccessToken.Get(); got != "mock-token" {
		t.Fatalf("expected access token %q, got %q", "mock-token", got)
	}
}
