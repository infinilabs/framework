/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infinilabs.com */

package pipeline

import (
	"fmt"
	"net/http"
	"sort"

	log "github.com/cihub/seelog"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
)

// Staged rollout (#125, SPEC §8.2): a deployment with a rollout spec does
// not push new content to every target at once — the reconciler writes the
// canary wave first, waits until those instances REPORT running the staged
// content (digest match, see DeployedContentDigest), then advances in
// batches. The _rollout endpoint overrides the machine: force the next
// batch now, or roll every target back to the last converged content.

// RolloutState is the persisted progress of a staged delivery. Lives on
// the deployment; the reconciler advances it, the endpoints override it.
type RolloutState struct {
	// Active marks a rollout in progress (content staged, waves pending).
	Active bool `json:"active,omitempty" elastic_mapping:"-"`
	// Digest is the content digest being staged (what the waves converge to).
	Digest string `json:"digest,omitempty" elastic_mapping:"digest:{type:keyword}"`
	// Rolled lists the target instance IDs already on the staged content.
	Rolled []string `json:"rolled,omitempty" elastic_mapping:"rolled:{type:keyword}"`
	// RollbackContent is the last converged rendered config (the fallback
	// the rollback action rewrites every target to). Digest-only would be
	// unactionable — the bytes are what gets written back.
	RollbackContent string `json:"rollback_content,omitempty" elastic_mapping:"-"`
}

// rolloutSpecActive reports whether the spec asks for staged delivery at
// all (canary or batch > 0).
func rolloutSpecActive(spec RolloutSpec) bool {
	return spec.Canary > 0 || spec.Batch > 0
}

// rolloutGateOwner picks the deployment whose rollout spec stages this
// pipeline's content: exactly one rollout bearer AND one uniform render
// spec across the merged targets. Two bearers, or a merge that produces
// diverging per-instance renders, have no single digest to stage — they
// converge ungated (logged once per pass by the caller's Debugf absence:
// the fleet behaves exactly as before the rollout feature existed).
func rolloutGateOwner(deps []*PipelineDeployment, desired map[string]renderSpec) *PipelineDeployment {
	var owner *PipelineDeployment
	for _, dep := range deps {
		if rolloutSpecActive(dep.Placement.Rollout) {
			if owner != nil {
				return nil
			}
			owner = dep
		}
	}
	if owner == nil {
		return nil
	}
	var first renderSpec
	for _, spec := range desired {
		first = spec
		break
	}
	for _, spec := range desired {
		if spec != first {
			return nil
		}
	}
	return owner
}

// rolloutWaveSizes returns (first, step): the canary wave size and the
// follow-up batch size. Only canary set → step = canary (keep the same
// chunk); only batch set → the first wave IS a batch.
func rolloutWaveSizes(spec RolloutSpec) (int, int) {
	first, step := spec.Canary, spec.Batch
	if first <= 0 {
		first = spec.Batch
	}
	if step <= 0 {
		step = first
	}
	return first, step
}

// rolloutPlan is the pure gate the reconciler consults: given the rollout
// spec, the persisted state, the (deterministically ordered) target IDs,
// the digest of the content being distributed and a readiness callback,
// it returns which targets may receive the content NOW plus the next
// persisted state.
//
//   - spec inactive            → gate open, state untouched
//   - content changed          → new rollout: canary wave only, the
//                                 previous converged content becomes the
//                                 rollback target
//   - same content, active     → advance one batch when EVERY rolled
//                                 target is ready; hold otherwise
//   - waves cover all targets  → rollout completes (Active=false)
//
// ready is only consulted for instances in the current wave. force skips
// the readiness check (the manual advance action).
func rolloutPlan(spec RolloutSpec, st RolloutState, targets []string, digest, prevContent string, ready func(id string) bool, force bool) (allowed map[string]bool, next RolloutState, changed bool) {
	allowed = map[string]bool{}
	next = st

	if !rolloutSpecActive(spec) {
		for _, id := range targets {
			allowed[id] = true
		}
		return allowed, next, false
	}

	sorted := append([]string(nil), targets...)
	sort.Strings(sorted)

	// drop rolled IDs that fell out of scope (deployment re-targeted)
	rolled := make([]string, 0, len(st.Rolled))
	inTargets := map[string]bool{}
	for _, id := range sorted {
		inTargets[id] = true
	}
	for _, id := range st.Rolled {
		if inTargets[id] {
			rolled = append(rolled, id)
		}
	}
	pruned := len(rolled) != len(st.Rolled)

	if !st.Active || st.Digest != digest {
		// new staged rollout (or first ever): canary wave
		first, _ := rolloutWaveSizes(spec)
		if first > len(sorted) {
			first = len(sorted)
		}
		rolled = append([]string(nil), sorted[:first]...)
		next = RolloutState{
			Active:          true,
			Digest:          digest,
			Rolled:          rolled,
			RollbackContent: st.RollbackContent,
		}
		// the previous converged content is only a meaningful fallback
		// when it differs from what we are staging now
		if prevContent != "" {
			next.RollbackContent = prevContent
		}
		changed = true
	} else if force || rolloutWaveReady(rolled, ready) {
		_, step := rolloutWaveSizes(spec)
		nextRolled := append([]string(nil), rolled...)
		for _, id := range sorted {
			if len(nextRolled)-len(rolled) >= step {
				break
			}
			if !containsString(nextRolled, id) {
				nextRolled = append(nextRolled, id)
			}
		}
		if len(nextRolled) != len(rolled) {
			rolled = nextRolled
			changed = true
		}
	}

	// completion: every target carries the staged content
	if len(rolled) >= len(sorted) {
		next = RolloutState{
			Active:          false,
			Digest:          digest,
			Rolled:          rolled,
			RollbackContent: next.RollbackContent,
		}
		if st.Active {
			changed = true
		}
	} else {
		next.Active = true
		next.Digest = digest
		next.Rolled = rolled
	}
	if pruned && !changed {
		changed = true
	}

	for _, id := range rolled {
		allowed[id] = true
	}
	return allowed, next, changed
}

