package common

import "testing"

func TestTokenKeystoreHelpers(t *testing.T) {
	t.Setenv("KEYSTORE_PATH", t.TempDir())

	t.Run("missing token returns empty value", func(t *testing.T) {
		value, err := LoadTokenFromKeystore("missing-token")
		if err != nil {
			t.Fatalf("failed to load missing token: %v", err)
		}
		if value != "" {
			t.Fatalf("expected missing token to return empty value, got %q", value)
		}
	})

	t.Run("ensure token is idempotent", func(t *testing.T) {
		first, err := EnsureTokenInKeystore("managed-token")
		if err != nil {
			t.Fatalf("failed to initialize token: %v", err)
		}
		if first == "" {
			t.Fatal("expected generated token to be non-empty")
		}

		second, err := EnsureTokenInKeystore("managed-token")
		if err != nil {
			t.Fatalf("failed to reload token: %v", err)
		}
		if second != first {
			t.Fatalf("expected token helper to be idempotent, got %q then %q", first, second)
		}

		loaded, err := LoadTokenFromKeystore("managed-token")
		if err != nil {
			t.Fatalf("failed to load saved token: %v", err)
		}
		if loaded != first {
			t.Fatalf("expected saved token to match generated token, got %q want %q", loaded, first)
		}
	})
}
