/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package util

import (
	"encoding/json"
	"testing"

	"infini.sh/framework/lib/go-ucfg"
)

type secretNested struct {
	Password ucfg.SecretString   `json:"password"`
	Tokens   []ucfg.SecretString `json:"tokens,omitempty"`
}

type secretHolder struct {
	Name    string            `json:"name"`
	APIKey  ucfg.SecretString `json:"api_key"`
	Nested  *secretNested     `json:"nested,omitempty"`
	Headers map[string]ucfg.SecretString `json:"headers,omitempty"`
	plain   string            //nolint:unused // unexported on purpose
}

// TestMustToJSONBytesWithSecrets_RevealsPlainText verifies storage marshal
// emits the resolved secret values (SecretString.MarshalJSON would emit the
// "******" mask and destroy them on persist).
func TestMustToJSONBytesWithSecrets_RevealsPlainText(t *testing.T) {
	in := &secretHolder{
		Name:   "cluster-1",
		APIKey: ucfg.SecretString("tok-plain"),
		Nested: &secretNested{
			Password: ucfg.SecretString("pw-plain"),
			Tokens:   []ucfg.SecretString{ucfg.SecretString("t1"), ucfg.SecretString("t2")},
		},
		Headers: map[string]ucfg.SecretString{"auth": ucfg.SecretString("h1")},
	}

	out := MustToJSONBytesWithSecrets(in)

	var m map[string]interface{}
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if m["api_key"] != "tok-plain" {
		t.Fatalf("api_key = %v, want tok-plain", m["api_key"])
	}
	nested, _ := m["nested"].(map[string]interface{})
	if nested["password"] != "pw-plain" {
		t.Fatalf("nested.password = %v, want pw-plain", nested["password"])
	}
	tokens, _ := nested["tokens"].([]interface{})
	if len(tokens) != 2 || tokens[0] != "t1" || tokens[1] != "t2" {
		t.Fatalf("nested.tokens = %v, want [t1 t2]", nested["tokens"])
	}
	headers, _ := m["headers"].(map[string]interface{})
	if headers["auth"] != "h1" {
		t.Fatalf("headers.auth = %v, want h1", m["headers"])
	}
}

// TestMustToJSONBytesWithSecrets_RoundTrip verifies a struct persisted with
// the helper decodes back with intact Get() values, while normal
// json.Marshal keeps displaying the mask.
func TestMustToJSONBytesWithSecrets_RoundTrip(t *testing.T) {
	in := &secretHolder{APIKey: ucfg.SecretString("real-secret")}

	stored := MustToJSONBytesWithSecrets(in)
	var loaded secretHolder
	if err := json.Unmarshal(stored, &loaded); err != nil {
		t.Fatalf("unmarshal stored: %v", err)
	}
	if loaded.APIKey.Get() != "real-secret" {
		t.Fatalf("round-trip Get() = %q, want real-secret", loaded.APIKey.Get())
	}

	// Display marshal stays masked.
	display, err := json.Marshal(&loaded)
	if err != nil {
		t.Fatal(err)
	}
	if !jsonContains(display, ucfg.SecretShadowText) {
		t.Fatalf("display marshal should keep the mask, got %s", display)
	}
}

// TestMustToJSONBytesWithSecrets_DoesNotMutateInput verifies the input
// object is untouched (secrets revealed only in the marshaling copy).
func TestMustToJSONBytesWithSecrets_DoesNotMutateInput(t *testing.T) {
	in := &secretHolder{APIKey: ucfg.SecretString("real-secret")}
	_ = MustToJSONBytesWithSecrets(in)

	display, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if !jsonContains(display, ucfg.SecretShadowText) {
		t.Fatalf("input was mutated: display = %s", display)
	}
}

func jsonContains(b []byte, sub string) bool {
	return len(sub) == 0 || stringContains(string(b), sub)
}

func stringContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
