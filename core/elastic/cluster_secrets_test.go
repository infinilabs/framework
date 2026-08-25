/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package elastic

import (
	"encoding/json"
	"os"
	"testing"

	"infini.sh/framework/core/model"
	"infini.sh/framework/core/keystore"
	ucfg "infini.sh/framework/lib/go-ucfg"
)

// TestClusterSecretsORMRoundtrip reproduces the secured-cluster 401: a
// plain-text password set on an ElasticsearchConfig is marshaled to the
// shadow mask when the record is persisted, and only the keystore holds
// the real value for HydrateClusterSecrets to restore.
func TestClusterSecretsORMRoundtrip(t *testing.T) {
	wd, _ := os.Getwd()
	if err := os.Setenv(keystore.PathEnvKey, wd); err != nil {
		t.Fatal(err)
	}

	// 1) Create: the request decodes a plain password; stash it, then
	//    marshal the record the way the ORM does (mask must be stored).
	cfg := ElasticsearchConfig{}
	cfg.ID = "cs-test-1"
	cfg.Name = "secured"
	ba := model.BasicAuth{Username: "admin", Password: ucfg.SecretString("real-pass-123")}
	cfg.BasicAuth = &ba
	StashClusterSecrets(&cfg)

	stored, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if string(stored) == "" || !jsonContains(stored, `"password":"******"`) {
		t.Fatalf("ORM marshal must persist the mask, got: %s", stored)
	}
	if jsonContains(stored, "real-pass-123") {
		t.Fatalf("ORM marshal must not leak the real password: %s", stored)
	}

	// 2) Read back (what every ORM consumer sees) + hydrate.
	loaded := ElasticsearchConfig{}
	if err := json.Unmarshal(stored, &loaded); err != nil {
		t.Fatal(err)
	}
	loaded.ID = cfg.ID
	if got := loaded.BasicAuth.Password.Get(); got != ucfg.SecretShadowText {
		t.Fatalf("loaded password should be the mask, got %q", got)
	}
	HydrateClusterSecrets(&loaded)
	if got := loaded.BasicAuth.Password.Get(); got != "real-pass-123" {
		t.Fatalf("hydrated password = %q, want real-pass-123", got)
	}

	// 3) Update that does NOT retype the password (mask round-trips):
	//    stash must be skipped, hydration must still restore.
	edited := ElasticsearchConfig{}
	if err := json.Unmarshal(stored, &edited); err != nil {
		t.Fatal(err)
	}
	edited.ID = cfg.ID
	StashClusterSecrets(&edited) // password is the mask: no-op
	HydrateClusterSecrets(&edited)
	if got := edited.BasicAuth.Password.Get(); got != "real-pass-123" {
		t.Fatalf("unretyped edit must keep the stashed secret, got %q", got)
	}

	// 4) Retyped password via the raw delta (partial-update mode).
	StashClusterSecretsFromDelta(cfg.ID, map[string]interface{}{
		"basic_auth": map[string]interface{}{"username": "admin", "password": "new-pass-456"},
	})
	rehydrated := ElasticsearchConfig{}
	_ = json.Unmarshal(stored, &rehydrated)
	rehydrated.ID = cfg.ID
	HydrateClusterSecrets(&rehydrated)
	if got := rehydrated.BasicAuth.Password.Get(); got != "new-pass-456" {
		t.Fatalf("retyped password must replace the stashed secret, got %q", got)
	}

	// 5) Delete removes the secret; hydration then leaves the mask.
	RemoveClusterSecrets(cfg.ID)
	after := ElasticsearchConfig{}
	_ = json.Unmarshal(stored, &after)
	after.ID = cfg.ID
	HydrateClusterSecrets(&after)
	if got := after.BasicAuth.Password.Get(); got != ucfg.SecretShadowText {
		t.Fatalf("after delete hydration must leave the mask, got %q", got)
	}
}

func jsonContains(b []byte, sub string) bool {
	return len(sub) == 0 || (len(b) >= len(sub) && stringContains(string(b), sub))
}

func stringContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
