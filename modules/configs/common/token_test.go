package common

import (
	"testing"

	"infini.sh/framework/core/keystore"
)

func TestEnsureTokenInKeystoreReusesStoredValue(t *testing.T) {
	t.Setenv(keystore.PathEnvKey, t.TempDir())

	first, err := EnsureTokenInKeystore(ManagerTokenKeystoreKey)
	if err != nil {
		t.Fatalf("ensure token first call: %v", err)
	}
	if len(first) != 48 {
		t.Fatalf("unexpected token length: %d", len(first))
	}

	second, err := EnsureTokenInKeystore(ManagerTokenKeystoreKey)
	if err != nil {
		t.Fatalf("ensure token second call: %v", err)
	}
	if second != first {
		t.Fatalf("expected stored token to be reused, got %q and %q", first, second)
	}

	loaded, err := LoadTokenFromKeystore(ManagerTokenKeystoreKey)
	if err != nil {
		t.Fatalf("load token from keystore: %v", err)
	}
	if loaded != first {
		t.Fatalf("unexpected loaded token: %q", loaded)
	}
}
