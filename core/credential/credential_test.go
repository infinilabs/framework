package credential

import (
	"os"
	"sync"
	"testing"

	"infini.sh/framework/core/keystore"
)

var credentialTestKeystoreDir string
var credentialTestKeystoreOnce sync.Once

func useCredentialTestKeystore(t *testing.T) {
	t.Helper()

	credentialTestKeystoreOnce.Do(func() {
		dir, err := os.MkdirTemp("", "framework-credential-keystore-*")
		if err != nil {
			t.Fatalf("create credential test keystore dir: %v", err)
		}
		credentialTestKeystoreDir = dir
	})

	t.Setenv(keystore.PathEnvKey, credentialTestKeystoreDir)
}

func TestTokenCredentialRoundTrip(t *testing.T) {
	useCredentialTestKeystore(t)

	cred := &Credential{
		Name: "agent token",
		Type: Token,
		Payload: map[string]interface{}{
			Token: map[string]interface{}{
				"value": "super-secret-token",
			},
		},
	}

	if err := cred.Encode(); err != nil {
		t.Fatalf("encode token credential: %v", err)
	}

	encryptedValue, ok := cred.Payload[Token].(map[string]interface{})["value"].(string)
	if !ok || encryptedValue == "" || encryptedValue == "super-secret-token" {
		t.Fatalf("expected encrypted token payload, got %#v", cred.Payload[Token])
	}

	value, err := cred.DecodeToken()
	if err != nil {
		t.Fatalf("decode token credential: %v", err)
	}
	if value != "super-secret-token" {
		t.Fatalf("unexpected token value: %q", value)
	}
}

func TestEncodeTokenRejectsInvalidPayload(t *testing.T) {
	useCredentialTestKeystore(t)

	tests := []struct {
		name    string
		payload map[string]interface{}
	}{
		{
			name: "wrong payload type",
			payload: map[string]interface{}{
				Token: "invalid",
			},
		},
		{
			name: "wrong token value type",
			payload: map[string]interface{}{
				Token: map[string]interface{}{
					"value": 1,
				},
			},
		},
		{
			name: "empty token value",
			payload: map[string]interface{}{
				Token: map[string]interface{}{
					"value": "",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cred := &Credential{
				Name:    "invalid token",
				Type:    Token,
				Payload: tc.payload,
			}
			if err := cred.Encode(); err == nil {
				t.Fatal("expected token encode error")
			}
		})
	}
}

func TestDecodeTokenRejectsOtherCredentialType(t *testing.T) {
	useCredentialTestKeystore(t)

	cred := &Credential{
		Name: "basic auth",
		Type: BasicAuth,
		Payload: map[string]interface{}{
			BasicAuth: map[string]interface{}{
				"username": "admin",
				"password": "admin",
			},
		},
	}

	if err := cred.Encode(); err != nil {
		t.Fatalf("encode basic auth credential: %v", err)
	}

	if _, err := cred.DecodeToken(); err == nil {
		t.Fatal("expected decode token to reject basic auth credential")
	}
}
