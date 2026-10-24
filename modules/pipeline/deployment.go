/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infinilabs.com */

package pipeline

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	log "github.com/cihub/seelog"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/model"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/util"
	configs_server "infini.sh/framework/modules/configs/server"
)

// PipelineDeployment is the DESIRED state: which pipeline runs where and
// how. The reconciler translates this into managed config rows that
// gateway/agent instances pull through the configs-center channel.
//
// This is the control-plane object — instances never see it; they only
// receive the translated pipeline configs.
type PipelineDeployment struct {
	orm.ORMObjectBase `json:",inline" elastic_mapping:"-"`

	Name       string `json:"name" elastic_mapping:"name:{type:keyword}"`
	PipelineID string `json:"pipeline_id" elastic_mapping:"pipeline_id:{type:keyword}"`

	Placement PlacementSpec `json:"placement" elastic_mapping:"placement:{type:object}"`

	// Enabled gates the reconciler: disabled = no configs pushed.
	Enabled bool `json:"enabled" elastic_mapping:"enabled:{type:boolean}"`

	// Version increments on every spec change (reconciler compares with
	// the last-pushed version to detect drift).
	Version int `json:"version" elastic_mapping:"version:{type:integer}"`

	// State is the reconciler-owned progress of a staged rollout (#125):
	// which content digest is being staged, who already carries it and
	// what a rollback would fall back to. Managed by rollout.go.
	State RolloutState `json:"rollout_state,omitempty" elastic_mapping:"rollout_state:{type:object}"`
}

// PlacementSpec declares WHERE and HOW the pipeline runs.
type PlacementSpec struct {
	// Selector picks the target instance set (label match).
	Selector map[string]string `json:"selector,omitempty" config:"selector"`

	// Instances lists explicit target instance IDs. When non-empty, the
	// reconciler creates per-instance managed configs; empty = broadcast.
	Instances []string `json:"instances,omitempty" config:"instances"`

	// Strategy: "singleton" (one active runner, lock-contested) |
	// "all" (every instance runs a copy)
	// "sharded" requires kafka queue backend (deferred, D-12)
	Strategy string `json:"strategy" config:"strategy"`

	// Rollout controls staged delivery.
	Rollout RolloutSpec `json:"rollout,omitempty" config:"rollout"`
}

// RolloutSpec gates how many instances get the new version at once.
type RolloutSpec struct {
	Canary         int `json:"canary,omitempty" config:"canary"`                   // first N instances
	Batch          int `json:"batch,omitempty" config:"batch"`                     // then batches of N
	MaxUnavailable int `json:"max_unavailable,omitempty" config:"max_unavailable"` // concurrent down cap
}

// DeploymentStatus is the REPORTED state per deployment (what instances
// actually acknowledged running).
type DeploymentStatus struct {
	DeploymentID string           `json:"deployment_id"`
	Version      int              `json:"version"`
	Instances    []InstanceReport `json:"instances"`
	DesiredCount int              `json:"desired_count"`
	ReadyCount   int              `json:"ready_count"`
	// Rollout mirrors the staged-delivery progress when one is (or was)
	// active — the rollout progress block of the cluster overview.
	Rollout *RolloutState `json:"rollout,omitempty"`
}

// runtimePipelineReport is one pipeline entry of an instance's
// RuntimePipelines JSON (what _sync upserts on every heartbeat).
type runtimePipelineReport struct {
	State        string `json:"state"`
	Reason       string `json:"reason"`
	ConfigDigest string `json:"config_digest"`
}

// runtimePipelineStates parses the instance's reported pipeline states;
// absent/invalid JSON yields an empty map (unreported, blocks readiness).
func runtimePipelineStates(raw string) map[string]runtimePipelineReport {
	out := map[string]runtimePipelineReport{}
	if raw == "" {
		return out
	}
	var parsed map[string]runtimePipelineReport
	if json.Unmarshal([]byte(raw), &parsed) != nil {
		return out
	}
	return parsed
}

type InstanceReport struct {
	InstanceID string    `json:"instance_id"`
	State      string    `json:"state"` // running | stopped | failed | unknown
	Version    int       `json:"version"`
	LastSeen   time.Time `json:"last_seen"`
	// Reason carries the instance-reported failure explanation (compile
	// error, missing processor on that instance)
	Reason string `json:"reason,omitempty"`
	// Stale marks an instance that still runs the pipeline although the
	// deployment scope no longer includes it (recall in flight/failed)
	Stale bool `json:"stale,omitempty"`
	// Cordoned mirrors the instance's maintenance flag (reported via
	// _sync, #128): running tasks drain, nothing new starts there
	Cordoned bool `json:"cordoned,omitempty"`
}

