/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

// Package server implements the SERVER side of the standard managed-config
// protocol: the counterpart of modules/configs/client that every framework
// process runs when `configs.managed: true`.
//
// Routes (protocol-compatible with Console's managed plugin; Console keeps
// its richer implementation — token exchange, script hooks, websocket
// proxy — on its own product surface, this one carries the portable core
// any product can embed):
//
//	POST /instance/_register   self-description registration (upsert)
//	POST /configs/_sync        heartbeat + config diff delivery
//
// Sync semantics (mirrors the Console contract):
//   - the client posts its current managed configs + a hash of them
//   - the server replies {changed, configs:{created,updated,deleted}}
//   - unchanged content is skipped by version comparison; a config the
//     client marked Managed=false is never touched
//   - every sync refreshes the instance record (heartbeat via labels)
package server

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/env"
	log "infini.sh/framework/core/log"
	"infini.sh/framework/core/model"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/go-ucfg"
	"infini.sh/framework/modules/configs/common"
)

// Config holds the server-side settings (configs.server.* in YAML).
// Disabled by default; products opt in via Setup().
type Config struct {
	Enabled bool `config:"enabled"`

	// Auth gates both protocol routes (_register/_sync) with the standard
	// Bearer token mechanism: clients present
	//   Authorization: Bearer <token>   (or X-API-Token: <token>)
	// and the server constant-time-compares against the configured token
	// list. Multiple tokens allow zero-downtime rotation (add the new
	// token, roll the clients, remove the old one). Deployments MUST
	// configure auth.tokens in production: without it any host that
	// reaches the port can register instances and pull every assigned
	// config. When unset the server logs a warning and runs in open dev
	// mode. The framework client sends its token automatically via
	// configs.manager.token.
	Auth struct {
		Tokens []ucfg.SecretString `config:"tokens"`
	} `config:"auth"`
}

// ManagedConfig is one config file assigned to an instance (or "*", all
// instances). Version bumps on every content change; clients apply
// Created/Updated diffs by version comparison.
type ManagedConfig struct {
	orm.ORMObjectBase

	InstanceID string `json:"instance_id" elastic_mapping:"instance_id:{type:keyword}"` // target instance id, or "*" for all
	Name       string `json:"name" elastic_mapping:"name:{type:keyword}"`               // config file name, e.g. pipeline.yml
	Location   string `json:"location,omitempty" elastic_mapping:"location:{type:keyword}"`
	Content    string `json:"content,omitempty" elastic_mapping:"content:{type:text}"`
	Version    int64  `json:"version" elastic_mapping:"version:{type:long}"`
	Readonly   bool   `json:"readonly,omitempty" elastic_mapping:"readonly:{type:boolean}"`
}

// Label keys for heartbeat state on the instance record (model.Instance
// has no dedicated online-state fields; labels keep the wire type intact).
const (
	LabelLastSyncAt = "managed_last_sync_at"
	LabelRegistered = "managed_registered"
)

// Instance admission states (Instance.Status).
const (
	StatusPending  = "pending"  // registered, awaiting admin approval
	StatusApproved = "approved" // admitted; full sync + credentials
)

// AllInstancesID assigns a ManagedConfig to every syncing instance.
const AllInstancesID = "*"

// instanceTokenExchangeAPI rotates an instance's manager token.
const instanceTokenExchangeAPI = "/instance/_exchange_token"

// instanceApproveAPI admits a pending instance (management action).
const instanceApproveAPI = "/instance/:id/_approve"

type APIHandler struct {
	api.Handler
}

var handler = &APIHandler{}

