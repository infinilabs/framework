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
