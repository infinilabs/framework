/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infinilabs.com */

package pipeline

import (
	"fmt"
	"testing"

	"infini.sh/framework/core/model"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/configs/server"
)

// ── rolloutPlan (the pure gate) ──────────────────────────────────────────

func TestRolloutPlanCanaryThenBatches(t *testing.T) {
	spec := RolloutSpec{Canary: 1, Batch: 2}
	targets := []string{"i1", "i2", "i3", "i4", "i5"}
	digest := "d-new"

	// 1. first pass: canary only, rollback content captured
	allowed, st, changed := rolloutPlan(spec, RolloutState{}, targets, digest, "old-content", nil, false)
	if !changed || !st.Active || len(st.Rolled) != 1 || !allowed["i1"] || allowed["i2"] {
		t.Fatalf("canary wave wrong: allowed=%v state=%+v changed=%v", allowed, st, changed)
	}
	if st.RollbackContent != "old-content" {
		t.Fatalf("rollback target must capture prev content, got %q", st.RollbackContent)
	}

	// 2. canary not ready → hold exactly where we are
	ready := func(id string) bool { return false }
	allowed, st2, changed := rolloutPlan(spec, st, targets, digest, "", ready, false)
	if changed || !st2.Active || len(st2.Rolled) != 1 || len(allowed) != 1 {
		t.Fatalf("unready wave must hold: allowed=%v state=%+v changed=%v", allowed, st2, changed)
	}

	// 3. canary ready → one batch of 2 more
	ready = func(id string) bool { return true }
	_, st3, changed := rolloutPlan(spec, st, targets, digest, "", ready, false)
	if !changed || len(st3.Rolled) != 3 || !st3.Active {
		t.Fatalf("ready wave must advance by batch: %+v changed=%v", st3, changed)
	}

	// 4. last batch covers the rest → rollout completes
	_, st4, _ := rolloutPlan(spec, st3, targets, digest, "", ready, false)
	if st4.Active || len(st4.Rolled) != 5 {
		t.Fatalf("final wave must complete the rollout: %+v", st4)
	}
	if st4.RollbackContent != "old-content" {
		t.Fatalf("completed rollout must keep the rollback target, got %q", st4.RollbackContent)
	}
}

func TestRolloutPlanForceAdvancesWithoutReadiness(t *testing.T) {
	spec := RolloutSpec{Canary: 1, Batch: 1}
	targets := []string{"a", "b"}
	_, st, _ := rolloutPlan(spec, RolloutState{}, targets, "d", "", nil, false)
	if len(st.Rolled) != 1 {
		t.Fatalf("canary first: %+v", st)
	}
	// readiness says no, force says go — manual advance wins
	_, forced, _ := rolloutPlan(spec, st, targets, "d", "", func(string) bool { return false }, true)
	if len(forced.Rolled) != 2 || forced.Active {
		t.Fatalf("forced advance must finish the rollout: %+v", forced)
	}
}

func TestRolloutPlanUnreportedCanaryBlocks(t *testing.T) {
	spec := RolloutSpec{Canary: 1, Batch: 1}
	targets := []string{"a", "b"}
	_, st, _ := rolloutPlan(spec, RolloutState{}, targets, "d", "", nil, false)
	// readiness callback returning false = canary never confirmed
	_, held, changed := rolloutPlan(spec, st, targets, "d", "", func(string) bool { return false }, false)
	if changed || len(held.Rolled) != 1 {
		t.Fatalf("a dark canary must hold the rollout: %+v changed=%v", held, changed)
	}
}

func TestRolloutPlanPrunesOutOfScope(t *testing.T) {
	spec := RolloutSpec{Canary: 2, Batch: 2}
	st := RolloutState{Active: true, Digest: "d", Rolled: []string{"gone", "keep"}}
	allowed, next, _ := rolloutPlan(spec, st, []string{"keep", "other"}, "d", "", func(string) bool { return true }, false)
	if allowed["gone"] {
		t.Fatal("out-of-scope instance must not stay in the wave")
	}
	if len(next.Rolled) != 2 || !containsString(next.Rolled, "keep") || !containsString(next.Rolled, "other") {
		t.Fatalf("pruned wave must re-batch from the survivors: %+v", next)
	}
}

