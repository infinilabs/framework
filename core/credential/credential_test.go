package credential

import (
	"testing"

	"infini.sh/framework/core/model"
)

func TestCredentialTokenHelpers(t *testing.T) {
	t.Setenv("KEYSTORE_PATH", t.TempDir())

	t.Run("encode decode round trip", func(t *testing.T) {
		cred := &Credential{
			Name: "agent-token",
			Type: Token,
			Payload: map[string]interface{}{
				Token: map[string]interface{}{
					"value": "secret-token-value",
				},
			},
		}

		if err := cred.Encode(); err != nil {
			t.Fatalf("failed to encode token credential: %v", err)
		}

		decodedValue, err := cred.DecodeToken()
		if err != nil {
			t.Fatalf("failed to decode token credential: %v", err)
		}
		if decodedValue != "secret-token-value" {
			t.Fatalf("unexpected decoded token value: %q", decodedValue)
		}

		decoded, err := cred.Decode()
		if err != nil {
			t.Fatalf("failed to decode credential via generic path: %v", err)
		}
		token, ok := decoded.(model.Token)
		if !ok {
			t.Fatalf("unexpected decoded type: %T", decoded)
		}
		if token.Value != "secret-token-value" {
			t.Fatalf("unexpected decoded token payload: %q", token.Value)
		}
	})

	t.Run("decode requires salt", func(t *testing.T) {
		cred := &Credential{
			Name: "agent-token",
			Type: Token,
			Payload: map[string]interface{}{
				Token: map[string]interface{}{
					"value": "secret-token-value",
				},
			},
		}

		if err := cred.Encode(); err != nil {
			t.Fatalf("failed to encode token credential: %v", err)
		}

		delete(cred.Encrypt.Params, "salt")

		if _, err := cred.DecodeToken(); err == nil {
			t.Fatal("expected decode token to fail without salt")
		}
	})
}