// ─── reconciler ──────────────────────────────────────────────────────────

// reconcilePoke requests an immediate reconciler pass outside the 15s tick.
// Buffered so pokers never block; a poke landing while a pass is already
// running just coalesces into one follow-up pass.
var reconcilePoke = make(chan struct{}, 1)

// RequestReconcile asks the deployment reconciler to run a pass as soon as
// possible (async, best-effort). Publishers call it after upserting
// PipelineDeployment/PipelineConfigV2 so convergence starts immediately
// instead of on the next tick (#128).
func RequestReconcile() {
	select {
	case reconcilePoke <- struct{}{}:
	default:
	}
}

// reconcileDeployments runs on a ticker: for each enabled deployment,
// compare desired state with the managed-config rows that exist, and
// create/update/delete configs to converge.
func (module *PipeModule) startDeploymentReconciler() {
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
			case <-reconcilePoke:
			}
			if module.closed.Load() {
				return
			}
			module.reconcileOnce()
		}
	}()
	log.Debugf("deployment reconciler started (interval 15s)")
}

func (module *PipeModule) reconcileOnce() {
	// reconcile is a control-plane operation — skip on instances without
	// an ORM backend (lightweight agents that only run pushed configs)
	if !orm.IsHandlerRegistered() {
		return
	}
	octx := orm.NewContext().DirectAccess()

	// registered instances — the universe that selector/explicit scopes
	// resolve against (labels refresh on every sync heartbeat)
	orm.WithModel(octx, &model.Instance{})
	instRes, ierr := orm.SearchV2(octx, orm.NewQuery().Size(500))
	if ierr != nil {
		return
	}
	instances, _, _ := elastic.DecodeHits[model.Instance](instRes)

	// ALL deployments, disabled included: a disabled deployment keeps
	// converging with enabled:false in the rendered config, so instances
	// STOP the task while KEEPING the config (instant re-enable). Skipping
	// disabled ones would leave the pipeline running on every instance.
	orm.WithModel(octx, &PipelineDeployment{})
	res, err := orm.SearchV2(octx, orm.NewQuery().Size(200))
	if err != nil {
		return
	}
	deployments, _, _ := elastic.DecodeHits[PipelineDeployment](res)

	// group by pipeline: managed-config rows are named per PIPELINE
	// (pipeline-deploy-<id>.yml), so every deployment of the same
	// pipeline must converge together — one adds instances, another's
	// scope-shrink must not delete rows the first one still needs
	byPipeline := map[string][]*PipelineDeployment{}
	for i := range deployments {
		byPipeline[deployments[i].PipelineID] = append(byPipeline[deployments[i].PipelineID], &deployments[i])
	}
	for pipelineID, deps := range byPipeline {
		if rerr := module.reconcilePipeline(octx, pipelineID, deps, instances); rerr != nil {
			log.Warnf("reconciler: %v", rerr)
		}
	}
}

// renderSpec is how ONE target instance should run the pipeline — the
// merge result of every deployment of that pipeline targeting it.
type renderSpec struct {
	enabled   bool
	singleton bool
}

// resolveTargets maps a deployment's placement onto registered instances:
// label selector matches AND explicit instance IDs union. Neither set =
// broadcast (every instance, the v1 semantics).
func resolveTargets(dep *PipelineDeployment, instances []model.Instance) (map[string]bool, bool) {
	if len(dep.Placement.Selector) == 0 && len(dep.Placement.Instances) == 0 {
		return nil, true
	}
	known := map[string]bool{}
	targets := map[string]bool{}
	for i := range instances {
		known[instances[i].ID] = true
		// an empty selector must not match everything — when only
		// explicit IDs are set, label matching is skipped entirely
		if len(dep.Placement.Selector) > 0 && instanceMatchesLabels(&instances[i], dep.Placement.Selector) {
			targets[instances[i].ID] = true
		}
	}
	// explicit IDs resolve against REGISTERED instances only — a row for
	// an unknown ID is invisible garbage no one pulls; when the instance
	// later registers, the next cycle picks it up (self-healing scope)
	for _, id := range dep.Placement.Instances {
		if known[id] {
			targets[id] = true
		}
	}
	return targets, false
}

