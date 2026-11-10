package pipeline

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/model"
	"infini.sh/framework/core/orm"
	corepipe "infini.sh/framework/core/pipeline"
)

// deleteDeployments removes this env's persisted deployments: the shared
// test DB outlives each test, and env.reconcile() of a LATER test
// reconciles every leftover row under its own pid — leftovers stomp that
// test's converged rows otherwise.
func (e *deployTestEnv) deleteDeployments(t *testing.T) {
	t.Helper()
	octx := orm.NewContext().DirectAccess()
	orm.WithModel(octx, &PipelineDeployment{})
	res, err := orm.SearchV2(octx, orm.NewQuery().Filter(orm.TermQuery("pipeline_id", e.pid)).Size(100))
	if err != nil {
		return
	}
	deps, _, _ := elastic.DecodeHits[PipelineDeployment](res)
	for i := range deps {
		_ = orm.Delete(octx, &deps[i])
	}
}

// Targeted deployments must not leak their flags onto the broadcast
// scope: a disabled broadcast + an enabled single-instance deployment
// renders per-instance rows, never a globally-enabled wildcard
// (PR #433 review).
func TestReconcileDisabledBroadcastStaysScoped(t *testing.T) {
	e := newDeployTestEnv(t)
	deps := []*PipelineDeployment{
		{Name: "old-broadcast", PipelineID: e.pid, Enabled: false},
		{Name: "new-scope", PipelineID: e.pid, Enabled: true, Placement: PlacementSpec{Instances: []string{e.instA}}},
	}
	instances := []model.Instance{{ORMObjectBase: orm.ORMObjectBase{ID: e.instA}}, {ORMObjectBase: orm.ORMObjectBase{ID: e.instB}}}
	if err := e.module().reconcilePipeline(orm.NewContext().DirectAccess(), e.pid, deps, instances); err != nil {
		t.Fatal(err)
	}
	rows := e.rows()
	for id, row := range rows {
		if id == "*" && !strings.Contains(row.Content, `"enabled":false`) {
			t.Fatalf("targeted enabled deployment escaped onto an enabled broadcast row: %s", row.Content)
		}
	}
	if len(rows) == 0 {
		t.Fatal("reconcile wrote no rows")
	}
	if _, ok := rows[e.instB]; !ok || strings.Contains(rows[e.instB].Content, `"enabled":true`) {
		t.Fatalf("instance B must receive the disabled broadcast base: %v", rows[e.instB])
	}
}

// A broadcast deployment with a staged rollout must resolve into real
// instance targets — a wildcard row reaches everyone in one wave and
// defeats canary/batch staging (PR #433 review).
func TestBroadcastCanaryStagesInstances(t *testing.T) {
	e := newDeployTestEnv(t)
	t.Cleanup(func() { e.deleteDeployments(t) })
	dep := &PipelineDeployment{Name: "canary", PipelineID: e.pid, Enabled: true, Placement: PlacementSpec{Rollout: RolloutSpec{Canary: 1, Batch: 1}}}
	dep.ID = "pr433-canary"
	e.saveDeployment(dep)
	instances := []model.Instance{{ORMObjectBase: orm.ORMObjectBase{ID: e.instA}}, {ORMObjectBase: orm.ORMObjectBase{ID: e.instB}}}
	if err := e.module().reconcilePipeline(orm.NewContext().DirectAccess(), e.pid, []*PipelineDeployment{dep}, instances); err != nil {
		t.Fatal(err)
	}
	rows := e.rows()
	if _, all := rows["*"]; all {
		t.Fatal("canary:1 wrote a wildcard row which updates both target instances simultaneously")
	}
	if len(rows) != 1 {
		t.Fatalf("canary first wave must open exactly one instance row, got %d: %v", len(rows), rows)
	}
}

// End-to-end reconcileOnce on the disabled-broadcast + enabled-single
// combination (PR #433 review).
func TestReconcileOnceDisabledBroadcastStaysScoped(t *testing.T) {
	e := newDeployTestEnv(t)
	t.Cleanup(func() { e.deleteDeployments(t) })
	c := orm.NewContext().DirectAccess()
	cfg := corepipe.PipelineConfigV2{}
	cfg.ID = e.pid
	if _, err := orm.GetV2(c, &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.AutoStart = true
	enabled := true
	cfg.Enabled = &enabled
	if err := orm.Save(c, &cfg); err != nil {
		t.Fatal(err)
	}
	a := &PipelineDeployment{Name: "disabled-all", PipelineID: e.pid, Enabled: false}
	a.ID = "pr433-disabled-all"
	e.saveDeployment(a)
	b := &PipelineDeployment{Name: "enabled-one", PipelineID: e.pid, Enabled: true, Placement: PlacementSpec{Strategy: "all", Instances: []string{e.instA}}}
	b.ID = "pr433-enabled-one"
	e.saveDeployment(b)
	e.module().reconcileOnce()
	for id, row := range e.rows() {
		if id == "*" && strings.Contains(row.Content, `"auto_start":true`) && !strings.Contains(row.Content, `"enabled":false`) {
			t.Fatalf("real reconcileOnce enabled the pipeline on all instances: %s", row.Content)
		}
	}
}

// The deployment-creation API must accept a broadcast canary and the
// reconciler must stage it per instance (PR #433 review).
func TestCreateBroadcastCanaryViaAPIStages(t *testing.T) {
	e := newDeployTestEnv(t)
	t.Cleanup(func() { e.deleteDeployments(t) })
	m := e.module()
	body := fmt.Sprintf(`{"name":"canary","pipeline_id":%q,"enabled":true,"placement":{"rollout":{"canary":1,"batch":1}}}`, e.pid)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/pipeline/deployments/", strings.NewReader(body))
	m.createDeploymentHandler(w, req, nil)
	if w.Code != 201 {
		t.Fatalf("create rejected: %d %s", w.Code, w.Body.String())
	}
	m.reconcileOnce()
	if row, ok := e.rows()["*"]; ok {
		t.Fatalf("valid API canary created a broadcast row instead of a single-instance stage: %s", row.Content)
	}
}