// rolloutWaveReady reports whether every instance of the current wave
// confirms running the staged content. An instance that never reported
// blocks the wave — conservative on purpose: a canary that went dark
// must stop the rollout, not let it sweep the fleet (the manual advance
// endpoint is the operator's override).
func rolloutWaveReady(rolled []string, ready func(id string) bool) bool {
	for _, id := range rolled {
		if !ready(id) {
			return false
		}
	}
	return true
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// rolloutActionHandler — POST /pipeline/deployments/:id/_rollout
// Body: {"action":"advance"|"rollback"}.
//   advance  → force the next batch immediately (readiness bypass)
//   rollback → rewrite EVERY target to the last converged content at once
func (h *PipeModule) rolloutActionHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	var body struct {
		Action string `json:"action"`
	}
	if err := h.DecodeJSON(req, &body); err != nil {
		h.Error400(w, err.Error())
		return
	}

	h.rolloutMu.Lock()
	octx := orm.NewContextWithParent(req.Context()).DirectAccess()
	dep := PipelineDeployment{}
	dep.ID = id
	exists, err := orm.GetV2(octx, &dep)
	if err != nil || !exists {
		h.rolloutMu.Unlock()
		h.WriteOpRecordNotFoundJSON(w, id)
		return
	}

	switch body.Action {
	case "advance":
		if !dep.State.Active {
			h.rolloutMu.Unlock()
			h.WriteError(w, "no rollout in progress (state inactive)", http.StatusConflict)
			return
		}
		// force-advance one batch: readiness always true
		_, planned, _ := rolloutPlan(dep.Placement.Rollout, dep.State,
			deploymentTargetIDs(&dep, h.loadRegisteredInstances(octx)),
			dep.State.Digest, "", func(string) bool { return true }, true)
		dep.State = planned
		if err := orm.Save(octx, &dep); err != nil {
			h.rolloutMu.Unlock()
			h.WriteError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		h.rolloutMu.Unlock()
		h.reconcileOnce()
		h.WriteJSON(w, util.MapStr{"result": "advanced", "rollout": dep.State}, http.StatusOK)

	case "rollback":
		if dep.State.RollbackContent == "" {
			h.rolloutMu.Unlock()
			h.WriteError(w, "no previous converged content recorded for this deployment", http.StatusConflict)
			return
		}
		if err := h.rollbackDeploymentContent(octx, &dep); err != nil {
			h.rolloutMu.Unlock()
			h.WriteError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// The rollback is a BRAKE, not a desired-state change: the pipeline
		// object still carries the new content, so the next reconciler pass
		// starts a fresh CANARY-staged rollout toward it — visible, gradual,
		// stoppable. Making a rollback permanent means rolling the pipeline
		// version back too (POST /pipelines/:id/versions/:v/rollback).
		dep.State = RolloutState{
			Active:          false,
			Digest:          deployedContentDigestOf([]byte(dep.State.RollbackContent)),
			RollbackContent: "", // one level of fallback; deeper = pipeline version history
		}
		if err := orm.Save(octx, &dep); err != nil {
			h.rolloutMu.Unlock()
			h.WriteError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		h.rolloutMu.Unlock()
		log.Infof("deployment %s rolled back: every target rewritten to the last converged content", dep.Name)
		h.WriteJSON(w, util.MapStr{
			"result":  "rolled-back",
			"rollout": dep.State,
			"note":    "desired state still carries the new content — the next reconcile re-stages it from canary; roll the pipeline version back to make this permanent",
		}, http.StatusOK)

	default:
		h.rolloutMu.Unlock()
		h.Error400(w, fmt.Sprintf("unknown action %q (advance|rollback)", body.Action))
	}
}
