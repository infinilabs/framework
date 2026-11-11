package pipeline

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/lane"
	corepipe "infini.sh/framework/core/pipeline"
)

type testPr433NoopProcessor struct{}

func (testPr433NoopProcessor) Name() string                    { return "test_pr433_noop" }
func (testPr433NoopProcessor) Process(*corepipe.Context) error { return nil }

// The REAL task stop/start cycle (HTTP handlers, processor reuse) must
// consume the lane the producers recreated — the compiled lane_consumer
// used to keep subscribing to the lane its own previous run deleted
// (PR #433 review).
func TestLaneTaskStopStartConsumesFreshLane(t *testing.T) {
	corepipe.RegisterProcessorPlugin("test_pr433_noop", func(*config.Config) (corepipe.Processor, error) { return testPr433NoopProcessor{}, nil })
	m := &PipeModule{}
	id := t.Name()
	defer func() {
		if lane.Get(id) != nil {
			_ = lane.Delete(id, true)
		}
	}()
	cfg := corepipe.PipelineConfigV2{Name: id, AutoStart: true, Processors: []map[string]interface{}{{"lane_consumer": map[string]interface{}{"lane": id, "batch_max": 1, "processor": []interface{}{map[string]interface{}{"test_pr433_noop": map[string]interface{}{}}}}}}}
	cfg.ID = id
	if err := m.createPipelineTask(cfg, true); err != nil {
		t.Fatal(err)
	}
	defer func() { m.stopAndWaitForRelease([]string{id}, time.Second); m.deleteTask(id) }()
	obj, _ := m.contexts.Load(id)
	ctx := obj.(*corepipe.Context)
	waitFor := func(fn func() bool) {
		t.Helper()
		end := time.Now().Add(time.Second)
		for !fn() {
			if time.Now().After(end) {
				t.Fatalf("task condition did not settle, state=%v", ctx.GetRunningState())
			}
			time.Sleep(time.Millisecond)
		}
	}
	waitFor(func() bool { return ctx.GetRunningState() == corepipe.STARTED })
	old := lane.Get(id)
	first := old.EmitAck(&event.Event{})
	before, cb := context.WithTimeout(context.Background(), time.Second)
	defer cb()
	if err := first.Wait(before); err != nil {
		t.Fatalf("initial task failed to consume: %v", err)
	}
	original, _ := m.pipelines.Load(id)
	params := httprouter.Params{{Key: "id", Value: id}}
	stopped := httptest.NewRecorder()
	m.stopPipelineTaskHandler(stopped, httptest.NewRequest("POST", "/pipeline/task/"+id+"/_stop", nil), params)
	if stopped.Code != 200 {
		t.Fatalf("stop API returned %d: %s", stopped.Code, stopped.Body.String())
	}
	waitFor(func() bool { return ctx.GetRunningState() == corepipe.STOPPED && ctx.IsPause() })
	fresh := lane.GetOrCreate(lane.LaneConfig{Name: id})
	if fresh == old {
		t.Fatal("stop did not remove the old lane")
	}
	started := httptest.NewRecorder()
	m.startPipelineTaskHandler(started, httptest.NewRequest("POST", "/pipeline/task/"+id+"/_start", nil), params)
	if started.Code != 200 {
		t.Fatalf("start API returned %d: %s", started.Code, started.Body.String())
	}
	waitFor(func() bool { return ctx.GetRunningState() == corepipe.STARTED })
	current, _ := m.pipelines.Load(id)
	if current != original {
		t.Fatal("task start rebuilt processors unexpectedly")
	}
	second := fresh.EmitAck(&event.Event{})
	after, ca := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer ca()
	if err := second.Wait(after); err != nil {
		t.Fatalf("HTTP stop/start reused the old processor and did not consume the fresh lane: %v", err)
	}
}
