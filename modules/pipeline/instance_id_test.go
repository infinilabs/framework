/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infinilabs.com */

package pipeline

import (
	"testing"

	"infini.sh/framework/core/global"
	"infini.sh/framework/core/pipeline"
)

// TestStageStatsCarryInstanceID — every stage row self-describes its
// emitting node (#127), so rows shipped to cluster views or persisted
// cross-instance can be grouped by origin.
func TestStageStatsCarryInstanceID(t *testing.T) {
	installStats()
	rec := globalStats.Load().(*statsRecorder)
	rec.OnStep(&pipeline.StepEvent{Pipeline: "iid-1", Processor: "echo", Outcome: pipeline.OutcomeOK})

	rows := stageStats("iid-1")
	if len(rows) == 0 {
		t.Fatal("no stage rows after feeding the recorder")
	}
	self := global.Env().SystemConfig.NodeConfig.ID
	for _, r := range rows {
		if r.InstanceID != self {
			t.Fatalf("stage %q row instance_id = %q, want %q", r.Stage, r.InstanceID, self)
		}
	}
}

// TestRecordPipelineEventStampsInstance — control-plane events (lock
// takeover) land in the flight ring attributed to this instance.
func TestRecordPipelineEventStampsInstance(t *testing.T) {
	installFlight()
	recordPipelineEvent("iid-evt-1", pipeline.OutcomeTakeover, "took from node-X")
	evs := flightQuery("iid-evt-1", "", true, 10)
	if len(evs) != 1 {
		t.Fatalf("event not retained: %+v", evs)
	}
	if evs[0].InstanceID != global.Env().SystemConfig.NodeConfig.ID {
		t.Fatalf("event instance_id = %q", evs[0].InstanceID)
	}
}
