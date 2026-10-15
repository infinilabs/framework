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
}

type InstanceReport struct {
	InstanceID string    `json:"instance_id"`
	State      string    `json:"state"` // running | stopped | failed | unknown
	Version    int       `json:"version"`
	LastSeen   time.Time `json:"last_seen"`
}

// ─── reconciler ──────────────────────────────────────────────────────────

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
				if module.closed.Load() {
					return
				}
				module.reconcileOnce()
			}
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
	orm.WithModel(octx, &PipelineDeployment{})

	q := orm.NewQuery().Filter(orm.TermQuery("enabled", true)).Size(100)
	res, err := orm.SearchV2(octx, q)
	if err != nil {
		return
	}
	deployments, _, _ := elastic.DecodeHits[PipelineDeployment](res)

	for _, dep := range deployments {
		if rerr := module.reconcileDeployment(&dep); rerr != nil {
			log.Warnf("reconciler: deployment %s: %v", dep.Name, rerr)
		}
	}
}

// reconcileDeployment converges ONE deployment toward its desired state.
// v1: push the pipeline config as a managed config row targeting the
// selected instances. Instance-side lock contention resolves singleton.
func (module *PipeModule) reconcileDeployment(dep *PipelineDeployment) error {
	// load the pipeline config
	octx := orm.NewContext().DirectAccess()
	cfg := pipeline.PipelineConfigV2{}
	cfg.ID = dep.PipelineID
	exists, err := orm.GetV2(octx, &cfg)
	if err != nil || !exists {
		return fmt.Errorf("pipeline %s not found", dep.PipelineID)
	}

	// translate to a managed-config row (the same channel stream publish
	// uses — instances pull and hot-reload)
	// v1: broadcast to all instances (strategy "all")
	// TODO: selector-based targeting, singleton lock, staged rollout
	configName := fmt.Sprintf("pipeline-deploy-%s.yml", dep.PipelineID)
	cfgJSON := util.MustToJSONBytes(map[string]interface{}{
		"pipeline": []interface{}{map[string]interface{}{
			"name":         cfg.Name,
			"enabled":      cfg.Enabled,
			"auto_start":   cfg.AutoStart,
			"keep_running": cfg.KeepRunning,
			"singleton":    dep.Placement.Strategy == "singleton",
			"processor":    cfg.Processors,
		}},
	})

	// upsert the ManagedConfig row (instances pull on next _sync)
	orm.WithModel(octx, &configs_server.ManagedConfig{})
	existing := configs_server.ManagedConfig{}
	foundQ := orm.NewQuery().
		Filter(orm.TermQuery("name", configName)).
		Filter(orm.TermQuery("instance_id", "*")).Size(1)
	foundRes, ferr := orm.SearchV2(octx, foundQ)
	if ferr == nil && foundRes != nil {
		if rows, _, derr := elastic.DecodeHits[configs_server.ManagedConfig](foundRes); derr == nil && len(rows) > 0 {
			existing = rows[0]
		}
	}

	existing.Name = configName
	existing.InstanceID = "*" // v1: broadcast; selector targeting next
	existing.Content = string(cfgJSON)
	existing.Version++
	existing.Readonly = true // deployment-owned

	if existing.ID == "" {
		existing.ID = fmt.Sprintf("deploy-%s", dep.PipelineID)
	}
	if serr := orm.Save(octx, &existing); serr != nil {
		return fmt.Errorf("push managed config: %w", serr)
	}

	log.Debugf("reconciler: deployment %s → config %s v%d (%d bytes, strategy=%s)",
		dep.Name, configName, existing.Version, len(cfgJSON), dep.Placement.Strategy)
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

	for _, inst := range instances {
		if inst.RuntimePipelines == "" {
			continue
		}
		var pipelines map[string]struct {
			State string `json:"state"`
		}
		if json.Unmarshal([]byte(inst.RuntimePipelines), &pipelines) != nil {
			continue
		}
		// check if this instance runs the deployment's pipeline
		taskName := dep.PipelineID
		if st, ok := pipelines[taskName]; ok {
			status.Instances = append(status.Instances, InstanceReport{
				InstanceID: inst.ID,
				State:      st.State,
				Version:    dep.Version,
				LastSeen:   time.Now().UTC(),
			})
		}
	}
	status.DesiredCount = 1 // v1: all strategy = every instance; singleton = 1
	if dep.Placement.Strategy == "singleton" {
		status.DesiredCount = 1
	}
	status.ReadyCount = 0
	for _, ir := range status.Instances {
		if ir.State == "STARTED" {
			status.ReadyCount++
		}
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
	dep.Version = 1
	dep.ID = util.GetUUID()

	octx := orm.NewContextWithParent(req.Context())
	if err := orm.Save(octx, &dep); err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
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

	// preserve identity + bump version
	newDep.ID = id
	newDep.Version = old.Version + 1

	if err := orm.Save(octx, &newDep); err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.WriteJSON(w, newDep, http.StatusOK)
}

// deleteDeploymentHandler — DELETE /pipeline/deployments/:id
func (h *PipeModule) deleteDeploymentHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	octx := orm.NewContextWithParent(req.Context()).DirectAccess()
	dep := PipelineDeployment{}
	dep.ID = id
	if err := orm.Delete(octx, &dep); err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// TODO: remove the managed config row (cascade)
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