// Setup registers the ORM schemas and the protocol routes. Call once from
// the product's module setup (e.g. logpilot's init). No-op when
// configs.server.enabled is false in the product config.
func Setup() {
	cfg := Config{Enabled: true}
	exists, err := env.ParseConfig("configs.server", &cfg)
	if err != nil {
		panic(err)
	}
	if exists && !cfg.Enabled {
		log.Debug("configs server disabled by configuration")
		return
	}

	orm.MustRegisterSchemaWithIndexName(model.Instance{}, "instance")
	orm.MustRegisterSchemaWithIndexName(ManagedConfig{}, "managed-configs")
	orm.MustRegisterSchemaWithIndexName(InstanceToken{}, "instance-tokens")

	gate := newTokenGate(cfg.Auth.Tokens)
	registerGate := gate
	if len(cfg.Auth.Tokens) == 0 {
		// Open mode is warned about at startup; the per-instance flow below
		// still mints tokens so closing the server later does not strand
		// instances that already registered.
		registerGate = func(next httprouter.Handle) httprouter.Handle { return next }
	}
	api.HandleAPIMethod(api.POST, common.REGISTER_API, registerGate(handler.registerInstance))
	api.HandleAPIMethod(api.POST, common.SYNC_API, gate(handler.syncConfigs))
	api.HandleAPIMethod(api.POST, instanceTokenExchangeAPI, gate(handler.exchangeTokenHandler))
	// Admission: management UI approves pending instances. The gate admits
	// admins (static token); the handler mints the instance credential.
	api.HandleAPIMethod(api.POST, instanceApproveAPI, gate(handler.approveInstanceHandler))

	api.HandleUIMethod(api.POST, common.REGISTER_API, registerGate(handler.registerInstance))
	api.HandleUIMethod(api.POST, common.SYNC_API, gate(handler.syncConfigs))
	api.HandleUIMethod(api.POST, instanceTokenExchangeAPI, gate(handler.exchangeTokenHandler))
	api.HandleUIMethod(api.POST, instanceApproveAPI, gate(handler.approveInstanceHandler))

	if len(cfg.Auth.Tokens) > 0 {
		log.Infof("configs server ready: %s + %s (bearer token auth, %d token(s) accepted)", common.REGISTER_API, common.SYNC_API, len(cfg.Auth.Tokens))
	} else {
		log.Warnf("configs server ready in OPEN mode (no configs.server.auth.tokens configured) - " +
			"any host reaching this port can register instances and pull assigned configs; configure auth.tokens in production")
	}
}

// staticTokens holds the configured bootstrap/admin tokens (set in Setup).
var staticTokens []string

// validateStaticToken constant-time checks against the configured static
// tokens (bootstrap admission + admin fallback).
func validateStaticToken(token string) bool {
	if token == "" || len(staticTokens) == 0 {
		return false
	}
	matched := 0
	for _, want := range staticTokens {
		matched |= subtle.ConstantTimeCompare([]byte(token), []byte(want))
	}
	return matched == 1
}

// extractBearerToken reads the access token from the standard
// Authorization: Bearer header, falling back to X-API-Token (the framework's
// conventional token header).
func extractBearerToken(req *http.Request) string {
	if h := req.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(h[len("Bearer "):])
	}
	return strings.TrimSpace(req.Header.Get("X-API-Token"))
}

