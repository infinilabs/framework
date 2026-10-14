/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infinilabs.com */

package pipeline

import (
	"sync"
	"sync/atomic"
	"time"

	"infini.sh/framework/core/lane"
	"infini.sh/framework/core/pipeline"
)

// StageStats is the standard per-stage metric entry: one row per
// processor (in-process stages) plus one row per lane carrier — the
// runtime projection of the StepEvent stream.
type StageStats struct {
	Stage      string `json:"stage"`   // processor name or lane name
	Carrier    string `json:"carrier"` // "processor" | "lane:<name>"
	Count      int64  `json:"count"`   // invocations
	Errors     int64  `json:"errors"`
	AvgLatency int64  `json:"avg_latency_ns"` // mean per invocation
	LastOutcom string `json:"last_outcome,omitempty"`
}

type procStat struct {
	count    int64
	errors   int64
	duration int64 // total ns
	last     pipeline.Outcome
}

// statsRecorder aggregates StepEvents into per-pipeline processor stats.
type statsRecorder struct {
	mu  sync.Mutex
	per map[string]map[string]*procStat
	seq int64
}

func newStatsRecorder() *statsRecorder { return &statsRecorder{per: map[string]map[string]*procStat{}} }

func (s *statsRecorder) OnStep(e *pipeline.StepEvent) {
	// every event counts (aggregation is cheap: map ops); sampling is a
	// flight-recorder concern, not a counter concern
	s.mu.Lock()
	m := s.per[e.Pipeline]
	if m == nil {
		m = map[string]*procStat{}
		s.per[e.Pipeline] = m
	}
	st := m[e.Processor]
	if st == nil {
		st = &procStat{}
		m[e.Processor] = st
	}
	st.count++
	st.duration += int64(e.Duration)
	st.last = e.Outcome
	if e.Outcome == pipeline.OutcomeError {
		st.errors++
	}
	s.mu.Unlock()
}

// stageStats snapshots one pipeline's processor stages plus every lane
// carrier (depth from the live registry).
func stageStats(pipelineName string) []StageStats {
	out := []StageStats{}
	if rec := globalStats.Load(); rec != nil {
		s := rec.(*statsRecorder)
		s.mu.Lock()
		if m := s.per[pipelineName]; m != nil {
			for name, st := range m {
				entry := StageStats{Stage: name, Carrier: "processor", Count: st.count, Errors: st.errors, LastOutcom: string(st.last)}
				if st.count > 0 {
					entry.AvgLatency = st.duration / st.count
				}
				out = append(out, entry)
			}
		}
		s.mu.Unlock()
	}
	for _, l := range lane.List() {
		out = append(out, StageStats{
			Stage:   l.Name,
			Carrier: "lane:" + l.Name,
			Count:   l.Emitted,
			Errors:  l.Drops + l.Spills, // degraded deliveries surface as stage errors
		})
	}
	return out
}

var globalStats atomic.Value

func installStats() {
	if globalStats.Load() != nil {
		return
	}
	pipeline.RegisterObserver(newStatsRecorder())
	globalStats.Store(newStatsRecorder())
}

var _ = time.Second
