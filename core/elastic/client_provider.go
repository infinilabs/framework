/* Copyright © INFINI LTD. All rights reserved. */

package elastic

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

// ──────────────────────────────────────────────────────────────────────────
// Client provider injection.
//
// The client factory (version probing + adapter selection, e.g.
// modules/elastic/common.InitClientWithConfig) lives in modules/elastic,
// which depends on this package — so it cannot be imported here without a
// dependency cycle. Instead, the module registers its factory once at setup
// time and this package exposes the config-keyed client cache on top of it.
//
// GetOrCreateClient therefore gives any caller (inline/stream config, a
// storage-backed cluster, or an ad-hoc probe config) a live, self-contained
// client without registering the cluster by ID first.
// ──────────────────────────────────────────────────────────────────────────

// ClientProvider builds a live client from a connection config. Registered
// by modules/elastic at module setup (before its enabled check, so apps that
// disable the elastic module but still talk to ES keep working).
type ClientProvider func(cfg ElasticsearchConfig) (API, error)

var (
	clientProvider   ClientProvider
	clientProviderMu sync.RWMutex
)

// RegisterClientProvider installs the client factory. Panics on nil or on a
// duplicate registration — double wiring is a programming error.
func RegisterClientProvider(p ClientProvider) {
	if p == nil {
		panic("elastic: RegisterClientProvider called with nil provider")
	}
	clientProviderMu.Lock()
	defer clientProviderMu.Unlock()
	if clientProvider != nil {
		panic("elastic: client provider already registered")
	}
	clientProvider = p
}

// registeredProvider returns the installed factory, or an error explaining
// what is missing (clearer than a nil-pointer panic at the call site).
func registeredProvider() (ClientProvider, error) {
	clientProviderMu.RLock()
	defer clientProviderMu.RUnlock()
	if clientProvider == nil {
		return nil, errors.New("no client provider registered; modules/elastic registers one via elastic.RegisterClientProvider at setup")
	}
	return clientProvider, nil
}

// clientCache reuses clients across calls that hand in identical configs.
// Keyed by connection identity (see configCacheKey), not by cluster ID.
// Bounded eviction (LRU/TTL) can be layered on later; for now a simple map
// since the number of distinct connection identities is small.
var (
	clientCache   = map[string]API{}
	clientCacheMu sync.RWMutex
)

// GetOrCreateClient returns a live ES client for the given config, building
// one on the first request and reusing it for subsequent calls with the same
// connection identity.
//
// Unlike GetElasticClient(clusterID), the cluster does NOT need to be
// registered: the returned client is self-contained — its metadata is built
// from the config and attached via SetMetadata, so it never falls back to the
// ID-based registry (which would panic if the cluster isn't registered).
//
// Set cfg.Version (and cfg.Distribution) to skip the network version probe
// when the caller already knows them; otherwise the provider probes the
// cluster once (on cache miss) to select the right version-specific adapter.
func GetOrCreateClient(cfg ElasticsearchConfig) (API, error) {
	provider, err := registeredProvider()
	if err != nil {
		return nil, err
	}

	key := configCacheKey(cfg)

	clientCacheMu.RLock()
	if c, ok := clientCache[key]; ok {
		clientCacheMu.RUnlock()
		return c, nil
	}
	clientCacheMu.RUnlock()

	client, err := provider(cfg)
	if err != nil {
		return nil, err
	}

	// Make the client self-contained: attach a metadata built from the config
	// so GetMetadata() returns it directly instead of looking the config up by
	// ID (and panicking if the cluster isn't registered).
	meta := &ElasticsearchMetadata{Config: &cfg}
	if s, ok := client.(interface {
		SetMetadata(*ElasticsearchMetadata)
	}); ok {
		s.SetMetadata(meta)
	}

	clientCacheMu.Lock()
	// A concurrent caller may have built one first; keep the winner.
	if existing, ok := clientCache[key]; ok {
		clientCacheMu.Unlock()
		return existing, nil
	}
	clientCache[key] = client
	clientCacheMu.Unlock()
	return client, nil
}

// InvalidateClient drops the cached client (if any) for the given config, so
// the next GetOrCreateClient rebuilds it — e.g. after a cluster's endpoint or
// credentials change.
func InvalidateClient(cfg ElasticsearchConfig) {
	key := configCacheKey(cfg)
	clientCacheMu.Lock()
	delete(clientCache, key)
	clientCacheMu.Unlock()
}

// ResetClientCacheForTest clears the client cache and unregisters the client
// provider. Test-only.
func ResetClientCacheForTest() {
	clientCacheMu.Lock()
	clientCache = map[string]API{}
	clientCacheMu.Unlock()
	clientProviderMu.Lock()
	clientProvider = nil
	clientProviderMu.Unlock()
}

// configCacheKey returns a canonical identity for a config's connection: two
// configs that reach the same cluster with the same auth/version share a key
// (and thus a client). Secrets are part of the key (held in memory only, never
// logged) so different credentials get different clients.
func configCacheKey(cfg ElasticsearchConfig) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s|%s|%s|", cfg.GetAnyEndpoint(), strings.Join(cfg.Endpoints, ","), cfg.Distribution)
	if cfg.BasicAuth != nil {
		fmt.Fprintf(&b, "%s:%s|", cfg.BasicAuth.Username, cfg.BasicAuth.Password.Get())
	}
	fmt.Fprintf(&b, "%s|%s", cfg.Token.Get(), cfg.Version)
	return b.String()
}
