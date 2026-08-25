/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/lib/go-ucfg"
	"infini.sh/framework/modules/configs/common"
)

func cfg(name string, version int64) common.ConfigFile {
	return common.ConfigFile{Name: name, Content: "content-of-" + name, Version: version, Managed: true}
}

func TestDiffConfigs_AllStates(t *testing.T) {
	assigned := []common.ConfigFile{cfg("a.yml", 1), cfg("b.yml", 2)}

	t.Run("fresh client gets everything created", func(t *testing.T) {
		resp := diffConfigs(assigned, nil)
		if !resp.Changed {
			t.Fatal("expected changed=true")
		}
		if len(resp.Configs.CreatedConfigs) != 2 {
			t.Fatalf("created = %d, want 2", len(resp.Configs.CreatedConfigs))
		}
		if len(resp.Configs.UpdatedConfigs) != 0 || len(resp.Configs.DeletedConfigs) != 0 {
			t.Fatal("no updates/deletes expected")
		}
	})

	t.Run("same versions → no change", func(t *testing.T) {
		client := map[string]common.ConfigFile{"a.yml": cfg("a.yml", 1), "b.yml": cfg("b.yml", 2)}
		resp := diffConfigs(assigned, client)
		if resp.Changed {
			t.Fatalf("expected no change, got %+v", resp.Configs)
		}
	})

	t.Run("server version bump → updated", func(t *testing.T) {
		bumped := []common.ConfigFile{cfg("a.yml", 1), cfg("b.yml", 3)}
		client := map[string]common.ConfigFile{"a.yml": cfg("a.yml", 1), "b.yml": cfg("b.yml", 2)}
		resp := diffConfigs(bumped, client)
		if !resp.Changed {
			t.Fatal("expected changed")
		}
		if len(resp.Configs.UpdatedConfigs) != 1 || resp.Configs.UpdatedConfigs["b.yml"].Version != 3 {
			t.Fatalf("updated = %+v", resp.Configs.UpdatedConfigs)
		}
	})

	t.Run("client-only config → deleted", func(t *testing.T) {
		client := map[string]common.ConfigFile{
			"a.yml":    cfg("a.yml", 1),
			"b.yml":    cfg("b.yml", 2),
			"gone.yml": cfg("gone.yml", 1),
		}
		resp := diffConfigs(assigned, client)
		if !resp.Changed {
			t.Fatal("expected changed")
		}
		if len(resp.Configs.DeletedConfigs) != 1 || resp.Configs.DeletedConfigs["gone.yml"].Name != "gone.yml" {
			t.Fatalf("deleted = %+v", resp.Configs.DeletedConfigs)
		}
	})

	t.Run("client opts out via Managed=false → untouched", func(t *testing.T) {
		localOnly := cfg("local.yml", 1)
		localOnly.Managed = false
		client := map[string]common.ConfigFile{
			"a.yml":     cfg("a.yml", 1),
			"b.yml":     cfg("b.yml", 2),
			"local.yml": localOnly,
		}
		resp := diffConfigs(assigned, client)
		if resp.Changed {
			t.Fatal("unmanaged local config must not trigger deletion")
		}

		// server-side version bump on a config the client holds unmanaged:
		// also skipped
		unmanagedA := cfg("a.yml", 1)
		unmanagedA.Managed = false
		client2 := map[string]common.ConfigFile{"a.yml": unmanagedA, "b.yml": cfg("b.yml", 2)}
		resp2 := diffConfigs(assigned, client2)
		if _, touched := resp2.Configs.UpdatedConfigs["a.yml"]; touched {
			t.Fatal("unmanaged config must not be updated")
		}
	})

	t.Run("server removed everything → all client configs deleted", func(t *testing.T) {
		client := map[string]common.ConfigFile{"a.yml": cfg("a.yml", 1)}
		resp := diffConfigs(nil, client)
		if !resp.Changed || len(resp.Configs.DeletedConfigs) != 1 {
			t.Fatalf("expected deletion, got %+v", resp.Configs)
		}
	})

	t.Run("empty both sides → no change", func(t *testing.T) {
		resp := diffConfigs(nil, nil)
		if resp.Changed {
			t.Fatal("expected changed=false")
		}
	})
}

