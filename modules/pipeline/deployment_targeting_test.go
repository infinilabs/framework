/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infinilabs.com */

package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/model"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/modules/configs/server"
	sqlite "infini.sh/framework/modules/sqlite"
)

// deployTestEnv wires an in-memory control plane: sqlite ORM with the
// three schemas the reconciler touches (deployments, instances, managed
// configs) plus one saved, compilable pipeline config. The ORM handler
// is process-global, so every env gets unique instance/label/pipeline
// IDs and tests stay isolated.
type deployTestEnv struct {
	t        *testing.T
	pid      string
	pipeName string // the pipeline's config NAME (what instances run it under)
	instA    string
	instB    string
	instC    string
	prod     string // label value shared by instA+instC
	stage    string // label value of instB
	edge     string // extra label of instC
}

var (
	deployEnvSeq int
	deployORMOnce sync.Once
)

// ensureDeployORMHandler installs the ONE sqlite handler for the whole
// test binary (orm.Register is a process-global last-wins registry — a
// second registration anywhere would flip later tests onto a handler
// missing their schemas). Every schema any test in this package needs
// registers here; side-effect-free so stats tests can call it without
// minting instances.
func ensureDeployORMHandler(t *testing.T) {
	t.Helper()
	deployORMOnce.Do(func() {
		handler := &sqlite.SQLiteORM{Config: sqlite.SQLiteConfig{Enabled: true,
			DBPath: filepath.Join(os.TempDir(), fmt.Sprintf("pipeline-deploy-test-%d.db", time.Now().UnixNano()))}}
		if err := handler.Open(); err != nil {
			t.Fatal(err)
		}
		orm.Register("deploy-test", handler)
		orm.MustRegisterSchemaWithIndexName(pipeline.PipelineConfigV2{}, "pipeline-configs-test")
		orm.MustRegisterSchemaWithIndexName(PipelineDeployment{}, "pipeline-deployments-test")
		orm.MustRegisterSchemaWithIndexName(model.Instance{}, "instance-test")
		orm.MustRegisterSchemaWithIndexName(server.ManagedConfig{}, "managed-configs-test")
		orm.MustRegisterSchemaWithIndexName(PipelineStageStat{}, "pipeline-stage-stats-test")
		if err := orm.InitSchema(); err != nil {
			t.Fatal(err)
		}
	})
}

func newDeployTestEnv(t *testing.T) *deployTestEnv {
	deployEnvSeq++
	sfx := fmt.Sprintf("%d", deployEnvSeq)
	ensureDeployORMHandler(t)

	env := &deployTestEnv{
		t: t, pid: "pipe-" + sfx, pipeName: "deploy-test-pipeline-" + sfx,
		instA: "gw-a-" + sfx, instB: "gw-b-" + sfx, instC: "gw-c-" + sfx,
		prod: "prod-" + sfx, stage: "staging-" + sfx, edge: "edge-" + sfx,
	}
	env.saveInstance(env.instA, map[string]string{"env": env.prod})
	env.saveInstance(env.instB, map[string]string{"env": env.stage})
	env.saveInstance(env.instC, map[string]string{"env": env.prod, "tier": env.edge})

	octx := orm.NewContext().DirectAccess()
	orm.WithModel(octx, &pipeline.PipelineConfigV2{})
	cfg := pipeline.PipelineConfigV2{}
	cfg.ID = env.pid
	cfg.Name = env.pipeName
	cfg.Processors = []map[string]interface{}{
		{"validate_test_echo": map[string]interface{}{"message": "hi"}},
	}
	if err := orm.Save(octx, &cfg); err != nil {
		t.Fatal(err)
	}
	return env
}

func (e *deployTestEnv) saveInstance(id string, labels map[string]string) {
	octx := orm.NewContext().DirectAccess()
	orm.WithModel(octx, &model.Instance{})
	inst := model.Instance{}
	inst.ID = id
	inst.Labels = labels
	if err := orm.Save(octx, &inst); err != nil {
		e.t.Fatal(err)
	}
}

func (e *deployTestEnv) saveDeployment(dep *PipelineDeployment) {
	octx := orm.NewContext().DirectAccess()
	orm.WithModel(octx, &PipelineDeployment{})
	if err := orm.Save(octx, dep); err != nil {
		e.t.Fatal(err)
	}
}

