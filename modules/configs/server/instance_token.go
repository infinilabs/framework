/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package server

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"time"

	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
)

// ──────────────────────────────────────────────────────────────────────────
// Per-instance manager tokens (console_design: managed-config-security.md §4).
//
// Lifecycle (portable adaptation of Console's managed token flow — the
// final-state design from pr/framework-managed-token-flow-20260524 as
// evolved on console_framework, simplified for the embedded server):
//
//   1. BOOTSTRAP: a fresh instance registers using one of the statically
//      configured tokens (configs.server.auth.tokens). The server mints an
//      instance-scoped token and returns it in the register response.
//   2. STEADY STATE: the instance presents its per-instance token
//      (Authorization: Bearer) on every /configs/_sync.
//   3. ROTATION: POST /instance/_exchange_token with the current token
//      mints a replacement; the previous token stays valid for 1h
//      (grace window for in-flight syncs).
//
// Storage: only the SHA-256 hash of each token is persisted (tokens are
// bearer-equivalent secrets; hashing at rest means a database leak does not
// leak credentials). Comparison is constant-time.
// ──────────────────────────────────────────────────────────────────────────

// rotationGrace is how long a superseded token remains valid after exchange.
const rotationGrace = time.Hour

// InstanceToken is the per-instance manager credential record.
type InstanceToken struct {
	orm.ORMObjectBase

	InstanceID string `json:"instance_id" elastic_mapping:"instance_id:{type:keyword}"`
	// TokenHash is sha256(token) of the current token.
	TokenHash string `json:"token_hash" elastic_mapping:"token_hash:{type:keyword}"`
	// PreviousHash is sha256(token) of the superseded token (rotation grace).
	PreviousHash string `json:"previous_hash,omitempty" elastic_mapping:"previous_hash:{type:keyword}"`
	// RotatedAt is when the current token was minted (grace window anchor).
	RotatedAt time.Time `json:"rotated_at" elastic_mapping:"rotated_at:{type:date}"`
}

// MintInstanceToken creates (or rotates) the token record for an instance
// and returns the plaintext token — the ONLY time it exists in the clear
// outside the client's memory.
func MintInstanceToken(ctx *orm.Context, instanceID string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)

	rec := loadInstanceToken(ctx, instanceID)
	if rec == nil {
		rec = &InstanceToken{InstanceID: instanceID}
		rec.ID = util.GetUUID()
	} else {
		rec.PreviousHash = rec.TokenHash
	}
	rec.TokenHash = hashToken(token)
	rec.RotatedAt = time.Now().UTC()
	orm.WithModel(ctx, rec)
	if err := orm.Save(ctx, rec); err != nil {
		return "", err
	}
	return token, nil
}

// ValidateInstanceToken checks a presented token against the instance's
// current record: current token always passes; the previous token passes
// only inside the rotation grace window. Constant-time per comparison.
func ValidateInstanceToken(ctx *orm.Context, instanceID, token string) bool {
	if token == "" {
		return false
	}
	rec := loadInstanceToken(ctx, instanceID)
	if rec == nil {
		return false
	}
	got := hashToken(token)
	match := subtle.ConstantTimeCompare([]byte(got), []byte(rec.TokenHash))
	if match == 1 {
		return true
	}
	if rec.PreviousHash != "" && time.Since(rec.RotatedAt) < rotationGrace {
		return subtle.ConstantTimeCompare([]byte(got), []byte(rec.PreviousHash)) == 1
	}
	return false
}

func loadInstanceToken(ctx *orm.Context, instanceID string) *InstanceToken {
	orm.WithModel(ctx, &InstanceToken{})
	qb := orm.NewQuery().
		Filter(orm.TermQuery("instance_id", instanceID)).
		Size(1)
	res, err := orm.SearchV2(ctx, qb)
	if err != nil || res == nil {
		return nil
	}
	tokens, _, _ := elastic.DecodeHits[InstanceToken](res)
	if len(tokens) == 0 {
		return nil
	}
	return &tokens[0]
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// ──────────────────────────────────────────────────────────────────────────
// HTTP: token exchange
// ──────────────────────────────────────────────────────────────────────────

// exchangeTokenHandler — POST /instance/_exchange_token
//
// Body: {"instance_id": "..."} authenticated by the CURRENT token (Bearer).
// Response: {"manager_token": "<new>", "grace_seconds": 3600}
func (h *APIHandler) exchangeTokenHandler(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var body struct {
		InstanceID string `json:"instance_id"`
	}
	if err := h.DecodeJSON(req, &body); err != nil || body.InstanceID == "" {
		h.WriteError(w, "instance_id is required", http.StatusBadRequest)
		return
	}

	presented := extractBearerToken(req)
	if presented == "" {
		w.Header().Set("WWW-Authenticate", `Bearer realm="configs"`)
		h.WriteError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := orm.NewContextWithParent(req.Context()).DirectAccess()

	// Accepted credentials: the current standard manager token, the
	// instance's registered self API token (the exchange's whole purpose:
	// self token → manager token), or a static admin token.
	ok := matchesManagerToken(ctx, body.InstanceID, presented)
	if !ok {
		ok = matchesRegisteredAccessToken(ctx, body.InstanceID, presented)
	}
	if !ok {
		ok = validateStaticToken(presented)
	}
	if !ok {
		w.Header().Set("WWW-Authenticate", `Bearer realm="configs"`)
		h.WriteError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	token, err := MintInstanceToken(ctx, body.InstanceID)
	if err != nil {
		h.WriteError(w, "mint token: "+err.Error(), http.StatusInternalServerError)
		return
	}
	h.WriteJSON(w, util.MapStr{
		"manager_token": token,
		"grace_seconds": int(rotationGrace.Seconds()),
	}, http.StatusOK)
}
