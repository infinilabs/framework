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

// Loss must be counted, never silent: OK steps skipped by sampling and
// entries evicted from a full ring both land in the lost counters.
func TestFlight_LossCounters(t *testing.T) {
	installFlight()
	f := globalFlight.Load().(*flightRecorder)

	errsBefore := f.evicted.Load()
	sampledBefore := f.sampledOut.Load()

	// 200 OK steps: all but ~2 are sampled out
	for i := 0; i < 200; i++ {
		f.OnStep(&pipeline.StepEvent{Pipeline: "loss", Outcome: pipeline.OutcomeOK})
	}
	sampled := f.sampledOut.Load() - sampledBefore
	if sampled < 190 || sampled > 198 { // ~99% of 200
		t.Fatalf("sampledOut = %d, want ~198", sampled)
	}

	// errors only: two full rings worth — the older ring overflows
	for i := 0; i < flightRing*2; i++ {
		f.OnStep(&pipeline.StepEvent{Pipeline: "loss-ring", Outcome: pipeline.OutcomeError})
	}
	if evicted := f.evicted.Load() - errsBefore; evicted != flightRing {
		t.Fatalf("evicted = %d, want %d", evicted, flightRing)
	}

	// stats surface through the package helper (handler reads this)
	lost := flightLostStats()
	if lost.SampledOut < sampled || lost.Evicted < flightRing {
		t.Fatalf("flightLostStats = %+v, want counters >= this test's deltas", lost)
	}
}
