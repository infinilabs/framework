/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package server

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"

	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
)

// ──────────────────────────────────────────────────────────────────────────
// Enrollment tokens — one-time (or limited-use) admission tickets.
//
// The register endpoint is reachable by design (new agents have no
// credentials yet), which makes it a spam/forgery surface. An enrollment
// token is the out-of-band distributed ticket that redeems registration:
//
//   admin generates (UI/API, TTL + max uses) → embeds in the agent's
//   config (configs.enrollment_token) → agent registers presenting it →
//   server validates (hash, expiry, remaining uses) and consumes it.
//
// Forged/flooded registrations without a valid ticket are rejected with
// 403 before any record is written. Combined with the admission flow
// (pending → approve), this closes the register surface completely:
// no ticket → no pending record; no approval → no credentials/configs.
//
// Storage: sha256-at-rest like instance tokens; the plaintext is shown
// exactly once, at generation time.
// ──────────────────────────────────────────────────────────────────────────

// EnrollmentTokenKeystorePrefix distinguishes enrollment tokens in storage.
const enrollmentTokenModel = "enrollment-tokens"

// EnrollmentToken is a limited-use registration ticket.
type EnrollmentToken struct {
	orm.ORMObjectBase

	Name string `json:"name,omitempty"`

	// TokenHash is sha256(plaintext); the plaintext exists only in the
	// generation response.
	TokenHash string `json:"token_hash" elastic_mapping:"token_hash:{type:keyword}"`

	// MaxUses caps redemptions (1 = one-time). 0 treated as 1.
	MaxUses int `json:"max_uses" elastic_mapping:"max_uses:{type:integer}"`

	// UsedCounts is the redemption counter.
	UsedCount int `json:"used_count" elastic_mapping:"used_count:{type:integer}"`

	// ExpiresAt: zero value = no expiry (still bounded by MaxUses).
	ExpiresAt time.Time `json:"expires_at,omitempty" elastic_mapping:"expires_at:{type:date}"`

	// Revoked soft-deletes the token.
	Revoked bool `json:"revoked,omitempty" elastic_mapping:"revoked:{type:boolean}"`

	CreatedBy string    `json:"created_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

const enrollmentTokenPrefix = "et-"

// mintEnrollmentToken creates a token with the given policy and returns
// (record, plaintext). The plaintext is shown to the operator exactly once.
func mintEnrollmentToken(name string, maxUses int, ttl time.Duration, createdBy string) (*EnrollmentToken, string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return nil, "", err
	}
	plaintext := enrollmentTokenPrefix + hex.EncodeToString(raw)

	if maxUses < 1 {
		maxUses = 1
	}
	rec := &EnrollmentToken{
		Name:      name,
		TokenHash: hashToken(plaintext),
		MaxUses:   maxUses,
		ExpiresAt: time.Now().UTC().Add(ttl),
		CreatedBy: createdBy,
		CreatedAt: time.Now().UTC(),
	}
	rec.ID = util.GetUUID()
	return rec, plaintext, nil
}

// redeemEnrollmentToken validates and consumes a presented enrollment
// token. Returns nil when the ticket is invalid (unknown, revoked,
// expired, or exhausted) — the caller rejects the registration.
func redeemEnrollmentToken(ctx *orm.Context, plaintext string) *EnrollmentToken {
	if plaintext == "" || !strings.HasPrefix(plaintext, enrollmentTokenPrefix) {
		return nil
	}
	orm.WithModel(ctx, &EnrollmentToken{})
	qb := orm.NewQuery().
		Filter(orm.TermQuery("token_hash", hashToken(plaintext))).
		Size(1)
	res, err := orm.SearchV2(ctx, qb)
	if err != nil || res == nil {
		return nil
	}
	tokens, _, _ := decodeEnrollmentHits(res)
	if len(tokens) == 0 {
		return nil
	}
	t := &tokens[0]
	if t.Revoked || t.UsedCount >= t.MaxUses {
		return nil
	}
	if !t.ExpiresAt.IsZero() && time.Now().UTC().After(t.ExpiresAt) {
		return nil
	}
	t.UsedCount++
	if err := orm.Save(ctx, t); err != nil {
		return nil
	}
	return t
}

func decodeEnrollmentHits(res *orm.SearchResult) ([]EnrollmentToken, int64, error) {
	return elastic.DecodeHits[EnrollmentToken](res)
}

// rateLimiter is a small fixed-window per-key limiter for the register
// endpoint (flood control on the publicly reachable surface).
type rateLimiter struct {
	mu      sync.Mutex
	window  time.Duration
	maxHits int
	hits    map[string]*hitBucket
}

type hitBucket struct {
	count int
	since time.Time
}

func newRateLimiter(window time.Duration, maxHits int) *rateLimiter {
	return &rateLimiter{window: window, maxHits: maxHits, hits: map[string]*hitBucket{}}
}

func (r *rateLimiter) allow(key string) bool {
	if r == nil || r.maxHits <= 0 {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	b, ok := r.hits[key]
	if !ok || now.Sub(b.since) >= r.window {
		// lazy reset; also prune stale entries opportunistically
		if len(r.hits) > 4096 {
			r.hits = map[string]*hitBucket{}
		}
		r.hits[key] = &hitBucket{count: 1, since: now}
		return true
	}
	b.count++
	return b.count <= r.maxHits
}

func clientIP(req *http.Request) string {
	// best-effort: proxy chains first, then remote addr
	if fwd := req.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	if real := req.Header.Get("X-Real-IP"); real != "" {
		return real
	}
	addr := req.RemoteAddr
	if i := strings.LastIndex(addr, ":"); i > 0 {
		addr = addr[:i]
	}
	return addr
}

// ──────────────────────────────────────────────────────────────────────────
// HTTP: management endpoints for enrollment tokens (admin-gated by the
// static token gate, same as _approve).
// ──────────────────────────────────────────────────────────────────────────

// enrollmentTokensHandler — GET /instance/_enrollment_tokens (list, masked)
func (h *APIHandler) enrollmentTokensHandler(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	ctx := orm.NewContextWithParent(req.Context()).DirectAccess()
	orm.WithModel(ctx, &EnrollmentToken{})
	qb := orm.NewQuery().
		SortBy(orm.Sort{Field: "created_at", SortType: orm.DESC}).
		Size(100)
	res, err := orm.SearchV2(ctx, qb)
	if err != nil || res == nil {
		h.WriteJSON(w, util.MapStr{"tokens": []interface{}{}}, http.StatusOK)
		return
	}
	tokens, _, _ := decodeEnrollmentHits(res)
	out := make([]util.MapStr, 0, len(tokens))
	for _, t := range tokens {
		status := "valid"
		if t.Revoked {
			status = "revoked"
		} else if t.UsedCount >= t.MaxUses {
			status = "exhausted"
		} else if !t.ExpiresAt.IsZero() && time.Now().UTC().After(t.ExpiresAt) {
			status = "expired"
		}
		out = append(out, util.MapStr{
			"id":         t.ID,
			"name":       t.Name,
			"max_uses":   t.MaxUses,
			"used_count": t.UsedCount,
			"expires_at": t.ExpiresAt,
			"status":     status,
			"created_by": t.CreatedBy,
			"created_at": t.CreatedAt,
		})
	}
	h.WriteJSON(w, util.MapStr{"tokens": out}, http.StatusOK)
}

// createEnrollmentTokenHandler — POST /instance/_enrollment_tokens
//
// Body: {"name": "web-tier rollout", "max_uses": 50, "ttl_hours": 24}
// Response carries the plaintext exactly once.
func (h *APIHandler) createEnrollmentTokenHandler(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var body struct {
		Name     string `json:"name"`
		MaxUses  int    `json:"max_uses"`
		TTLHours int    `json:"ttl_hours"`
	}
	if err := h.DecodeJSON(req, &body); err != nil {
		h.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if body.MaxUses <= 0 {
		body.MaxUses = 1
	}
	if body.MaxUses > 10000 {
		h.WriteError(w, "max_uses too large (max 10000)", http.StatusBadRequest)
		return
	}
	ttl := time.Duration(body.TTLHours) * time.Hour
	if body.TTLHours <= 0 {
		ttl = 24 * time.Hour
	}

	rec, plaintext, err := mintEnrollmentToken(body.Name, body.MaxUses, ttl, req.Header.Get("X-API-USER"))
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ctx := orm.NewContextWithParent(req.Context()).DirectAccess()
	if err := orm.Save(ctx, rec); err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.WriteJSON(w, util.MapStr{
		"id":         rec.ID,
		"name":       rec.Name,
		"token":      plaintext, // shown exactly once
		"max_uses":   rec.MaxUses,
		"expires_at": rec.ExpiresAt,
	}, http.StatusOK)
}

// revokeEnrollmentTokenHandler — DELETE /instance/_enrollment_tokens/:id
func (h *APIHandler) revokeEnrollmentTokenHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	ctx := orm.NewContextWithParent(req.Context()).DirectAccess()
	rec := EnrollmentToken{}
	rec.ID = id
	exists, err := orm.GetV2(ctx, &rec)
	if err != nil || !exists {
		h.WriteOpRecordNotFoundJSON(w, id)
		return
	}
	rec.Revoked = true
	if err := orm.Save(ctx, &rec); err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.WriteJSON(w, util.MapStr{"id": id, "revoked": true}, http.StatusOK)
}