func instanceMatchesLabels(inst *model.Instance, selector map[string]string) bool {
	for k, v := range selector {
		if inst.Labels[k] != v {
			return false
		}
	}
	return true
}

// reconcilePipeline converges ONE pipeline's deployments into the EXACT
// set of managed-config rows: one row per target instance (or a single
// broadcast "*" row). Rows for instances that fell out of every scope are
// deleted — the instance's next sync reports the file Deleted and the
// config watcher removes it and stops the task. Nothing more, nothing
// less: converging means the row set equals the desired set.
func (module *PipeModule) reconcilePipeline(octx *orm.Context, pipelineID string, deps []*PipelineDeployment, instances []model.Instance) error {
	cfg := pipeline.PipelineConfigV2{}
	cfg.ID = pipelineID
	exists, err := orm.GetV2(octx, &cfg)
	if err != nil || !exists {
		return fmt.Errorf("pipeline %s not found (deployments: %s)", pipelineID, deploymentNames(deps))
	}

	// defensive gate: never distribute a config that no longer compiles
	// (e.g. a processor plugin went missing after an upgrade)
	if errs, _ := ValidatePipelineConfig(&cfg); len(errs) > 0 {
		return fmt.Errorf("pipeline %s failed validation, refusing to distribute: %s",
			cfg.Name, errs[0].Message)
	}

	// merge the target set + render flags across all deployments of this
	// pipeline: an enabled deployment outranks a disabled one targeting
	// the same instance; singleton wins over all. Any broadcast
	// deployment subsumes targeting (a "*" row already reaches everyone).
	broadcast := false
	desired := map[string]renderSpec{}
	for _, dep := range deps {
		targets, bcast := resolveTargets(dep, instances)
		if bcast {
			broadcast = true
			continue
		}
		spec := renderSpec{enabled: dep.Enabled, singleton: dep.Placement.Strategy == "singleton"}
		for id := range targets {
			prev := desired[id]
			prev.enabled = prev.enabled || spec.enabled
			prev.singleton = prev.singleton || spec.singleton
			desired[id] = prev
		}
	}
	if broadcast {
		spec := renderSpec{}
		for _, dep := range deps {
			spec.enabled = spec.enabled || dep.Enabled
			spec.singleton = spec.singleton || dep.Placement.Strategy == "singleton"
		}
		desired = map[string]renderSpec{configs_server.AllInstancesID: spec}
	}

	configName := fmt.Sprintf("pipeline-deploy-%s.yml", pipelineID)

	// existing rows of this config name (deployment-owned convention)
	orm.WithModel(octx, &configs_server.ManagedConfig{})
	rowsRes, rerr := orm.SearchV2(octx, orm.NewQuery().
		Filter(orm.TermQuery("name", configName)).Size(500))
	if rerr != nil {
		return fmt.Errorf("query managed config %s: %w", configName, rerr)
	}
	existingRows, _, _ := elastic.DecodeHits[configs_server.ManagedConfig](rowsRes)
	existing := map[string]configs_server.ManagedConfig{}
	for _, row := range existingRows {
		existing[row.InstanceID] = row
	}

	// instance syncs apply updates strictly by version ("newest version
	// per name" wins on the server too) — every write must land ABOVE the
	// name's current max version or clients keep stale content forever
	// (e.g. migrating the legacy broadcast row onto per-instance rows)
	maxVersion := int64(0)
	for _, row := range existingRows {
		if row.Version > maxVersion {
			maxVersion = row.Version
		}
	}

	// staged rollout gate (#125): a rollout-bearing deployment stages the
	// content — only the current wave (canary first, then batches once
	// the wave reports running it, see rollout.go) may receive it.
	var gateAllowed map[string]bool
	if owner := rolloutGateOwner(deps, desired); owner != nil {
		var sampleSpec renderSpec
		for _, spec := range desired {
			sampleSpec = spec
			break
		}
		content := string(renderDeployedPipeline(&cfg, sampleSpec))
		digest := deployedContentDigest(deployedPipelineMap(&cfg, sampleSpec))
		// the fallback a rollback would restore: what rows that do not
		// carry the new content still hold ("" when none exist yet)
		prevContent := ""
		for _, row := range existingRows {
			if row.Content != content {
				prevContent = row.Content
				break
			}
		}
		byID := map[string]model.Instance{}
		for i := range instances {
			byID[instances[i].ID] = instances[i]
		}
		singleton := owner.Placement.Strategy == "singleton"
			ready := func(id string) bool {
				inst, ok := byID[id]
				if !ok {
					return false
				}
				// a frozen report must not count forever: an instance that
				// died keeps its last STARTED+digest — require a heartbeat
				// within 3 default sync intervals (30s each) or the wave
				// holds until a live holder proves the content
				if inst.Updated != nil && time.Since(*inst.Updated) > 90*time.Second {
					return false
				}
				st, reported := runtimePipelineStates(inst.RuntimePipelines)[cfg.Name]
			if !reported || st.State == "FAILED" {
				return false
			}
			if st.ConfigDigest != digest {
				return false // still running the old content
			}
			if singleton {
				return true // non-FAILED holders count: the lock picks the runner
			}
			return st.State == "STARTED"
		}
		targets := make([]string, 0, len(desired))
		for id := range desired {
			targets = append(targets, id)
		}
		module.rolloutMu.Lock()
		allowed, next, changed := rolloutPlan(owner.Placement.Rollout, owner.State, targets, digest, prevContent, ready, false)
		if changed {
			owner.State = next
			dctx := orm.NewContext().DirectAccess()
			orm.WithModel(dctx, &PipelineDeployment{})
			if serr := orm.Save(dctx, owner); serr != nil {
				log.Warnf("reconciler: persist rollout state of %s failed: %v (gate held this pass)", owner.Name, serr)
			} else {
				log.Debugf("reconciler: rollout of %s now at %d/%d targets (active=%v)", owner.Name, len(next.Rolled), len(targets), next.Active)
			}
		}
		module.rolloutMu.Unlock()
		gateAllowed = allowed
	}

	for instID, spec := range desired {
		if gateAllowed != nil && !gateAllowed[instID] {
			continue // staged rollout: this instance's wave has not opened yet
		}
		content := string(renderDeployedPipeline(&cfg, spec))
		row, ok := existing[instID]
		// converged: the pushed content already matches — skip the
		// version bump and re-push so instances don't rewrite files
		if ok && row.Content == content {
			continue
		}
		row.Name = configName
		row.InstanceID = instID
		row.Content = content
		row.Version = maxVersion + 1
		row.Readonly = true // deployment-owned
		if row.ID == "" {
			// broadcast keeps the legacy v1 ID so an existing row
			// updates in place instead of duplicating
			if instID == configs_server.AllInstancesID {
				row.ID = fmt.Sprintf("deploy-%s", pipelineID)
			} else {
				row.ID = fmt.Sprintf("deploy-%s-%s", pipelineID, instID)
			}
		}
		if serr := orm.Save(octx, &row); serr != nil {
			return fmt.Errorf("push managed config %s→%s: %w", configName, instID, serr)
		}
		maxVersion = row.Version
		log.Debugf("reconciler: %s → instance %s v%d (%d bytes, enabled=%v singleton=%v)",
			configName, instID, row.Version, len(content), spec.enabled, spec.singleton)
	}

	// garbage-collect rows that no deployment targets anymore — the
	// instance recalls the file (and stops the task) on its next sync
	for instID, row := range existing {
		if _, ok := desired[instID]; !ok {
			if derr := orm.Delete(octx, &row); derr != nil {
				return fmt.Errorf("recall managed config %s→%s: %w", configName, instID, derr)
			}
			log.Infof("reconciler: %s dropped instance %s from scope — recalling config", configName, instID)
		}
	}
	return nil
}

