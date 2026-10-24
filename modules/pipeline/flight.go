/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infinilabs.com */

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
type flightRecorder struct {
	mu    sync.Mutex
	rings map[string][]pipeline.StepEvent
	seq   int64
}

func newFlightRecorder() *flightRecorder {
	return &flightRecorder{rings: map[string][]pipeline.StepEvent{}}
}

func (f *flightRecorder) OnStep(e *pipeline.StepEvent) {
	keep := e.Outcome != pipeline.OutcomeOK
	if !keep {
		n := atomic.AddInt64(&f.seq, 1)
		if n%flightSample != 0 {
			return
		}
	}
	cp := *e
	f.mu.Lock()
	ring := append(f.rings[e.Pipeline], cp)
	if len(ring) > flightRing {
		ring = ring[len(ring)-flightRing:]
	}
	f.rings[e.Pipeline] = ring
	f.mu.Unlock()
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

// recordPipelineEvent files a control-plane lifecycle event (singleton
// lock takeover...) into the flight ring — the alert surface operators
// watch. No-op when flight is not installed; non-ok outcomes are always
// retained by the recorder.
func recordPipelineEvent(pipelineName string, outcome pipeline.Outcome, msg string) {
	f := globalFlight.Load()
	if f == nil {
		return
	}
	f.(*flightRecorder).OnStep(&pipeline.StepEvent{
		Pipeline: pipelineName,
		Outcome:  outcome,
		Err:      msg,
	})
}
