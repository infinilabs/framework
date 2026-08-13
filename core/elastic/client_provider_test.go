/* Copyright © INFINI LTD. All rights reserved. */

package elastic

import (
	"errors"
	"testing"
)

func mkCfg(endpoint, version string) ElasticsearchConfig {
	c := ElasticsearchConfig{}
	c.ID = "test-" + endpoint
	c.Name = endpoint
	c.Endpoint = endpoint
	c.Distribution = Elasticsearch
	c.Version = version // set → provider skips the network version probe
	c.Enabled = true
	return c
}

// stubClient satisfies the fat API interface by embedding it (nil); tests
// only exercise pointer identity and metadata plumbing, never API calls.
type stubClient struct {
	API
	meta *ElasticsearchMetadata
}

func (s *stubClient) SetMetadata(m *ElasticsearchMetadata) { s.meta = m }
func (s *stubClient) GetMetadata() *ElasticsearchMetadata  { return s.meta }

// stubProvider returns a fresh client per call so tests can tell builds apart.
func stubProvider(cfg ElasticsearchConfig) (API, error) {
	return &stubClient{}, nil
}

func TestConfigCacheKey(t *testing.T) {
	a := mkCfg("http://a:9200", "8.0.0")
	a2 := mkCfg("http://a:9200", "8.0.0")
	b := mkCfg("http://b:9200", "8.0.0")

	if configCacheKey(a) != configCacheKey(a2) {
		t.Fatal("identical configs must share a key")
	}
	if configCacheKey(a) == configCacheKey(b) {
		t.Fatal("different endpoints must produce different keys")
	}

	a3 := mkCfg("http://a:9200", "9.0.0")
	if configCacheKey(a) == configCacheKey(a3) {
		t.Fatal("different versions must produce different keys")
	}
}

func TestGetOrCreateClient_CachesByConfig(t *testing.T) {
	ResetClientCacheForTest()
	RegisterClientProvider(stubProvider)
	cfg := mkCfg("http://x:9200", "8.0.0")

	c1, err := GetOrCreateClient(cfg)
	if err != nil {
		t.Fatalf("first GetOrCreateClient: %v", err)
	}
	c2, err := GetOrCreateClient(cfg)
	if err != nil {
		t.Fatalf("second GetOrCreateClient: %v", err)
	}
	if c1 != c2 {
		t.Fatal("same config should return the same cached client pointer")
	}

	other := mkCfg("http://y:9200", "8.0.0")
	c3, err := GetOrCreateClient(other)
	if err != nil {
		t.Fatalf("GetOrCreateClient(other): %v", err)
	}
	if c1 == c3 {
		t.Fatal("different config should return a different client")
	}
}

func TestGetOrCreateClient_SelfContained(t *testing.T) {
	ResetClientCacheForTest()
	RegisterClientProvider(stubProvider)
	cfg := mkCfg("http://self:9200", "8.0.0")

	client, err := GetOrCreateClient(cfg)
	if err != nil {
		t.Fatalf("GetOrCreateClient: %v", err)
	}

	// The client must carry its own metadata built from the config — it should
	// NOT need the cluster to be registered by ID (no RegisterInstance ran).
	mg, ok := client.(interface {
		GetMetadata() *ElasticsearchMetadata
	})
	if !ok {
		t.Fatal("client does not expose GetMetadata")
	}
	md := mg.GetMetadata()
	if md == nil || md.Config == nil || md.Config.Endpoint != "http://self:9200" {
		t.Fatalf("self-contained metadata not set from config: %+v", md)
	}
}

func TestGetOrCreateClient_ProviderErrorsPropagate(t *testing.T) {
	ResetClientCacheForTest()
	RegisterClientProvider(func(cfg ElasticsearchConfig) (API, error) {
		return nil, errors.New("boom")
	})
	if _, err := GetOrCreateClient(mkCfg("http://err:9200", "8.0.0")); err == nil {
		t.Fatal("provider error must propagate")
	}
}

func TestGetOrCreateClient_NoProviderRegistered(t *testing.T) {
	ResetClientCacheForTest()
	_, err := GetOrCreateClient(mkCfg("http://none:9200", "8.0.0"))
	if err == nil || !contains(err.Error(), "no client provider registered") {
		t.Fatalf("expected a clear missing-provider error, got: %v", err)
	}
}

func TestRegisterClientProvider_DoubleRegistrationPanics(t *testing.T) {
	ResetClientCacheForTest()
	RegisterClientProvider(stubProvider)
	defer func() {
		if recover() == nil {
			t.Fatal("duplicate registration must panic")
		}
		ResetClientCacheForTest()
	}()
	RegisterClientProvider(stubProvider)
}

func TestInvalidateClient(t *testing.T) {
	ResetClientCacheForTest()
	RegisterClientProvider(stubProvider)
	cfg := mkCfg("http://inv:9200", "8.0.0")

	c1, _ := GetOrCreateClient(cfg)
	InvalidateClient(cfg)
	c2, _ := GetOrCreateClient(cfg)

	if c1 == c2 {
		t.Fatal("InvalidateClient should force a rebuild on next GetOrCreateClient")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
