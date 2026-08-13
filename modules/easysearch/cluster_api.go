/* Copyright © INFINI LTD. All rights reserved. */

package easysearch

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security"
	"infini.sh/framework/core/util"
)

// ──────────────────────────────────────────────────────────────────────────
// Cluster management REST API — public, reusable, ORM-backed.
//
// CRUD over ElasticsearchConfig clusters using only the standard ORM (sqlite
// or any non-elastic store). Routes use the /easysearch/ prefix (not
// /elasticsearch/) so they don't collide with legacy /elasticsearch/ routes.
//
// Handler conventions follow the standard module CRUD pattern (see
// coco/modules/integration): Write*JSON response envelopes, partial-field
// updates, and a _search endpoint driven by orm.NewQueryBuilderFromRequest
// (pagination/sort/filter/full-text via query string or request body).
//
// Decoupled from modules/elastic: no live-client registration, no in-memory
// metadata registry. Health status shown in responses is the value persisted
// on each cluster's Labels by the elasticsearch module's health loop.
// ──────────────────────────────────────────────────────────────────────────

// ClusterAPI provides REST CRUD for ElasticsearchConfig clusters. It embeds
// api.Handler for DecodeJSON/WriteJSON/WriteError.
type ClusterAPI struct {
	api.Handler
}

// registerClusterAPI registers the /easysearch/ CRUD routes. Called from
// Module.Setup so every app that registers the easysearch module gets cluster
// management. The /_test route is public (AllowPublicAccess) because it's a
// pre-registration connectivity probe (no cluster exists yet).
func registerClusterAPI() {

	// Generic permission keys for cluster management. Apps assign these to roles
	// to control who can manage clusters.
	var (
		permClusterRead   = security.GetOrInitPermission("generic", "easysearch:cluster", security.Read)
		permClusterCreate = security.GetOrInitPermission("generic", "easysearch:cluster", security.Create)
		permClusterUpdate = security.GetOrInitPermission("generic", "easysearch:cluster", security.Update)
		permClusterDelete = security.GetOrInitPermission("generic", "easysearch:cluster", security.Delete)
		permClusterSearch = security.GetOrInitPermission("generic", "easysearch:cluster", security.Search)
	)

	h := &ClusterAPI{}
	api.HandleUIMethod(api.POST, "/easysearch/_test", h.testConnection, api.RequirePermission(permClusterRead), api.AllowOPTIONSS(), api.Feature(api.FeatureCORS))
	api.HandleUIMethod(api.POST, "/easysearch/", h.createCluster, api.RequirePermission(permClusterCreate))
	api.HandleUIMethod(api.GET, "/easysearch/_search", h.searchClusters, api.RequirePermission(permClusterSearch))
	api.HandleUIMethod(api.POST, "/easysearch/_search", h.searchClusters, api.RequirePermission(permClusterSearch))
	api.HandleUIMethod(api.GET, "/easysearch/:id", h.getCluster, api.RequirePermission(permClusterRead))
	api.HandleUIMethod(api.PUT, "/easysearch/:id", h.updateCluster, api.RequirePermission(permClusterUpdate))
	api.HandleUIMethod(api.DELETE, "/easysearch/:id", h.deleteCluster, api.RequirePermission(permClusterDelete))
}