// rows returns the deployment-owned managed-config rows keyed by
// instance_id.
func (e *deployTestEnv) rows() map[string]server.ManagedConfig {
	octx := orm.NewContext().DirectAccess()
	orm.WithModel(octx, &server.ManagedConfig{})
	res, err := orm.SearchV2(octx, orm.NewQuery().Filter(orm.TermQuery("name", fmt.Sprintf("pipeline-deploy-%s.yml", e.pid))).Size(100))
	if err != nil {
		e.t.Fatal(err)
	}
	rows, _, _ := elastic.DecodeHits[server.ManagedConfig](res)
	out := map[string]server.ManagedConfig{}
	for _, r := range rows {
		out[r.InstanceID] = r
	}
	return out
}

func (e *deployTestEnv) reconcile() {
	octx := orm.NewContext().DirectAccess()
	orm.WithModel(octx, &model.Instance{})
	instRes, err := orm.SearchV2(octx, orm.NewQuery().Size(100))
	if err != nil {
		e.t.Fatal(err)
	}
	instances, _, _ := elastic.DecodeHits[model.Instance](instRes)

	orm.WithModel(octx, &PipelineDeployment{})
	res, err := orm.SearchV2(octx, orm.NewQuery().Size(100))
	if err != nil {
		e.t.Fatal(err)
	}
	deployments, _, _ := elastic.DecodeHits[PipelineDeployment](res)
	for i := range deployments {
		if err := e.module().reconcilePipeline(octx, e.pid, []*PipelineDeployment{&deployments[i]}, instances); err != nil {
			e.t.Fatalf("reconcile: %v", err)
		}
	}
}

func (e *deployTestEnv) module() *PipeModule { return &PipeModule{} }

func TestResolveTargetsModes(t *testing.T) {
	instances := []model.Instance{
			{ORMObjectBase: orm.ORMObjectBase{ID: "a"}, Labels: map[string]string{"env": "prod"}},
		{ORMObjectBase: orm.ORMObjectBase{ID: "b"}, Labels: map[string]string{"env": "staging"}},
	}
	// no selector + no instances → broadcast
	targets, broadcast := resolveTargets(&PipelineDeployment{}, instances)
	if !broadcast || targets != nil {
		t.Fatalf("expected broadcast, got %v %v", targets, broadcast)
	}
	// label match
	targets, broadcast = resolveTargets(&PipelineDeployment{Placement: PlacementSpec{Selector: map[string]string{"env": "prod"}}}, instances)
	if broadcast || len(targets) != 1 || !targets["a"] {
		t.Fatalf("expected {a}, got %v broadcast=%v", targets, broadcast)
	}
	// multi-label AND
	targets, _ = resolveTargets(&PipelineDeployment{Placement: PlacementSpec{Selector: map[string]string{"env": "prod", "tier": "edge"}}}, instances)
	if len(targets) != 0 {
		t.Fatalf("expected no match for AND miss, got %v", targets)
	}
	// explicit IDs, unknown dropped
	targets, broadcast = resolveTargets(&PipelineDeployment{Placement: PlacementSpec{Instances: []string{"b", "ghost"}}}, instances)
	if broadcast || len(targets) != 1 || !targets["b"] {
		t.Fatalf("expected {b}, got %v broadcast=%v", targets, broadcast)
	}
	// explicit + labels union
	targets, _ = resolveTargets(&PipelineDeployment{Placement: PlacementSpec{Selector: map[string]string{"env": "prod"}, Instances: []string{"b"}}}, instances)
	if len(targets) != 2 || !targets["a"] || !targets["b"] {
		t.Fatalf("expected {a,b}, got %v", targets)
	}
}