func deploymentNames(deps []*PipelineDeployment) string {
	names := ""
	for i, dep := range deps {
		if i > 0 {
			names += ", "
		}
		names += dep.Name
	}
	return names
}

// renderDeployedPipeline serializes the pipeline config handed to
// instances. A disabled spec renders enabled:false + auto_start:false —
// the instance's config watcher stops (and cleans) the running task on
// the enabled→disabled transition while keeping the file for instant
// re-enable.
func renderDeployedPipeline(cfg *pipeline.PipelineConfigV2, spec renderSpec) []byte {
	return util.MustToJSONBytes(map[string]interface{}{
		"pipeline": []interface{}{deployedPipelineMap(cfg, spec)},
	})
}

// deployedPipelineMap is the inner object a deployment renders — the six
// fields that define the deployed behavior. The same shape is digested on
// BOTH sides (control plane here, instance in RuntimeSnapshot), so a
// digest comparison proves which content an instance actually runs.
func deployedPipelineMap(cfg *pipeline.PipelineConfigV2, spec renderSpec) map[string]interface{} {
	pipelineEnabled := cfg.Enabled
	pipelineAutoStart := cfg.AutoStart
	if !spec.enabled {
		// explicit false pointer (nil would mean "default enabled" at
		// the instance's isPipelineEnabled check)
		f := false
		pipelineEnabled = &f
		pipelineAutoStart = false
	}
	return map[string]interface{}{
		"name":         cfg.Name,
		"enabled":      pipelineEnabled,
		"auto_start":   pipelineAutoStart,
		"keep_running": cfg.KeepRunning,
		"singleton":    spec.singleton,
		"processor":    cfg.Processors,
	}
}

