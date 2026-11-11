package pipeline

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"infini.sh/framework/core/model"
	"infini.sh/framework/core/orm"
	corepipe "infini.sh/framework/core/pipeline"
	configsserver "infini.sh/framework/modules/configs/server"
)

// Broadcast migration keeps each unrolled target on its previous content.
func TestBroadcastMigrationKeepsOldContentUntilWaveOpens(t *testing.T) {
	e := newDeployTestEnv(t)
	t.Cleanup(func() { e.deleteDeployments(t) })
	dep := &PipelineDeployment{Name: t.Name(), PipelineID: e.pid, Enabled: true}
	dep.ID = t.Name()
	e.saveDeployment(dep)
	instances := []model.Instance{
		{ORMObjectBase: orm.ORMObjectBase{ID: e.instA}},
		{ORMObjectBase: orm.ORMObjectBase{ID: e.instB}},
		{ORMObjectBase: orm.ORMObjectBase{ID: e.instC}},
	}
	m := e.module()
	reconcile := func() {
		t.Helper()
		if err := m.reconcilePipeline(orm.NewContext().DirectAccess(), e.pid, []*PipelineDeployment{dep}, instances); err != nil {
			t.Fatal(err)
		}
	}
	reconcile()
	old, ok := e.rows()["*"]
	if !ok {
		t.Fatal("initial broadcast row missing")
	}
	c := orm.NewContext().DirectAccess()
	cfg := corepipe.PipelineConfigV2{}
	cfg.ID = e.pid
	if _, err := orm.GetV2(c, &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Processors = []map[string]interface{}{{"validate_test_echo": map[string]interface{}{"message": "v2"}}}
	if err := orm.Save(c, &cfg); err != nil {
		t.Fatal(err)
	}
	dep.Placement.Rollout = RolloutSpec{Canary: 1, Batch: 1}
	e.saveDeployment(dep)
	// Fail the second preserved row so the retry covers a partial migration.
	var armed atomic.Bool
	var saved atomic.Int32
	injected := errors.New("preserved config write failed")
	orm.RegisterDataOperationPreHook(0, func(ctx *orm.Context, op orm.Operation, value interface{}) (*orm.Context, interface{}, error) {
		row, ok := value.(*configsserver.ManagedConfig)
		if ok && armed.Load() && row.Name == old.Name && row.InstanceID != "*" && row.Content == old.Content && saved.Add(1) == 2 {
			return ctx, value, injected
		}
		return ctx, value, nil
	}, orm.OpSave)
	t.Cleanup(func() { armed.Store(false) })
	armed.Store(true)
	if err := m.reconcilePipeline(orm.NewContext().DirectAccess(), e.pid, []*PipelineDeployment{dep}, instances); !errors.Is(err, injected) {
		t.Fatalf("expected preserved-row write failure, got %v", err)
	}
	partial := e.rows()
	if row, ok := partial["*"]; !ok || row.Content != old.Content {
		t.Fatal("partial migration removed the broadcast fallback")
	}
	if len(partial) != 2 {
		t.Fatalf("expected one saved target and the broadcast, got %v", rowInstanceIDs(partial))
	}
	armed.Store(false)
	reconcile()
	rows := e.rows()
	if _, exists := rows["*"]; exists {
		t.Fatal("wildcard must retire after old content is copied")
	}
	if len(rows) != 3 || rows[e.instA].Content == old.Content {
		t.Fatalf("expected one canary and two preserved targets, got %v", rowInstanceIDs(rows))
	}
	for _, id := range []string{e.instB, e.instC} {
		if row, ok := rows[id]; !ok || row.Content != old.Content || row.Version <= old.Version {
			t.Fatalf("unrolled instance %s lost its old config: %+v", id, rows[id])
		}
	}
	// A second pass without readiness must not rewrite the preserved rows.
	reconcile()
	for _, id := range []string{e.instB, e.instC} {
		if got := e.rows()[id]; got.Content != old.Content || got.Version != rows[id].Version {
			t.Fatalf("held wave changed %s: %+v", id, got)
		}
	}
	// Readiness opens B's wave but leaves C on the original broadcast bytes.
	digest := deployedContentDigestOf([]byte(rows[e.instA].Content))
	instances[0].RuntimePipelines = fmt.Sprintf(`{%q:{"state":"STARTED","config_digest":%q}}`, cfg.Name, digest)
	reconcile()
	rows = e.rows()
	if rows[e.instB].Content == old.Content || rows[e.instC].Content != old.Content {
		t.Fatal("readiness must advance only the next wave")
	}
}