func TestRolloutPlanInactiveSpecOpenGate(t *testing.T) {
	allowed, _, changed := rolloutPlan(RolloutSpec{}, RolloutState{}, []string{"a", "b"}, "d", "", nil, false)
	if len(allowed) != 2 {
		t.Fatalf("no rollout spec = ungated: %v", allowed)
	}
	if changed {
		t.Fatal("ungated pass must not report a state change")
	}
}

// ── digest parity: control-plane render vs instance-side reconstruction ──

func TestDeployedContentDigestParity(t *testing.T) {
	cfg := &pipeline.PipelineConfigV2{}
	cfg.Name = "parity-check"
	cfg.Processors = []map[string]interface{}{
		{"test_rollout_echo": map[string]interface{}{"message": "hi"}},
		{"test_rollout_echo": map[string]interface{}{"message": "bye"}},
	}

	spec := renderSpec{enabled: true, singleton: false}
	rendered := renderDeployedPipeline(cfg, spec)
	controlSide := deployedContentDigestOf(rendered)

	// instance side: the config an agent actually loads from the rendered
	// file, re-rendered through the same canonical shape
	var wrapper struct {
		Pipeline []struct {
			Name        string                   `json:"name"`
			Enabled     *bool                    `json:"enabled"`
			AutoStart   bool                     `json:"auto_start"`
			KeepRunning bool                     `json:"keep_running"`
			Singleton   bool                     `json:"singleton"`
			Processors  []map[string]interface{} `json:"processor"`
		} `json:"pipeline"`
	}
	if err := util.FromJSONBytes(rendered, &wrapper); err != nil || len(wrapper.Pipeline) != 1 {
		t.Fatalf("rendered config must parse: %v", err)
	}
	loaded := pipeline.PipelineConfigV2{
		Name:        wrapper.Pipeline[0].Name,
		Enabled:     wrapper.Pipeline[0].Enabled,
		AutoStart:   wrapper.Pipeline[0].AutoStart,
		KeepRunning: wrapper.Pipeline[0].KeepRunning,
		Singleton:   wrapper.Pipeline[0].Singleton,
		Processors:  wrapper.Pipeline[0].Processors,
	}
	instanceSide := deployedContentDigest(deployedPipelineMap(&loaded, renderSpec{
		enabled:   isPipelineEnabled(loaded.Enabled),
		singleton: loaded.Singleton,
	}))

	if controlSide != instanceSide || controlSide == "" {
		t.Fatalf("digest mismatch: control=%s instance=%s", controlSide, instanceSide)
	}
}

// ── staged reconcile through the real ORM harness ────────────────────────

// reportInstance overwrites an instance's runtime report (what _sync
// would upsert).
func reportInstance(t *testing.T, id string, pipelines string) {
	t.Helper()
	octx := orm.NewContext().DirectAccess()
	orm.WithModel(octx, &model.Instance{})
	inst := model.Instance{}
	inst.ID = id
	found, err := orm.GetV2(octx, &inst)
	if err != nil || !found {
		t.Fatalf("instance %s missing: %v", id, err)
	}
	inst.RuntimePipelines = pipelines
	if err := orm.Save(octx, &inst); err != nil {
		t.Fatal(err)
	}
}

func loadDeployment(t *testing.T, id string) PipelineDeployment {
	t.Helper()
	octx := orm.NewContext().DirectAccess()
	orm.WithModel(octx, &PipelineDeployment{})
	dep := PipelineDeployment{}
	dep.ID = id
	found, err := orm.GetV2(octx, &dep)
	if err != nil || !found {
		t.Fatalf("deployment %s missing: %v", id, err)
	}
	return dep
}

func rowInstanceIDs(rows map[string]server.ManagedConfig) []string {
	ids := make([]string, 0, len(rows))
	for id := range rows {
		ids = append(ids, id)
	}
	return ids
}