// deployedContentDigest digests a rendered pipeline map — the identity the
// staged rollout compares instance reports against.
func deployedContentDigest(m map[string]interface{}) string {
	return util.MD5digest(string(util.MustToJSONBytes(m)))
}

// deployedContentDigestOf digests FULL rendered config bytes (the
// {"pipeline":[{...}]} wrapper a managed-config row carries) by
// re-canonicalizing the inner object — comparable with
// deployedContentDigest.
func deployedContentDigestOf(rendered []byte) string {
	var wrapper struct {
		Pipeline []map[string]interface{} `json:"pipeline"`
	}
	if err := util.FromJSONBytes(rendered, &wrapper); err != nil || len(wrapper.Pipeline) == 0 {
		return ""
	}
	return deployedContentDigest(wrapper.Pipeline[0])
}

// loadRegisteredInstances lists the instance universe selectors and
// explicit targeting resolve against.
func (h *PipeModule) loadRegisteredInstances(octx *orm.Context) []model.Instance {
	orm.WithModel(octx, &model.Instance{})
	res, err := orm.SearchV2(octx, orm.NewQuery().Size(500))
	if err != nil {
		return nil
	}
	instances, _, _ := elastic.DecodeHits[model.Instance](res)
	return instances
}

// deploymentTargetIDs resolves a deployment's scope to concrete instance
// IDs (broadcast = every registered instance).
func deploymentTargetIDs(dep *PipelineDeployment, instances []model.Instance) []string {
	targets, broadcast := resolveTargets(dep, instances)
	if broadcast {
		ids := make([]string, 0, len(instances))
		for i := range instances {
			ids = append(ids, instances[i].ID)
		}
		return ids
	}
	ids := make([]string, 0, len(targets))
	for id := range targets {
		ids = append(ids, id)
	}
	return ids
}

// rollbackDeploymentContent rewrites EVERY target's managed-config row to
// the deployment's recorded fallback content at once — a rollback is
// urgent, no staging (#125 _rollout action=rollback).
func (h *PipeModule) rollbackDeploymentContent(octx *orm.Context, dep *PipelineDeployment) error {
	instances := h.loadRegisteredInstances(octx)
	targets, broadcast := resolveTargets(dep, instances)
	if broadcast {
		targets = map[string]bool{}
		for i := range instances {
			targets[instances[i].ID] = true
		}
	}
	if len(targets) == 0 {
		return fmt.Errorf("deployment %s has no targets in scope right now", dep.Name)
	}

	configName := fmt.Sprintf("pipeline-deploy-%s.yml", dep.PipelineID)
	orm.WithModel(octx, &configs_server.ManagedConfig{})
	rowsRes, rerr := orm.SearchV2(octx, orm.NewQuery().
		Filter(orm.TermQuery("name", configName)).Size(500))
	if rerr != nil {
		return fmt.Errorf("query managed config %s: %w", configName, rerr)
	}
	rows, _, _ := elastic.DecodeHits[configs_server.ManagedConfig](rowsRes)
	existing := map[string]configs_server.ManagedConfig{}
	maxVersion := int64(0)
	for _, row := range rows {
		existing[row.InstanceID] = row
		if row.Version > maxVersion {
			maxVersion = row.Version
		}
	}

	for instID := range targets {
		row := existing[instID]
		row.Name = configName
		row.InstanceID = instID
		row.Content = dep.State.RollbackContent
		row.Version = maxVersion + 1
		row.Readonly = true
		if row.ID == "" {
			if instID == configs_server.AllInstancesID {
				row.ID = fmt.Sprintf("deploy-%s", dep.PipelineID)
			} else {
				row.ID = fmt.Sprintf("deploy-%s-%s", dep.PipelineID, instID)
			}
		}
		if err := orm.Save(octx, &row); err != nil {
			return fmt.Errorf("rollback row %s→%s: %w", configName, instID, err)
		}
		maxVersion = row.Version
	}
	return nil
}