func TestDeploymentLabelScopeConvergesExactly(t *testing.T) {
	env := newDeployTestEnv(t)
	env.saveDeployment(&PipelineDeployment{ORMObjectBase: orm.ORMObjectBase{ID: "d1"}, Name: "prod-only", PipelineID: env.pid, Enabled: true,
		Placement: PlacementSpec{Strategy: "all", Selector: map[string]string{"env": env.prod}}})

	env.reconcile()
	rows := env.rows()
	// exactly the two env=prod instances, no broadcast row, no staging
	if len(rows) != 2 || rows[env.instA].ID == "" || rows[env.instC].ID == "" || rows[env.instB].ID != "" || rows["*"].ID != "" {
		t.Fatalf("expected exactly {gw-a,gw-c}, got %v", rowIDs(rows))
	}
	v1 := rows[env.instA].Version

	// converge again → no version churn, still exactly two rows
	env.reconcile()
	rows = env.rows()
	if len(rows) != 2 || rows[env.instA].Version != v1 {
		t.Fatalf("expected stable convergence, got %s v%d", rowIDs(rows), rows["gw-a"].Version)
	}
}

func TestDeploymentScopeChangeRecallsOldInstances(t *testing.T) {
	env := newDeployTestEnv(t)
	env.saveDeployment(&PipelineDeployment{ORMObjectBase: orm.ORMObjectBase{ID: "d1"}, Name: "mover", PipelineID: env.pid, Enabled: true,
		Placement: PlacementSpec{Strategy: "all", Instances: []string{env.instA}}})

	env.reconcile()
	if rows := env.rows(); len(rows) != 1 || rows[env.instA].ID == "" {
		t.Fatalf("expected {gw-a}, got %v", rowIDs(rows))
	}
	oldVersion := env.rows()[env.instA].Version

	// scope A → B: switch to gw-b; gw-a must be recalled (row deleted),
	// gw-b's row must land ABOVE gw-a's old version or gw-a's client
	// would refuse the "update" it never sees... (version monotonicity
	// per name is what keeps instances from sticking to stale content)
	dep := PipelineDeployment{}
	{
		octx := orm.NewContext().DirectAccess()
		orm.WithModel(octx, &PipelineDeployment{})
		dep.ID = "d1"
		if found, err := orm.GetV2(octx, &dep); err != nil || !found {
			t.Fatal("deployment vanished")
		}
	}
	dep.Placement.Instances = []string{env.instB}
	env.saveDeployment(&dep)

	env.reconcile()
	rows := env.rows()
	if len(rows) != 1 || rows[env.instB].ID == "" {
		t.Fatalf("expected scope change to leave exactly {gw-b}, got %v", rowIDs(rows))
	}
	if rows[env.instB].Version <= oldVersion {
		t.Fatalf("new row version %d must exceed old %d (per-name monotonicity)", rows["gw-b"].Version, oldVersion)
	}
}

func TestDeploymentBroadcastMigratesToTargeted(t *testing.T) {
	env := newDeployTestEnv(t)
	env.saveDeployment(&PipelineDeployment{ORMObjectBase: orm.ORMObjectBase{ID: "d1"}, Name: "legacy", PipelineID: env.pid, Enabled: true,
		Placement: PlacementSpec{Strategy: "all"}})

	env.reconcile()
	rows := env.rows()
	if len(rows) != 1 || rows["*"].ID == "" {
		t.Fatalf("expected broadcast row, got %v", rowIDs(rows))
	}
	broadcastVersion := rows["*"].Version

	// edit the deployment to target one instance — the legacy "*" row
	// must be replaced by the per-instance row, seeded above its version
	dep := PipelineDeployment{}
	{
		octx := orm.NewContext().DirectAccess()
		orm.WithModel(octx, &PipelineDeployment{})
		dep.ID = "d1"
		if found, err := orm.GetV2(octx, &dep); err != nil || !found {
			t.Fatal("deployment vanished")
		}
	}
	dep.Placement.Selector = map[string]string{"tier": env.edge}
	env.saveDeployment(&dep)

	env.reconcile()
	rows = env.rows()
	if len(rows) != 1 || rows[env.instC].ID == "" || rows["*"].ID != "" {
		t.Fatalf("expected {gw-c} only, got %v", rowIDs(rows))
	}
	if rows[env.instC].Version <= broadcastVersion {
		t.Fatalf("migrated row version %d must exceed broadcast %d", rows["gw-c"].Version, broadcastVersion)
	}
}

func rowIDs(rows map[string]server.ManagedConfig) []string {
	out := []string{}
	for id := range rows {
		out = append(out, id)
	}
	return out
}