func TestStagedReconcileCanaryBatchComplete(t *testing.T) {
	env := newDeployTestEnv(t)
	dep := &PipelineDeployment{
		PipelineID: env.pid,
		Enabled:    true,
		Placement: PlacementSpec{
			Strategy:  "all",
			Instances: []string{env.instA, env.instB, env.instC},
		},
	}
	dep.ID = "d1" // shared upsert id: exactly ONE deployment row exists per test run
	dep.Name = "rollout-test"
	env.saveDeployment(dep)

	// phase 0: converge WITHOUT a rollout spec — every target on content v1
	env.reconcile()
	rows := env.rows()
	if len(rows) != 3 {
		t.Fatalf("ungated reconcile must converge all 3 targets, got %d", len(rows))
	}
	var v1Content string
	for _, row := range rows {
		v1Content = row.Content
		break
	}

	// change the pipeline (content v2) AND arm staged delivery
	octx := orm.NewContext().DirectAccess()
	orm.WithModel(octx, &pipeline.PipelineConfigV2{})
	cfg := pipeline.PipelineConfigV2{}
	cfg.ID = env.pid
	if found, err := orm.GetV2(octx, &cfg); err != nil || !found {
		t.Fatalf("pipeline missing: %v", err)
	}
	cfg.Processors = append(cfg.Processors, map[string]interface{}{
		"validate_test_echo": map[string]interface{}{"message": "v2-marker"},
	})
	if err := orm.Save(octx, &cfg); err != nil {
		t.Fatal(err)
	}
	armed := loadDeployment(t, dep.ID)
	armed.Placement.Rollout = RolloutSpec{Canary: 1, Batch: 1}
	env.saveDeployment(&armed)

	// pass 1: canary only — exactly one row on the new content
	env.reconcile()
	rows = env.rows()
	if len(rows) != 3 {
		t.Fatalf("staging never drops converged rows, got %d", len(rows))
	}
	var canaryID string
	onV2 := 0
	for id, row := range rows {
		if row.Content != v1Content {
			canaryID = id
			onV2++
		}
	}
	if onV2 != 1 {
		t.Fatalf("canary pass must update exactly 1 row to v2, got %d", onV2)
	}
	digest := deployedContentDigestOf([]byte(rows[canaryID].Content))

	// pass 2: canary has not reported → nothing else moves
	env.reconcile()
	onV2 = 0
	for _, row := range env.rows() {
		if row.Content != v1Content {
			onV2++
		}
	}
	if onV2 != 1 {
		t.Fatalf("unreported canary must hold the rollout, onV2=%d", onV2)
	}

	// canary reports running the pushed content → next batch opens
	reportInstance(t, canaryID, fmt.Sprintf(`{%q:{"state":"STARTED","config_digest":%q}}`, env.pipeName, digest))
	env.reconcile()
	onV2 = 0
	for _, row := range env.rows() {
		if row.Content != v1Content {
			onV2++
		}
	}
	if onV2 != 2 {
		t.Fatalf("ready canary must open the next batch, onV2=%d", onV2)
	}

	// every v2 target reports → completion covers the fleet
	for id, row := range env.rows() {
		if row.Content != v1Content {
			reportInstance(t, id, fmt.Sprintf(`{%q:{"state":"STARTED","config_digest":%q}}`, env.pipeName, digest))
		}
	}
	env.reconcile()
	onV2 = 0
	for _, row := range env.rows() {
		if row.Content != v1Content {
			onV2++
		}
	}
	if onV2 != 3 {
		t.Fatalf("completed rollout must cover all targets, onV2=%d", onV2)
	}

	// terminal state persisted: inactive, v1 recorded as the rollback target
	st := loadDeployment(t, dep.ID)
	if st.State.Active {
		t.Fatalf("rollout must be terminal: %+v", st.State)
	}
	if st.State.RollbackContent != v1Content {
		t.Fatalf("rollback target must be the pre-rollout content (v1), got %d bytes", len(st.State.RollbackContent))
	}
}

func TestStagedReconcileOldDigestDoesNotAdvance(t *testing.T) {
	env := newDeployTestEnv(t)
	dep := &PipelineDeployment{
		PipelineID: env.pid,
		Enabled:    true,
		Placement: PlacementSpec{
			Strategy:  "all",
			Instances: []string{env.instA, env.instB},
			Rollout:   RolloutSpec{Canary: 1, Batch: 1},
		},
	}
	dep.ID = "d1" // shared upsert id (see rollout test above)
	dep.Name = "rollout-stale-digest"
	env.saveDeployment(dep)

	env.reconcile()
	rows := env.rows()
	if len(rows) != 1 {
		t.Fatalf("canary pass must write exactly 1 row, got %d", len(rows))
	}
	var canaryID string
	for id := range rows {
		canaryID = id
	}
	// canary reports STARTED but with the OLD content's digest — the very
	// lie the digest check exists to catch
	reportInstance(t, canaryID, fmt.Sprintf(`{%q:{"state":"STARTED","config_digest":"stale"}}`, env.pipeName))
	env.reconcile()
	if got := len(env.rows()); got != 1 {
		t.Fatalf("a STARTED-but-stale canary must hold the rollout, rows=%d", got)
	}
}