// getDeploymentStatus builds the REPORTED state from instance runtime
// data (desired vs reported comparison for the cluster overview).
func (h *PipeModule) getDeploymentStatus(dep *PipelineDeployment) DeploymentStatus {
	status := DeploymentStatus{
		DeploymentID: dep.ID,
		Version:      dep.Version,
	}

	// query instances that have runtime pipeline data
	octx := orm.NewContext().DirectAccess()
	orm.WithModel(octx, &model.Instance{})
	q := orm.NewQuery().Size(200)
	res, err := orm.SearchV2(octx, q)
	if err != nil {
		return status
	}
	instances, _, _ := elastic.DecodeHits[model.Instance](res)

	// resolve the pipeline NAME — instances run pipelines under their
	// config name (YAML configs default ID = Name), not the config row ID
	taskCtx := orm.NewContext().DirectAccess()
	cfg := pipeline.PipelineConfigV2{}
	cfg.ID = dep.PipelineID
	if found, gerr := orm.GetV2(taskCtx, &cfg); gerr != nil || !found {
		return status
	}

	targets, broadcast := resolveTargets(dep, instances)

	for _, inst := range instances {
		inScope := broadcast || targets[inst.ID]
		// the honest "when did this instance last check in" — the ORM
		// record's updated stamp refreshes on every _sync heartbeat.
		// Read-time now() would show dead instances as freshly reporting
		// forever (#130: a killed gateway kept displaying STARTED with a
		// brand-new last_seen).
		lastSeen := time.Now().UTC()
		if inst.Updated != nil {
			lastSeen = inst.Updated.UTC()
		}
		if inst.RuntimePipelines == "" {
			// in-scope instances that have not reported yet are part of
			// the desired count — surface them as pending instead of
			// silently counting toward a number the list never explains
			if inScope && dep.Enabled {
				status.Instances = append(status.Instances, InstanceReport{
					InstanceID: inst.ID,
					State:      "PENDING",
					Version:    dep.Version,
					LastSeen:   lastSeen,
				})
			}
			continue
		}
		var pipelines map[string]struct {
			State  string `json:"state"`
			Reason string `json:"reason"`
		}
		if json.Unmarshal([]byte(inst.RuntimePipelines), &pipelines) != nil {
			continue
		}
		// check if this instance runs the deployment's pipeline
		if st, ok := pipelines[cfg.Name]; ok {
			status.Instances = append(status.Instances, InstanceReport{
				InstanceID: inst.ID,
				State:      st.State,
				Version:    dep.Version,
				LastSeen:   lastSeen,
				Reason:     st.Reason,
				// stale = the instance still runs the pipeline but the
				// scope no longer includes it (config recall in flight,
				// or a recall that failed — visible instead of silent)
				Stale:    !inScope,
				Cordoned: inst.Cordoned,
			})
		}
	}
	status.DesiredCount = len(instances)
	if !broadcast {
		status.DesiredCount = len(targets)
	}
	if dep.Placement.Strategy == "singleton" {
		status.DesiredCount = 1
	}
	status.ReadyCount = 0
	for _, ir := range status.Instances {
		if ir.Stale {
			continue
		}
		if ir.State == "STARTED" {
			status.ReadyCount++
		}
	}
	if dep.State.Active || len(dep.State.Rolled) > 0 {
		st := dep.State
		status.Rollout = &st
	}
	return status
}

// ─── Fleet API ───────────────────────────────────────────────────────────

