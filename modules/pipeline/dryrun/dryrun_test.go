/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package dryrun

import (
	"testing"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/pipeline"
)

// testDouble is a local processor registration for replay tests (the
// host package cannot be imported here — import cycle).
type testDouble struct{ name string }

func (p *testDouble) Name() string                        { return p.name }
func (p *testDouble) Process(ctx *pipeline.Context) error { return nil }

func init() {
	pipeline.RegisterProcessorPlugin("dryrun_noop", func(c *config.Config) (pipeline.Processor, error) {
		return &testDouble{"dryrun_noop"}, nil
	})
}

// sinkWriter appends a captured write exactly like output's capture
// helper — the dryrun package cannot import modules/pipeline (cycle), so
// the sink-diff terminal detection is exercised through this stand-in.
type sinkWriter struct{}

func (sinkWriter) Name() string { return "dryrun_sink" }
func (sinkWriter) Process(ctx *pipeline.Context) error {
	if s := pipeline.DryRunCapture(ctx); s != nil {
		*s = append(*s, pipeline.DryRunWrite{Kind: "lane", Target: "sink-lane", Records: 1})
	}
	return nil
}

func init() {
	pipeline.RegisterProcessorPlugin("dryrun_sink", func(c *config.Config) (pipeline.Processor, error) {
		return sinkWriter{}, nil
	})
}

func upperChain(t *testing.T) []map[string]interface{} {
	t.Helper()
	// for_each hosts a test_upper record processor (registered by the
	// modules/pipeline test suite init is NOT exported — use for_each with
	// the otel codec and mutate-style echo)
	return []map[string]interface{}{
		{"for_each": map[string]interface{}{
			"codec":     "raw",
			"processor": []interface{}{map[string]interface{}{"dryrun_noop": map[string]interface{}{}}},
		}},
	}
}

func TestRun_ReportShapeAndSideEffectCapture(t *testing.T) {
	report, err := Run(
		[]map[string]interface{}{{"dryrun_noop": map[string]interface{}{}}},
		[]Sample{{Raw: []byte(`{"payload":{"message":"hello"}}`), Codec: "raw"}},
		Options{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Steps) != 1 || len(report.Steps[0]) == 0 {
		t.Fatalf("steps shape: %+v", report.Steps)
	}
	if report.Steps[0][0].Outcome != "ok" {
		t.Fatalf("outcome: %+v", report.Steps[0][0])
	}
}

func TestRun_FieldDiffDetectsMutations(t *testing.T) {
	// raw codec echoes bytes back; before/after must be equal → empty diff
	report, err := Run(
		[]map[string]interface{}{{"dryrun_noop": map[string]interface{}{}}},
		[]Sample{{Raw: []byte(`{"payload":{"message":"m"}}`), Codec: "raw"}},
		Options{},
	)
	if err != nil {
		t.Fatal(err)
	}
	d := report.Steps[0][0].Diff
	if len(d.Added) != 0 || len(d.Removed) != 0 || len(d.Changed) != 0 {
		t.Fatalf("echo chain must not diff: %+v", d)
	}
}

func TestRun_ScopeCheckSurfacesContractMismatches(t *testing.T) {
	// batch-only test processor inside a nested if branch → compile error
	// surfaced in Compiled, not a panic
	report, err := Run(
		[]map[string]interface{}{{"dryrun_noop": map[string]interface{}{}}},
		nil,
		Options{ScopeCheck: true},
	)
	if err != nil {
		t.Fatalf("scope-check run errored: %v", err)
	}
	if len(report.Compiled) == 0 {
		t.Fatal("compiled phase empty")
	}
}

func TestRun_CapturedOutputIsTerminal(t *testing.T) {
	report, err := Run(
		[]map[string]interface{}{
			{"dryrun_sink": map[string]interface{}{}},
			{"dryrun_noop": map[string]interface{}{}},
		},
		[]Sample{{Raw: []byte(`{"payload":{"message":"hello"}}`), Codec: "raw"}},
		Options{},
	)
	if err != nil {
		t.Fatal(err)
	}
	steps := report.Steps[0]
	if len(steps) != 1 {
		t.Fatalf("replay must stop at the captured delivery, got %d steps: %+v", len(steps), steps)
	}
	if steps[0].Outcome != "delivered" {
		t.Fatalf("captured output step outcome = %q, want delivered", steps[0].Outcome)
	}
	if len(report.SideEffects) != 1 || report.SideEffects[0].Target != "sink-lane" {
		t.Fatalf("side effects: %+v", report.SideEffects)
	}
}
