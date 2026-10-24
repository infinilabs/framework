/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infinilabs.com */

package pipeline

import (
	"testing"

	"infini.sh/framework/core/pipeline"
)

func TestFlight_RecordsErrorsAlwaysAndSamplesOK(t *testing.T) {
	installFlight()
	f := globalFlight.Load().(*flightRecorder)

	// errors always land
	f.OnStep(&pipeline.StepEvent{Pipeline: "fp", Outcome: pipeline.OutcomeError, Err: "boom", TraceID: "tr-1"})
	// OK steps: 2x flightSample land at most 2
	for i := 0; i < 2*flightSample; i++ {
		f.OnStep(&pipeline.StepEvent{Pipeline: "fp", Outcome: pipeline.OutcomeOK})
	}
	errs := flightQuery("fp", "", true, 10)
	if len(errs) != 1 || errs[0].Err != "boom" {
		t.Fatalf("error query: %+v", errs)
	}
	all := flightQuery("fp", "", false, flightRing)
	if len(all) > 2+3 { // 2 sampled + error + slack
		t.Fatalf("OK sampling not applied: %d events", len(all))
	}
	// trace filter
	if got := flightQuery("fp", "tr-1", false, 10); len(got) != 1 || got[0].TraceID != "tr-1" {
		t.Fatalf("trace query: %+v", got)
	}
}

func TestFlight_RingBounded(t *testing.T) {
	installFlight()
	f := globalFlight.Load().(*flightRecorder)
	for i := 0; i < flightRing*2; i++ {
		f.OnStep(&pipeline.StepEvent{Pipeline: "ring", Outcome: pipeline.OutcomeError})
	}
	f.mu.Lock()
	n := len(f.rings["ring"])
	f.mu.Unlock()
	if n != flightRing {
		t.Fatalf("ring = %d, want %d", n, flightRing)
	}
}
