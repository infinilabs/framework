/* Copyright © INFINI LTD. All rights reserved. */

package elastic

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
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
	// GetAnyEndpoint panics on endpoint-less configs (a programming error
	// when building clients), but cache invalidation may legitimately see
	// stripped-down records (e.g. a delete hook handed an ID-only model) —
	// degrade to an empty endpoint there instead of panicking.
	anyEndpoint := ""
	if cfg.Endpoint != "" || len(cfg.Endpoints) > 0 {
		anyEndpoint = cfg.GetAnyEndpoint()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s|%s|%s|", anyEndpoint, strings.Join(cfg.Endpoints, ","), cfg.Distribution)
	if cfg.BasicAuth != nil {
		fmt.Fprintf(&b, "%s:%s|", cfg.BasicAuth.Username, cfg.BasicAuth.Password.Get())
	}
	fmt.Fprintf(&b, "%s|%s", cfg.Token.Get(), cfg.Version)
	return b.String()
}

// SameConnectionIdentity reports whether two configs would share a cached
// client — same endpoints, credentials, distribution and version (see
// configCacheKey). Used by cluster-change hooks to skip re-initialization
// when only non-connection fields (labels, health status) changed.
func SameConnectionIdentity(a, b ElasticsearchConfig) bool {
	return configCacheKey(a) == configCacheKey(b)
}

// probeTransport is the HTTP transport for connectivity probes: TLS
// verification disabled (ES clusters commonly use self-signed certs).
var probeTransport = &http.Transport{
	TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
}

// ProbeCluster does a raw GET to the cluster's root endpoint to detect
// connectivity and the server version/distribution, without building a
// version-specific adapter. BasicAuth or X-API-TOKEN is applied when
// present. Shared by pre-registration connectivity tests and any caller
// that needs the version before a client exists.
//
// Note: modules/elastic/adapter.ClusterVersion is a fasthttp-based twin
// used inside the factory's version probing; consolidating the two is
// tracked in the refactor plan (SEARCH_ORM_REFACTOR_PLAN, P3 follow-up).
func ProbeCluster(cfg *ElasticsearchConfig) (version, distribution string, err error) {
	endpoint := cfg.Endpoint
	if endpoint == "" && len(cfg.Endpoints) > 0 {
		endpoint = cfg.Endpoints[0]
	}
	httpReq, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return "", "", err
	}
	if cfg.BasicAuth != nil && cfg.BasicAuth.Username != "" {
		httpReq.SetBasicAuth(cfg.BasicAuth.Username, cfg.BasicAuth.Password.Get())
	} else if t := cfg.Token.Get(); t != "" {
		httpReq.Header.Set("X-API-TOKEN", t)
	}

	client := &http.Client{Timeout: 10 * time.Second, Transport: probeTransport}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return "", "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(truncate(string(body), 200)))
	}
	var info struct {
		Version struct {
			Number       string `json:"number"`
			Distribution string `json:"distribution"`
		} `json:"version"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return "", "", fmt.Errorf("parse version response: %w", err)
	}
	return info.Version.Number, info.Version.Distribution, nil
}

// truncate caps s to n runes, appending "…" when truncated.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
