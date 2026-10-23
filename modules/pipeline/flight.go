/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"sync"
	"sync/atomic"

	"infini.sh/framework/core/pipeline"
)

// flightRing caps the retained steps per pipeline.
const flightRing = 1024

// flightSample keeps 1/N of the OK steps; error steps are ALWAYS kept.
const flightSample = 100

// flightRecorder is the sampled execution observer: a bounded ring of
// StepEvents per pipeline, errors always captured, OK steps sampled 1/N.
// It installs globally at PipeModule setup — the empty-registry fast
// path applies before installation, and the per-step cost after is one
// counter increment plus the sampled append.
// Loss is counted, never silent: sampledOut tracks OK steps skipped by
// the 1/N sampling, evicted tracks entries pushed out of a full ring —
// both surfaced through the flight endpoint so consumers know how much
// of the picture they are not seeing.
type flightRecorder struct {
	mu    sync.Mutex
	rings map[string][]pipeline.StepEvent
	seq   int64

	sampledOut atomic.Int64
	evicted    atomic.Int64
}

func newFlightRecorder() *flightRecorder {
	return &flightRecorder{rings: map[string][]pipeline.StepEvent{}}
}

func (f *flightRecorder) OnStep(e *pipeline.StepEvent) {
	keep := e.Outcome != pipeline.OutcomeOK
	if !keep {
		n := atomic.AddInt64(&f.seq, 1)
		if n%flightSample != 0 {
			f.sampledOut.Add(1)
			return
		}
	}
	cp := *e
	f.mu.Lock()
	ring := append(f.rings[e.Pipeline], cp)
	if len(ring) > flightRing {
		f.evicted.Add(int64(len(ring) - flightRing))
		ring = ring[len(ring)-flightRing:]
	}
	f.rings[e.Pipeline] = ring
	f.mu.Unlock()
}

// FlightLostStats is the loss accounting of the flight recorder,
// cumulative since process start.
type FlightLostStats struct {
	// OK steps skipped by the 1/N sampling (by design, still a cost
	// consumers should see).
	SampledOut int64 `json:"sampled_out"`
	// Oldest entries pushed out of a full ring (oldest-first eviction).
	Evicted int64 `json:"evicted"`
}

// flightLostStats reads the recorder's loss counters; zero values when
// no recorder is installed.
func flightLostStats() FlightLostStats {
	f := globalFlight.Load()
	if f == nil {
		return FlightLostStats{}
	}
	rec := f.(*flightRecorder)
	return FlightLostStats{
		SampledOut: rec.sampledOut.Load(),
		Evicted:    rec.evicted.Load(),
	}
}

// flightQuery filters a pipeline's ring: by trace id, errors-only, newest
// first, capped at limit.
func flightQuery(pipelineName, traceID string, errorsOnly bool, limit int) []pipeline.StepEvent {
	if limit <= 0 || limit > flightRing {
		limit = 50
	}
	f := globalFlight.Load()
	if f == nil {
		return nil
	}
	rec := f.(*flightRecorder)
	rec.mu.Lock()
	ring := rec.rings[pipelineName]
	out := make([]pipeline.StepEvent, 0, limit)
	for i := len(ring) - 1; i >= 0 && len(out) < limit; i-- {
		e := ring[i]
		if traceID != "" && e.TraceID != traceID {
			continue
		}
		if errorsOnly && e.Outcome == pipeline.OutcomeOK {
			continue
		}
		out = append(out, e)
	}
	rec.mu.Unlock()
	return out
}

var globalFlight atomic.Value

// installFlight wires the recorder once (idempotent).
func installFlight() {
	if globalFlight.Load() != nil {
		return
	}
	pipeline.RegisterObserver(newFlightRecorder())
	globalFlight.Store(newFlightRecorder())
}