// newTokenGate wraps a protocol handler with constant-time Bearer-token
// validation against the accepted token list. No tokens configured = open
// pass-through (dev mode, loudly warned at startup).
func newTokenGate(tokens []ucfg.SecretString) func(httprouter.Handle) httprouter.Handle {
	if len(tokens) == 0 {
		return func(next httprouter.Handle) httprouter.Handle { return next }
	}
	wants := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if v := t.Get(); v != "" {
			wants = append(wants, v)
		}
	}
	staticTokens = wants
	return func(next httprouter.Handle) httprouter.Handle {
		return func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
			got := extractBearerToken(req)
			if got == "" {
				w.Header().Set("WWW-Authenticate", `Bearer realm="configs"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			// Constant-time compare against EVERY accepted token (all of
			// them, so the response time does not reveal which position
			// matched); any match passes.
			matched := 0
			for _, want := range wants {
				matched |= subtle.ConstantTimeCompare([]byte(got), []byte(want))
			}
			if matched != 1 {
				w.Header().Set("WWW-Authenticate", `Bearer realm="configs"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next(w, req, ps)
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────
// POST /instance/_register
// ──────────────────────────────────────────────────────────────────────────

// registerBody decodes both the wrapped form ({client:{...}}) and the
// legacy plain model.Instance the framework client sends.
type registerBody struct {
	Client model.Instance `json:"client"`
}

func (h *APIHandler) registerInstance(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	// Read the body ONCE: DecodeJSON consumes (and closes) r.Body, so a
	// legacy-payload fallback that re-reads it fails with
	// "invalid Read on closed Body" — which silently broke every bare
	// model.Instance registration (the format deployed agents send).
	body, err := readBody(req)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}
	var instance model.Instance
	if wrapped := struct {
		Client model.Instance `json:"client"`
	}{}; util.FromJSONBytes(body, &wrapped) == nil && wrapped.Client.ID != "" {
		instance = wrapped.Client
	} else if err := util.FromJSONBytes(body, &instance); err != nil || instance.ID == "" {
		h.WriteError(w, "instance id is required (plain Instance or {client:{...}} payload)", http.StatusBadRequest)
		return
	}
	// The managed agent's self-generated API token rides in access_token
	// (framework token management: the agent mints it via
	// access_token.CreateAPIToken at startup and registers it here; the
	// manager stores it for reverse calls — pipeline tasks, stats, proxy).
	if instance.AccessToken != nil && strings.TrimSpace(instance.AccessToken.Value) != "" {
		log.Debugf("configs server: instance %s registered an API access token", instance.ID)
	}

	// An EXISTING instance re-registering must prove identity with its own
	// token (a static token also qualifies — bootstrap admin). A fresh
	// instance is authenticated by the static gate already.
	ormCtx := orm.NewContextWithParent(req.Context()).DirectAccess()
	existingToken := loadInstanceToken(ormCtx, instance.ID)
	if existingToken != nil {
		presented := extractBearerToken(req)
		if !ValidateInstanceToken(ormCtx, instance.ID, presented) && !validateStaticToken(presented) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="configs"`)
			h.WriteError(w, "unauthorized: instance token required to re-register", http.StatusUnauthorized)
			return
		}
	}

	created, err := upsertInstance(&instance)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Admission: a PENDING instance is visible in the management UI but
	// receives no credentials and no configs until an admin approves it.
	approved := instance.Status == StatusApproved

	resp := util.MapStr{
		"id":       instance.ID,
		"approved": approved,
	}
	if approved {
		// Mint (or rotate on re-register) the per-instance token; the
		// response is the only place the plaintext ever appears.
		token, err := MintInstanceToken(ormCtx, instance.ID)
		if err != nil {
			h.WriteError(w, "mint instance token: "+err.Error(), http.StatusInternalServerError)
			return
		}
		resp["manager_token"] = token
	}
	if !created {
		resp["exists"] = true // the framework client treats "exists" as success
	} else {
		resp["created"] = true
	}
	h.WriteJSON(w, resp, http.StatusOK)
}

// upsertInstance persists/refreshes the registration. Returns true when
// the instance was newly created.
func upsertInstance(instance *model.Instance) (bool, error) {
	ctx := orm.NewContext().DirectAccess()
	existing := model.Instance{}
	existing.ID = instance.ID
	exists, err := orm.GetV2(ctx, &existing)
	if err != nil && !isNotFound(err) {
		return false, err
	}
	// not-found is the normal first-registration path, not an error
	exists = err == nil && exists

	now := strconv.FormatInt(time.Now().UnixMilli(), 10)
	if instance.Labels == nil {
		instance.Labels = map[string]string{}
	}
	instance.Labels[LabelRegistered] = now
	instance.Labels[LabelLastSyncAt] = now
	if instance.Status == "" {
		instance.Status = StatusPending
	}

	if exists {
		// keep server-side timestamps; refresh the self-description
		created := existing.Created
		instanceCopy := *instance
		instanceCopy.Created = created
		return false, orm.Save(ctx, &instanceCopy)
	}
	created := time.Now().UTC()
	instance.Created = &created
	return true, orm.Save(ctx, instance)
}

// ──────────────────────────────────────────────────────────────────────────
// POST /configs/_sync
// ──────────────────────────────────────────────────────────────────────────

func (h *APIHandler) syncConfigs(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var obj common.ConfigSyncRequest
	if err := h.DecodeJSON(req, &obj); err != nil {
		h.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if obj.Client.ID == "" {
		h.WriteError(w, "client.id is required", http.StatusBadRequest)
		return
	}

	// Authentication: the static gate has already accepted the caller, but
	// instances that hold per-instance tokens must be checked against them —
	// a revoked static token must not keep a registered instance alive, and
	// conversely a valid instance token must pass even if statics rotate.
	// Accepted credentials: the minted InstanceToken (Bearer, from the
	// register/exchange response) OR the agent's registered self API token
	// (X-API-Token — what the framework client sends before exchange).
	if loadInstanceStatus(orm.NewContext().DirectAccess(), obj.Client.ID) == StatusApproved {
		// Credential enforcement applies to admitted instances only:
		// pending ones hold no paired credential yet and their sync
		// carries nothing sensitive (empty config set) — heartbeat
		// visibility is exactly what pending needs.
		if presentToken := loadInstanceToken(orm.NewContext().DirectAccess(), obj.Client.ID); presentToken != nil {
			presented := extractBearerToken(req)
			if !ValidateInstanceToken(orm.NewContext().DirectAccess(), obj.Client.ID, presented) &&
				!matchesRegisteredAccessToken(orm.NewContext().DirectAccess(), obj.Client.ID, presented) {
				w.Header().Set("WWW-Authenticate", `Bearer realm="configs"`)
				h.WriteError(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
	}

	// Heartbeat: refresh the instance record on every sync.
	if _, err := upsertInstance(&obj.Client); err != nil {
		log.Debugf("configs server: heartbeat upsert failed for %s: %v", obj.Client.ID, err)
	}

	assigned := loadAssignedConfigs(obj.Client.ID)
	if status := loadInstanceStatus(orm.NewContext().DirectAccess(), obj.Client.ID); status != StatusApproved { // empty (legacy) counts as pending
		// Not approved yet (or status unknown): heartbeat counts, configs
		// do not flow. The client keeps re-registering and will pick up
		// approval on the next register/sync cycle.
		assigned = nil
	}

	// Fast path: identical hash and no forced sync → nothing changed.
	serverHash := ConfigsHash(assigned)
	if !obj.ForceSync && obj.Hash != "" && obj.Hash == serverHash {
		h.WriteJSON(w, common.ConfigSyncResponse{Changed: false}, http.StatusOK)
		return
	}

	resp := diffConfigs(assigned, obj.Configs.Configs)
	h.WriteJSON(w, resp, http.StatusOK)
}

// loadAssignedConfigs returns the server-side config files assigned to the
// instance (its own + the "*" catch-all), newest version per name.
func loadAssignedConfigs(instanceID string) []common.ConfigFile {
	ctx := orm.NewContext().DirectAccess()
	orm.WithModel(ctx, &ManagedConfig{})

	qb := orm.NewQuery().
		Filter(orm.TermQuery("instance_id", instanceID)).
		Filter(orm.TermQuery("instance_id", AllInstancesID)).
		Size(1000)
	res, err := orm.SearchV2(ctx, qb)
	if err != nil || res == nil {
		return nil
	}
	stored, _, _ := decodeManagedConfigs(res)

	out := make([]common.ConfigFile, 0, len(stored))
	for _, mc := range stored {
		out = append(out, common.ConfigFile{
			Name:     mc.Name,
			Location: mc.Location,
			Content:  mc.Content,
			Version:  mc.Version,
			Managed:  true,
			Hash:     util.MD5digest(mc.Content),
			Size:     int64(len(mc.Content)),
			Updated:  time.Now().UnixMilli(),
		})
	}
	return out
}

// decodeManagedConfigs decodes search hits via the shared elastic mapper.
func decodeManagedConfigs(res *orm.SearchResult) ([]ManagedConfig, int64, error) {
	return elastic.DecodeHits[ManagedConfig](res)
}

// diffConfigs builds the protocol response: created (server-only),
// updated (version newer than the client's), deleted (client-only).
// Configs the client marked Managed=false are never touched.
func diffConfigs(assigned []common.ConfigFile, clientConfigs map[string]common.ConfigFile) common.ConfigSyncResponse {
	resp := common.ConfigSyncResponse{}
	resp.Configs.CreatedConfigs = map[string]common.ConfigFile{}
	resp.Configs.UpdatedConfigs = map[string]common.ConfigFile{}
	resp.Configs.DeletedConfigs = map[string]common.ConfigFile{}

	serverMap := map[string]common.ConfigFile{}
	for _, c := range assigned {
		serverMap[c.Name] = c
	}

	for name, sc := range serverMap {
		cc, ok := clientConfigs[name]
		if !ok {
			resp.Configs.CreatedConfigs[name] = sc
			continue
		}
		if !cc.Managed {
			continue // client opted this file out of management
		}
		if sc.Version > cc.Version {
			resp.Configs.UpdatedConfigs[name] = sc
		}
	}
	for name, cc := range clientConfigs {
		if _, ok := serverMap[name]; !ok {
			if !cc.Managed {
				continue
			}
			resp.Configs.DeletedConfigs[name] = cc
		}
	}

	resp.Changed = len(resp.Configs.CreatedConfigs) > 0 ||
		len(resp.Configs.UpdatedConfigs) > 0 ||
		len(resp.Configs.DeletedConfigs) > 0
	return resp
}

// ConfigsHash mirrors the framework client's hash: MD5 of the JSON of the
// assigned config list, so both sides compare identical digests.
func ConfigsHash(files []common.ConfigFile) string {
	if len(files) == 0 {
		return ""
	}
	b, err := json.Marshal(files)
	if err != nil {
		return ""
	}
	return util.MD5digest(string(b))
}

func readBody(req *http.Request) ([]byte, error) {
	defer func() { _ = req.Body.Close() }()
	return io.ReadAll(req.Body)
}

// isNotFound reports whether err is a backend not-found marker (sqlite's
// ErrNotFound / elastic's ErrNotFound), which upsert treats as "create".
func isNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not found")
}

// matchesRegisteredAccessToken constant-time compares the presented token
// with the instance's registered self API token (Instance.AccessToken).
func matchesRegisteredAccessToken(ctx *orm.Context, instanceID, presented string) bool {
	if presented == "" {
		return false
	}
	inst := model.Instance{}
	inst.ID = instanceID
	exists, err := orm.GetV2(ctx, &inst)
	if err != nil || !exists || inst.AccessToken == nil {
		return false
	}
	want := strings.TrimSpace(inst.AccessToken.Value)
	if want == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(presented), []byte(want)) == 1
}

// loadInstanceStatus returns the admission status of an instance ("" when
// unknown).
func loadInstanceStatus(ctx *orm.Context, instanceID string) string {
	inst := model.Instance{}
	inst.ID = instanceID
	exists, err := orm.GetV2(ctx, &inst)
	if err != nil || !exists {
		return ""
	}
	return inst.Status
}

// approveInstanceHandler — POST /instance/:id/_approve
//
// Management action: flip a pending instance to approved and mint its
// per-instance token. The instance receives the token on its next
// register (the framework client re-registers while unapproved) or
// exchange call.
func (h *APIHandler) approveInstanceHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	ctx := orm.NewContextWithParent(req.Context()).DirectAccess()

	inst := model.Instance{}
	inst.ID = id
	exists, err := orm.GetV2(ctx, &inst)
	if err != nil && !isNotFound(err) {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err != nil || !exists {
		h.WriteOpRecordNotFoundJSON(w, id)
		return
	}

	if inst.Status != StatusApproved {
		inst.Status = StatusApproved
		if err := orm.Save(ctx, &inst); err != nil {
			h.WriteError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Infof("configs server: instance %s (%s) approved", id, inst.Name)
	}

	token, err := MintInstanceToken(ctx, id)
	if err != nil {
		h.WriteError(w, "mint instance token: "+err.Error(), http.StatusInternalServerError)
		return
	}
	h.WriteJSON(w, util.MapStr{"id": id, "status": StatusApproved, "manager_token": token}, http.StatusOK)
}