// createCluster — POST /easysearch/
// Persists a cluster record. It becomes a live ES client when the
// elasticsearch module next loads clusters from the ORM (boot/reload).
func (h *ClusterAPI) createCluster(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var cfg elastic.ElasticsearchConfig
	if err := h.DecodeJSON(req, &cfg); err != nil {
		h.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if cfg.Name == "" {
		h.WriteError(w, "name is required", http.StatusBadRequest)
		return
	}
	if cfg.ID == "" {
		cfg.ID = util.GetUUID()
	}
	if cfg.Distribution == "" {
		cfg.Distribution = elastic.Elasticsearch
	}
	// Mark as dynamically-managed so the elasticsearch module's health loop
	// persists status for it (the loop keys off this source value).
	cfg.Source = elastic.ElasticsearchConfigSourceElasticsearch
	cfg.Enabled = true

	ctx := orm.NewContextWithParent(req.Context())
	ctx.Refresh = orm.WaitForRefresh
	if err := orm.Create(ctx, &cfg); err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.WriteCreatedOKJSON(w, cfg.ID)
}

// searchClusters — GET/POST /easysearch/_search
// Standard query-builder search (pagination, sorting, filtering, full-text on
// the name field via ?query=). Returns an ES-shaped SearchResponse; each hit's
// _source carries the cluster with its persisted health_status (in Labels)
// from the elasticsearch module's health loop.
func (h *ClusterAPI) searchClusters(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	//handle url query args, convert to query builder
	builder, err := orm.NewQueryBuilderFromRequest(req, "name")
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	builder.EnableBodyBytes()
	if len(builder.Sorts()) == 0 {
		builder.SortBy(orm.Sort{Field: "created", SortType: orm.DESC})
	}

	ctx := orm.NewContextWithParent(req.Context())
	orm.WithModel(ctx, &elastic.ElasticsearchConfig{})
	res, err := orm.SearchV2(ctx, builder)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	searchRes, err := parseSearchResponse(res)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.WriteJSON(w, searchRes, http.StatusOK)
}

// getCluster — GET /easysearch/:id
func (h *ClusterAPI) getCluster(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	cfg := elastic.ElasticsearchConfig{}
	cfg.ID = id
	ctx := orm.NewContextWithParent(req.Context())
	exists, err := orm.GetV2(ctx, &cfg)
	if !exists || err != nil {
		h.WriteGetMissingJSON(w, id)
		return
	}
	h.WriteGetOKJSON(w, id, cfg)
}

// updateCluster — PUT /easysearch/:id
// Partial update: only the fields present in the request body are changed
// (orm.UpdatePartialFields merges the delta onto the stored record). Secrets
// omitted from the body — e.g. the password, which GET responses return
// masked — keep their stored values, so a partial update never breaks the
// connection.
func (h *ClusterAPI) updateCluster(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	obj := elastic.ElasticsearchConfig{}
	obj.ID = id

	delta := util.MapStr{}
	if err := h.DecodeJSON(req, &delta); err != nil {
		h.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := orm.NewContextWithParent(req.Context())
	ctx.Refresh = orm.WaitForRefresh
	if err := orm.UpdatePartialFields(ctx, &obj, delta); err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.WriteOpRecordNotFoundJSON(w, id)
			return
		}
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.WriteUpdatedOKJSON(w, obj.ID)
}

// deleteCluster — DELETE /easysearch/:id
// Removes the cluster record. Reserved clusters (e.g. the system cluster)
// cannot be deleted.
func (h *ClusterAPI) deleteCluster(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	cfg := elastic.ElasticsearchConfig{}
	cfg.ID = id
	ctx := orm.NewContextWithParent(req.Context())
	exists, err := orm.GetV2(ctx, &cfg)
	if !exists || err != nil {
		h.WriteOpRecordNotFoundJSON(w, id)
		return
	}
	if cfg.Reserved {
		h.WriteError(w, "reserved cluster cannot be deleted", http.StatusForbidden)
		return
	}
	ctx.Refresh = orm.WaitForRefresh
	if err := orm.Delete(ctx, &cfg); err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.WriteDeletedOKJSON(w, id)
}

// testConnection — POST /easysearch/_test
// Pre-registration connectivity + version probe via a raw HTTP request. Does
// NOT persist or register the cluster. Public so it works before login is
// required (callers use it to validate a form before creating).
func (h *ClusterAPI) testConnection(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var cfg elastic.ElasticsearchConfig
	if err := h.DecodeJSON(req, &cfg); err != nil {
		h.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}
	endpoint := cfg.Endpoint
	if endpoint == "" && len(cfg.Endpoints) > 0 {
		endpoint = cfg.Endpoints[0]
	}
	if endpoint == "" {
		h.WriteError(w, "endpoint is required", http.StatusBadRequest)
		return
	}
	if cfg.Distribution == "" {
		cfg.Distribution = elastic.Elasticsearch
	}

	version, distribution, err := probeCluster(&cfg)
	if err != nil {
		h.WriteJSON(w, map[string]interface{}{
			"connected":    false,
			"error":        err.Error(),
			"distribution": cfg.Distribution,
		}, http.StatusOK)
		return
	}
	if distribution == "" {
		distribution = cfg.Distribution
	}
	h.WriteJSON(w, map[string]interface{}{
		"connected":    true,
		"version":      version,
		"distribution": distribution,
	}, http.StatusOK)
}

// ──────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────

// parseSearchResponse decodes an orm.SearchResult payload (ES-shaped JSON:
// {"hits":{"hits":[{"_source":{...}}]}}) into elastic.SearchResponse. Accepts
// both []byte and string payloads; nil/empty payloads yield a zero response.
func parseSearchResponse(res *orm.SearchResult) (elastic.SearchResponse, error) {
	out := elastic.SearchResponse{}
	if res == nil {
		return out, nil
	}
	var raw []byte
	switch payload := res.Payload.(type) {
	case []byte:
		raw = payload
	case string:
		raw = []byte(payload)
	default:
		return out, nil
	}
	if len(raw) == 0 {
		return out, nil
	}
	if err := util.FromJSONBytes(raw, &out); err != nil {
		return elastic.SearchResponse{}, err
	}
	return out, nil
}

// probeTransport is the HTTP transport for connectivity probes: TLS verification
// disabled (ES clusters commonly use self-signed certs).
var probeTransport = &http.Transport{
	TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
}

// probeCluster does a raw GET to the cluster's root endpoint to detect
// connectivity and the server version/distribution, without building a
// version-specific adapter. BasicAuth or X-API-TOKEN is applied when present.
func probeCluster(cfg *elastic.ElasticsearchConfig) (version, distribution string, err error) {
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
