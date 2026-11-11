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

type staticLaneNoop struct{}

func (staticLaneNoop) Name() string                    { return "test_static_lane_noop" }
func (staticLaneNoop) Process(*corepipe.Context) error { return nil }

// HTTP stop/start preserves the configured lane namespace and overflow policy.
func TestStaticLaneTaskRestartPreservesConfig(t *testing.T) {
	corepipe.RegisterProcessorPlugin("test_static_lane_noop", func(*config.Config) (corepipe.Processor, error) { return staticLaneNoop{}, nil })
	name := t.Name()
	static := lane.LaneConfig{Name: name, Namespace: "test-family", Capacity: 7, Overflow: "drop"}
	if _, err := lane.Create(static); err != nil {
		t.Fatal(err)
	}
	m := &PipeModule{}
	cfg := corepipe.PipelineConfigV2{Name: name, AutoStart: true, Processors: []map[string]interface{}{{"lane_consumer": map[string]interface{}{"lane": name, "batch_max": 1, "processor": []interface{}{map[string]interface{}{"test_static_lane_noop": map[string]interface{}{}}}}}}}
	cfg.ID = name
	if err := m.createPipelineTask(cfg, true); err != nil {
		t.Fatal(err)
	}
	defer func() {
		m.stopAndWaitForRelease([]string{name}, time.Second)
		m.deleteTask(name)
		if err := lane.Delete(name, true); err != nil {
			t.Error(err)
		}
	}()
	obj, _ := m.contexts.Load(name)
	ctx := obj.(*corepipe.Context)
	await := func(fn func() bool) {
		t.Helper()
		deadline := time.Now().Add(time.Second)
		for !fn() {
			if time.Now().After(deadline) {
				t.Fatalf("task state did not settle: %v", ctx.GetRunningState())
			}
			time.Sleep(time.Millisecond)
		}
	}
	await(func() bool { return ctx.GetRunningState() == corepipe.STARTED })
	first := lane.Get(name)
	r := first.EmitAck(&event.Event{})
	wait, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := r.Wait(wait); err != nil {
		t.Fatal(err)
	}
	if got := first.Config(); got != static {
		t.Fatalf("first run already mismatched: %+v", got)
	}
	params := httprouter.Params{{Key: "id", Value: name}}
	response := httptest.NewRecorder()
	m.stopPipelineTaskHandler(response, httptest.NewRequest("POST", "/pipeline/task/"+name+"/_stop", nil), params)
	if response.Code != 200 {
		t.Fatal(response.Code)
	}
	await(func() bool { return ctx.GetRunningState() == corepipe.STOPPED && ctx.IsPause() })
	if lane.Get(name) != nil {
		t.Fatal("stopped lane is still open")
	}
	response = httptest.NewRecorder()
	m.startPipelineTaskHandler(response, httptest.NewRequest("POST", "/pipeline/task/"+name+"/_start", nil), params)
	if response.Code != 200 {
		t.Fatal(response.Code)
	}
	await(func() bool { return ctx.GetRunningState() == corepipe.STARTED && lane.Get(name) != nil })
	if got := lane.Get(name).Config(); got != static {
		t.Fatalf("HTTP stop/start lost declared config: got %+v; want %+v", got, static)
	}
}