// createDeploymentHandler — POST /pipeline/deployments/
func (h *PipeModule) createDeploymentHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var dep PipelineDeployment
	if err := h.DecodeJSON(req, &dep); err != nil {
		h.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if dep.Name == "" || dep.PipelineID == "" {
		h.WriteError(w, "name and pipeline_id are required", http.StatusBadRequest)
		return
	}
	if dep.Placement.Strategy == "" {
		dep.Placement.Strategy = "all" // v1 default (D-12: no sharded)
	}
	if dep.Placement.Strategy != "all" && dep.Placement.Strategy != "singleton" {
		h.WriteError(w, fmt.Sprintf("unknown strategy %q (all|singleton)", dep.Placement.Strategy), http.StatusBadRequest)
		return
	}
	for k, v := range dep.Placement.Selector {
		if k == "" || v == "" {
			delete(dep.Placement.Selector, k)
		}
	}

	// the referenced pipeline must exist AND compile — a deployment is a
	// distribution promise; accepting one that ships a broken config to
	// every instance just moves the failure to the far end
	octx := orm.NewContextWithParent(req.Context()).DirectAccess()
	cfg := pipeline.PipelineConfigV2{}
	cfg.ID = dep.PipelineID
	// GetV2 reports a missing row as an error ("record not found"); both
	// that and a genuine backend failure fail CLOSED here — a control
	// plane that cannot read the pipeline must not accept deployments
	exists, err := orm.GetV2(octx, &cfg)
	if err != nil || !exists {
		h.WriteError(w, fmt.Sprintf("pipeline %s not found", dep.PipelineID), http.StatusBadRequest)
		return
	}
	if !h.validateAndReject(w, &cfg) {
		return
	}

	dep.Version = 1
	dep.ID = util.GetUUID()

	if err := orm.Save(octx, &dep); err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	RequestReconcile()
	h.WriteJSON(w, dep, http.StatusCreated)
}

// listDeploymentsHandler — GET /pipeline/deployments/
func (h *PipeModule) listDeploymentsHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	octx := orm.NewContextWithParent(req.Context()).DirectAccess()
	orm.WithModel(octx, &PipelineDeployment{})
	q := orm.NewQuery().Size(100)
	res, err := orm.SearchV2(octx, q)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	deployments, total, _ := elastic.DecodeHits[PipelineDeployment](res)
	h.WriteJSON(w, util.MapStr{"total": total, "items": deployments}, http.StatusOK)
}

// getDeploymentHandler — GET /pipeline/deployments/:id
func (h *PipeModule) getDeploymentHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	octx := orm.NewContextWithParent(req.Context()).DirectAccess()
	dep := PipelineDeployment{}
	dep.ID = id
	exists, err := orm.GetV2(octx, &dep)
	if err != nil || !exists {
		h.WriteOpRecordNotFoundJSON(w, id)
		return
	}
	h.WriteJSON(w, dep, http.StatusOK)
}

// updateDeploymentHandler — PUT /pipeline/deployments/:id
func (h *PipeModule) updateDeploymentHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	var newDep PipelineDeployment
	if err := h.DecodeJSON(req, &newDep); err != nil {
		h.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}

	octx := orm.NewContextWithParent(req.Context()).DirectAccess()
	old := PipelineDeployment{}
	old.ID = id
	exists, err := orm.GetV2(octx, &old)
	if err != nil || !exists {
		h.WriteOpRecordNotFoundJSON(w, id)
		return
	}

	// re-validation on update: the edit can rebind pipeline_id — the same
	// distribution promise as create (must exist AND compile) holds
	cfg := pipeline.PipelineConfigV2{}
	cfg.ID = newDep.PipelineID
	pexists, perr := orm.GetV2(octx, &cfg)
	if perr != nil || !pexists {
		h.WriteError(w, fmt.Sprintf("pipeline %s not found", newDep.PipelineID), http.StatusBadRequest)
		return
	}
	if !h.validateAndReject(w, &cfg) {
		return
	}

	// normalize placement: default strategy, drop empty selector keys
	if newDep.Placement.Strategy == "" {
		newDep.Placement.Strategy = "all"
	}
	for k, v := range newDep.Placement.Selector {
		if k == "" || v == "" {
			delete(newDep.Placement.Selector, k)
		}
	}

	// preserve identity + bump version
	newDep.ID = id
	newDep.Version = old.Version + 1

	if err := orm.Save(octx, &newDep); err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	RequestReconcile()
	h.WriteJSON(w, newDep, http.StatusOK)
}

