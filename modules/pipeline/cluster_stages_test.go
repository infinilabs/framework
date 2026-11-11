/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infinilabs.com */

package pipeline

import (
	"testing"
	"time"
)

// statRow builds a persisted snapshot row shorthand.
func statRow(inst, pipeline, stage string, count, errors int64, avg int64, ts time.Time) PipelineStageStat {
	return PipelineStageStat{
		InstanceID: inst, Pipeline: pipeline, Stage: stage, Carrier: "processor",
		Count: count, Errors: errors, AvgLatency: avg, Ts: ts,
	}
}

// TestAggregateStageRows_ClusterSumMatchesParts covers the #130
// acceptance math: the cluster aggregate equals the sum of each
// instance's LATEST tick (older ticks superseded), and latency is the
// invocation-weighted mean across instances.
func TestAggregateStageRows_ClusterSumMatchesParts(t *testing.T) {
	t0 := time.Now()
	t1 := t0.Add(10 * time.Second) // instance A's second tick supersedes the first
	rows := []PipelineStageStat{
		// instance A: tick1 (stale), tick2 (latest) — only tick2 may count
		statRow("inst-a", "p1", "echo", 100, 1, 1_000_000, t0),
		statRow("inst-a", "p1", "echo", 300, 2, 2_000_000, t1),
		// instance B: single tick
		statRow("inst-b", "p1", "echo", 100, 0, 6_000_000, t0),
		// a different stage stays its own row
		statRow("inst-b", "p1", "grok", 50, 5, 3_000_000, t0),
	}
	// deliberately shuffle: the aggregator must not depend on input order
	rows = []PipelineStageStat{rows[1], rows[3], rows[0], rows[2]}

	out := aggregateStageRows(rows)
	if len(out) != 2 {
		t.Fatalf("want 2 stage rows, got %d: %+v", len(out), out)
	}

	var echo *ClusterStageStat
	for i := range out {
		if out[i].Stage == "echo" {
			echo = &out[i]
		}
	}
	if echo == nil {
		t.Fatal("echo stage row missing")
	}
	// aggregate = latest tick of A + tick of B (stale A tick1 ignored)
	if echo.Count != 400 || echo.Errors != 2 {
		t.Fatalf("echo aggregate = %d/%d, want 400/2", echo.Count, echo.Errors)
	}
	// weighted latency: (2ms*300 + 6ms*100) / 400 = 3ms
	if echo.AvgLatency != 3_000_000 {
		t.Fatalf("weighted avg latency = %d, want 3000000", echo.AvgLatency)
	}
	// per-instance detail present and consistent with the aggregate
	if len(echo.PerInstance) != 2 {
		t.Fatalf("per-instance detail missing: %+v", echo.PerInstance)
	}
	if a := echo.PerInstance["inst-a"]; a.Count != 300 {
		t.Fatalf("inst-a detail = %+v, want the latest tick (300)", a)
	}
	var sum int64
	for _, st := range echo.PerInstance {
		sum += st.Count
	}
	if sum != echo.Count {
		t.Fatalf("aggregate %d != sum of parts %d", echo.Count, sum)
	}
}
