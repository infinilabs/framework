/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infinilabs.com */

package pipeline

import (
	"testing"
	"time"

	"infini.sh/framework/core/pipeline"
)

func TestStats_AggregatesPerProcessor(t *testing.T) {
	installStats()
	s := globalStats.Load().(*statsRecorder)
	s.OnStep(&pipeline.StepEvent{Pipeline: "sp", Processor: "a", Outcome: pipeline.OutcomeOK, Duration: 100 * time.Millisecond})
	s.OnStep(&pipeline.StepEvent{Pipeline: "sp", Processor: "a", Outcome: pipeline.OutcomeOK, Duration: 300 * time.Millisecond})
	s.OnStep(&pipeline.StepEvent{Pipeline: "sp", Processor: "a", Outcome: pipeline.OutcomeError, Duration: 50 * time.Millisecond})

	stages := stageStats("sp")
	var found *StageStats
	for i := range stages {
		if stages[i].Stage == "a" && stages[i].Carrier == "processor" {
			found = &stages[i]
		}
	}
	if found == nil {
		t.Fatalf("stage a missing: %+v", stages)
	}
	if found.Count != 3 || found.Errors != 1 {
		t.Fatalf("counters: %+v", found)
	}
	if want := int64(150 * time.Millisecond); found.AvgLatency != want {
		t.Fatalf("avg latency = %d, want %d", found.AvgLatency, want)
	}
}

// obsWiringProc pairs with TestStats_ObserverWiring: a no-op processor so
// the shared Processors.Process loop runs it and emits real StepEvents.
type obsWiringProc struct{}

func (obsWiringProc) Name() string               { return "wiring_probe" }
func (obsWiringProc) Process(_ *pipeline.Context) error { return nil }

// TestStats_ObserverWiring guards the installStats identity contract: the
// recorder registered as observer must be the one stageStats reads. The
// historical bug registered one instance and stored a second — every
// stage view stayed empty while counters accumulated into a dead object.
func TestStats_ObserverWiring(t *testing.T) {
	installStats()

	ctx := pipeline.AcquireContext(pipeline.PipelineConfigV2{Name: "stats-wiring"})
	ctx.Started()
	chain := &pipeline.Processors{List: []pipeline.Processor{obsWiringProc{}}}
	if err := chain.Process(ctx); err != nil {
		t.Fatal(err)
	}

	stages := stageStats("stats-wiring")
	for _, st := range stages {
		if st.Stage == "wiring_probe" && st.Carrier == "processor" && st.Count >= 1 {
			return
		}
	}
	t.Fatalf("observer-fed counters missing from stageStats: %+v", stages)
}
