/* Copyright © INFINI LTD. All rights reserved. */

package elastic

import (
	"testing"

	"infini.sh/framework/core/elastic"
	"infini.sh/framework/modules/elastic/common"
)

// TestClientProviderWiring proves the factory hand-off end to end: the module
// registers common.InitClientWithConfig as the core client provider, and
// core.GetOrCreateClient builds a self-contained client from a bare config
// (version preset → no network probe, fully offline).
func TestClientProviderWiring(t *testing.T) {
	elastic.ResetClientCacheForTest()
	elastic.RegisterClientProvider(common.InitClientWithConfig)

	cfg := elastic.ElasticsearchConfig{}
	cfg.ID = "wiring-test"
	cfg.Name = "wiring-test"
	cfg.Endpoint = "http://wiring-test:9200"
	cfg.Distribution = elastic.Elasticsearch
	cfg.Version = "8.0.0" // preset → adapter selection without probing
	cfg.Enabled = true

	c1, err := elastic.GetOrCreateClient(cfg)
	if err != nil {
		t.Fatalf("GetOrCreateClient via real factory: %v", err)
	}
	if c1 == nil {
		t.Fatal("factory returned a nil client")
	}

	// Self-contained: metadata comes from the config, no RegisterInstance ran.
	mg, ok := c1.(interface {
		GetMetadata() *elastic.ElasticsearchMetadata
	})
	if !ok {
		t.Fatal("client does not expose GetMetadata")
	}
	md := mg.GetMetadata()
	if md == nil || md.Config == nil || md.Config.Endpoint != cfg.Endpoint {
		t.Fatalf("self-contained metadata not set from config: %+v", md)
	}

	// Same config → same cached client; invalidate → rebuilt.
	c2, err := elastic.GetOrCreateClient(cfg)
	if err != nil || c2 != c1 {
		t.Fatalf("same config must return the cached client (c2==c1: %v, err: %v)", c2 == c1, err)
	}
	elastic.InvalidateClient(cfg)
	c3, err := elastic.GetOrCreateClient(cfg)
	if err != nil || c3 == c1 {
		t.Fatalf("after invalidate a new client must be built (c3==c1: %v, err: %v)", c3 == c1, err)
	}
}
