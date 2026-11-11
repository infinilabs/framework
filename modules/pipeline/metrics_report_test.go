/* Copyright © INFINI Ltd. All rights reserved. */

package pipeline

import (
	"testing"
	"time"

	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/modules/configs/common"
)

// the package's single sqlite handler (see ensureDeployORMHandler) already
// carries the pipeline-stage-stats-test schema — reuse it, no second
// orm.Register (last-wins would strand the deploy tests' schemas)

func TestReportedStageRows_TagByPipelineAndDedupNames(t *testing.T) {
	installStats()
	rec := globalStats.Load().(*statsRecorder)
	rec.OnStep(&pipeline.StepEvent{Pipeline: "pA", Processor: "a1", Outcome: pipeline.OutcomeOK})
	rec.OnStep(&pipeline.StepEvent{Pipeline: "pA", Processor: "a2", Outcome: pipeline.OutcomeOK})
	rec.OnStep(&pipeline.StepEvent{Pipeline: "pB", Processor: "b1", Outcome: pipeline.OutcomeOK})

	module := &PipeModule{}
	// two context keys resolving to the SAME Config.Name — the seen-map
	// must collect pA once (id-less configs key by name; duplicate keys
	// under one name must not double the rows)
	module.contexts.Store("pA", pipeline.AcquireContext(pipeline.PipelineConfigV2{Name: "pA"}))
	module.contexts.Store("pA-id-alias", pipeline.AcquireContext(pipeline.PipelineConfigV2{Name: "pA"}))
	module.contexts.Store("pB", pipeline.AcquireContext(pipeline.PipelineConfigV2{Name: "pB"}))
	// a context whose Config.Name is empty falls back to the key
	module.contexts.Store("pC", pipeline.AcquireContext(pipeline.PipelineConfigV2{}))

	rows := module.reportedStageRows()
	type key struct{ p, s string }
	got := map[key]int64{}
	for _, r := range rows {
		if r.Carrier != "processor" {
			t.Fatalf("unexpected carrier in report: %+v", r)
		}
		got[key{r.Pipeline, r.Stage}]++
	}
	want := map[key]int64{
		{"pA", "a1"}: 1, {"pA", "a2"}: 1,
		{"pB", "b1"}: 1,
		{"pC", "pC"}: 0, // placeholder replaced below; pC has no recorded steps
	}
	delete(want, key{"pC", "pC"})
	if len(got) != len(want) {
		t.Fatalf("rows = %+v, want exactly %+v", rows, want)
	}
	for k, n := range want {
		if got[k] != n {
			t.Fatalf("row %v appears %d times, want %d (rows: %+v)", k, got[k], n, rows)
		}
	}
}

func TestPersistReportedStageStats_SavesAndPrunes(t *testing.T) {
	ensureDeployORMHandler(t)
	prevRetention := stageStatsRetention
	defer func() { stageStatsRetention = prevRetention }()

	octx := orm.NewContext().DirectAccess()
	PersistReportedStageStats(octx, []common.StageStat{
		{Pipeline: "sp", InstanceID: "inst-1", Stage: "echo", Carrier: "processor", Count: 10, Errors: 0, AvgLatency: 1000},
		{Pipeline: "sp", InstanceID: "inst-1", Stage: "idle", Carrier: "processor", Count: 0, Errors: 0}, // zero row skipped
	})

	// a stale row beyond the retention window — direct save, Ts backdated
	old := PipelineStageStat{
		InstanceID: "inst-0", Pipeline: "sp", Stage: "echo", Carrier: "processor",
		Count: 1, Ts: time.Now().UTC().Add(-2 * time.Hour),
	}
	old.ID = "stats-old-tick"
	if err := orm.Save(octx, &old); err != nil {
		t.Fatal(err)
	}

	countRows := func() int {
		qctx := orm.NewContext().DirectAccess()
		orm.WithModel(qctx, &PipelineStageStat{})
		res, err := orm.SearchV2(qctx, orm.NewQuery().Size(100))
		if err != nil {
			t.Fatal(err)
		}
		rows, _, _ := elastic.DecodeHits[PipelineStageStat](res)
		return len(rows)
	}

	if n := countRows(); n != 2 {
		t.Fatalf("after sink: %d rows, want 2 (fresh + backdated)", n)
	}

	// 1h retention prunes the 2h-old row on the next persist pass
	stageStatsRetention = time.Hour
	PersistReportedStageStats(octx, []common.StageStat{
		{Pipeline: "sp", InstanceID: "inst-1", Stage: "echo", Carrier: "processor", Count: 12, Errors: 0, AvgLatency: 1000},
	})
	if n := countRows(); n != 2 {
		t.Fatalf("after prune: %d rows, want 2 (two fresh ticks, backdated gone)", n)
	}
}