// deleteDeploymentHandler — DELETE /pipeline/deployments/:id
func (h *PipeModule) deleteDeploymentHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	octx := orm.NewContextWithParent(req.Context()).DirectAccess()
	orm.WithModel(octx, &PipelineDeployment{})
	dep := PipelineDeployment{}
	dep.ID = id
	if exists, gerr := orm.GetV2(octx, &dep); gerr != nil || !exists {
		h.WriteOpRecordNotFoundJSON(w, id)
		return
	}
	if err := orm.Delete(octx, &dep); err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// cascade: drop the pushed managed config when no OTHER deployment
	// still references the same pipeline — instances see the file Deleted
	// on their next sync and the config watcher removes + stops the task.
	// Rows are per-instance (or the "*" broadcast) — remove them all.
	others := orm.NewQuery().
		Filter(orm.TermQuery("pipeline_id", dep.PipelineID)).Size(10)
	if res, qerr := orm.SearchV2(octx, others); qerr == nil && res != nil {
		if rows, _, derr := elastic.DecodeHits[PipelineDeployment](res); derr == nil && len(rows) == 0 {
			configName := fmt.Sprintf("pipeline-deploy-%s.yml", dep.PipelineID)
			// switch the ORM context to the managed-config model — the
			// queries above ran against PipelineDeployment
			orm.WithModel(octx, &configs_server.ManagedConfig{})
			mcQ := orm.NewQuery().
				Filter(orm.TermQuery("name", configName)).Size(500)
			removed := 0
			if mcRes, merr := orm.SearchV2(octx, mcQ); merr == nil && mcRes != nil {
				if mcRows, _, mderr := elastic.DecodeHits[configs_server.ManagedConfig](mcRes); mderr == nil {
					for _, row := range mcRows {
						_ = orm.Delete(octx, &row)
						removed++
					}
					if removed > 0 {
						log.Infof("deployment %s deleted; cascaded %d managed config rows of %s (instances will recall the file)", dep.Name, removed, configName)
					}
				}
			}
		}
	}
	RequestReconcile()
	h.WriteJSON(w, util.MapStr{"deleted": true, "id": id}, http.StatusOK)
}

// getDeploymentStatusHandler — GET /pipeline/deployments/:id/status
func (h *PipeModule) getDeploymentStatusHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	octx := orm.NewContextWithParent(req.Context()).DirectAccess()
	dep := PipelineDeployment{}
	dep.ID = id
	exists, err := orm.GetV2(octx, &dep)
	if err != nil || !exists {
		h.WriteOpRecordNotFoundJSON(w, id)
		return
	}
	status := h.getDeploymentStatus(&dep)
	h.WriteJSON(w, status, http.StatusOK)
}

// getClusterOverviewHandler — GET /pipeline/cluster/overview
// Matrix of deployments × instances (who runs what, what version).
func (h *PipeModule) getClusterOverviewHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	octx := orm.NewContextWithParent(req.Context()).DirectAccess()
	orm.WithModel(octx, &PipelineDeployment{})
	q := orm.NewQuery().Filter(orm.TermQuery("enabled", true)).Size(100)
	res, err := orm.SearchV2(octx, q)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	deployments, _, _ := elastic.DecodeHits[PipelineDeployment](res)

	type overviewEntry struct {
		Deployment string            `json:"deployment"`
		Pipeline   string            `json:"pipeline"`
		Strategy   string            `json:"strategy"`
		Version    int               `json:"version"`
		Selector   map[string]string `json:"selector,omitempty"`
	}
	out := make([]overviewEntry, 0, len(deployments))
	for _, dep := range deployments {
		out = append(out, overviewEntry{
			Deployment: dep.Name,
			Pipeline:   dep.PipelineID,
			Strategy:   dep.Placement.Strategy,
			Version:    dep.Version,
			Selector:   dep.Placement.Selector,
		})
	}
	// enrich with reported status
	type overviewWithStatus struct {
		overviewEntry
		Status *DeploymentStatus `json:"status,omitempty"`
	}
	outWithStatus := make([]overviewWithStatus, 0, len(deployments))
	for _, dep := range deployments {
		entry := overviewEntry{
			Deployment: dep.Name,
			Pipeline:   dep.PipelineID,
			Strategy:   dep.Placement.Strategy,
			Version:    dep.Version,
			Selector:   dep.Placement.Selector,
		}
		st := h.getDeploymentStatus(&dep)
		outWithStatus = append(outWithStatus, overviewWithStatus{entry, &st})
	}
	h.WriteJSON(w, util.MapStr{"deployments": outWithStatus}, http.StatusOK)
}