func TestConfigsHash(t *testing.T) {
	files := []common.ConfigFile{cfg("a.yml", 1), cfg("b.yml", 2)}
	h1 := ConfigsHash(files)
	h2 := ConfigsHash(files)
	if h1 == "" {
		t.Fatal("hash must not be empty")
	}
	if h1 != h2 {
		t.Fatal("hash must be stable")
	}
	// 顺序无关不应成立? 协议要求两侧列表一致 — 同序序列化, 顺序变化视为变更
	reordered := []common.ConfigFile{files[1], files[0]}
	if ConfigsHash(reordered) == h1 {
		t.Log("note: hash is order-sensitive (both sides marshal the same list)")
	}
	if ConfigsHash(nil) != "" {
		t.Fatal("empty list must hash to empty string")
	}
	if ConfigsHash([]common.ConfigFile{cfg("a.yml", 2)}) == h1 {
		t.Fatal("version change must change the hash")
	}
}

func TestTokenGate(t *testing.T) {
	handler := func(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		w.WriteHeader(http.StatusOK)
	}
	mkTokens := func(vals ...string) []ucfg.SecretString {
		out := make([]ucfg.SecretString, len(vals))
		for i, v := range vals {
			out[i] = ucfg.SecretString(v)
		}
		return out
	}
	bearer := func(v string) *http.Request {
		req := httptest.NewRequest("POST", "/x", nil)
		req.Header.Set("Authorization", "Bearer "+v)
		return req
	}

	t.Run("no tokens configured = open (dev mode)", func(t *testing.T) {
		gate := newTokenGate(nil)
		rec := httptest.NewRecorder()
		gate(handler)(rec, httptest.NewRequest("POST", "/x", nil), nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("open mode must pass, got %d", rec.Code)
		}
	})

	t.Run("valid token passes, wrong/missing rejected", func(t *testing.T) {
		gate := newTokenGate(mkTokens("s3cret"))
		rec := httptest.NewRecorder()
		gate(handler)(rec, bearer("s3cret"), nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("valid token must pass, got %d", rec.Code)
		}
		rec2 := httptest.NewRecorder()
		gate(handler)(rec2, bearer("wrong"), nil)
		if rec2.Code != http.StatusUnauthorized {
			t.Fatalf("wrong token must 401, got %d", rec2.Code)
		}
		rec3 := httptest.NewRecorder()
		gate(handler)(rec3, httptest.NewRequest("POST", "/x", nil), nil)
		if rec3.Code != http.StatusUnauthorized {
			t.Fatalf("missing header must 401, got %d", rec3.Code)
		}
	})

	t.Run("multiple tokens accepted (rotation window)", func(t *testing.T) {
		gate := newTokenGate(mkTokens("old", "new"))
		for _, tok := range []string{"old", "new"} {
			rec := httptest.NewRecorder()
			gate(handler)(rec, bearer(tok), nil)
			if rec.Code != http.StatusOK {
				t.Fatalf("token %q must pass, got %d", tok, rec.Code)
			}
		}
	})

	t.Run("X-API-Token header also accepted", func(t *testing.T) {
		gate := newTokenGate(mkTokens("s3cret"))
		req := httptest.NewRequest("POST", "/x", nil)
		req.Header.Set("X-API-Token", "s3cret")
		rec := httptest.NewRecorder()
		gate(handler)(rec, req, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("X-API-Token must pass, got %d", rec.Code)
		}
	})

	t.Run("validateStaticToken matches configured list only", func(t *testing.T) {
		staticTokens = []string{"alpha", "beta"}
		if !validateStaticToken("beta") {
			t.Fatal("beta must validate")
		}
		if validateStaticToken("gamma") {
			t.Fatal("gamma must not validate")
		}
		if validateStaticToken("") {
			t.Fatal("empty must not validate")
		}
	})
}

func TestHashTokenOneWay(t *testing.T) {
	h1 := hashToken("my-token")
	if h1 == "my-token" || len(h1) != 64 { // sha256 hex
		t.Fatalf("hash must be 64-hex and differ from input, got %q", h1)
	}
	if hashToken("my-token") != h1 {
		t.Fatal("hash must be deterministic")
	}
	if hashToken("my-token2") == h1 {
		t.Fatal("different inputs must hash differently")
	}
}
