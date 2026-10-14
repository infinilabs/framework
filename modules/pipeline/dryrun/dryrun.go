/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

// Package dryrun is the unified dry-run engine: replay sample payloads
// through a chain with per-step before/after snapshots, field diffs and
// side-effect capture (queue/lane writes are captured, never delivered).
package dryrun

import (
	"context"
	"fmt"
	"time"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/util"
)

// Sample is one replay input: raw payload bytes plus the codec that
// decodes them ("" = default).
type Sample struct {
	Raw   []byte `json:"raw"`
	Codec string `json:"codec"`
}

// Options controls a run.
type Options struct {
	// ScopeCheck compiles the chain with contract validation (mismatches
	// surface in Report.Compiled instead of running).
	ScopeCheck bool `json:"scope_check"`
	// SideEffects: "capture" (default — writes collected in the report) |
	// "passthrough" (real writes; explicit opt-in).
	SideEffects string `json:"side_effects"`
	// LLMPolicy: "real" (default) | "mock" (recorded replay; host-app
	// supplied via the mock registry).
	LLMPolicy string `json:"llm_policy"`
}

// FieldDiff is the added/removed/changed path set between two snapshots.
type FieldDiff struct {
	Added   util.MapStr `json:"added,omitempty"`
	Removed util.MapStr `json:"removed,omitempty"`
	Changed util.MapStr `json:"changed,omitempty"`
}

// StepResult is one processor's effect on one sample.
type StepResult struct {
	Step       int         `json:"step"`
	Processor  string      `json:"processor"`
	Before     util.MapStr `json:"before,omitempty"`
	After      util.MapStr `json:"after,omitempty"`
	Diff       FieldDiff   `json:"diff"`
	DurationMs int64       `json:"duration_ms"`
	Outcome    string      `json:"outcome"` // ok | error | dropped
	Err        string      `json:"err,omitempty"`
}

// StepDesc is the compile-phase outcome per entry.
type StepDesc struct {
	Step      int    `json:"step"`
	Processor string `json:"processor"`
	OK        bool   `json:"ok"`
	Note      string `json:"note,omitempty"`
}

// CapturedWrite is a side effect intercepted during the run.
type CapturedWrite = pipeline.DryRunWrite

// Report is the full dry-run result.
type Report struct {
	Compiled    []StepDesc      `json:"compiled"`
	Steps       [][]StepResult  `json:"steps"` // per sample × per step
	SideEffects []CapturedWrite `json:"side_effects"`
}

// Run replays samples through the chain. The chain runs in record scope
// per sample (the record convention); batch processors receive the
// single-record slice.
func Run(chain []map[string]interface{}, samples []Sample, opts Options) (*Report, error) {
	if opts.SideEffects == "" {
		opts.SideEffects = "capture"
	}
	cfgs, err := toConfigs(chain)
	if err != nil {
		return nil, err
	}

	report := &Report{}

	// compile phase
	scope := pipeline.ScopeInfo{Kind: pipeline.ScopeRecord, Strict: opts.ScopeCheck}
	procs, compileErr := pipeline.NewPipelineWithScope(cfgs, scope, true)
	for i, m := range chain {
		desc := StepDesc{Step: i, Processor: firstKey(m), OK: true}
		report.Compiled = append(report.Compiled, desc)
	}
	if compileErr != nil {
		// surface the compile error on the failing entry; when scope-check
		// is on the error is the deliverable (not a run failure)
		if n := len(report.Compiled); n > 0 {
			report.Compiled[n-1].OK = false
			report.Compiled[n-1].Note = compileErr.Error()
		} else {
			report.Compiled = append(report.Compiled, StepDesc{
				Step: 0, Processor: "<chain>", OK: false, Note: compileErr.Error(),
			})
		}
		if opts.ScopeCheck {
			return report, nil
		}
		return report, compileErr
	}

	// replay phase
	sink := &[]pipeline.DryRunWrite{}
	for _, s := range samples {
		results := replaySample(procs, s, opts, sink)
		report.Steps = append(report.Steps, results)
	}
	report.SideEffects = *sink
	return report, nil
}

func replaySample(procs *pipeline.Processors, s Sample, opts Options, sink *[]CapturedWrite) []StepResult {
	codec := pipeline.GetRecordCodec(s.Codec)
	rec, err := codec.Decode(s.Raw)
	if err != nil {
		return []StepResult{{Step: 0, Processor: "codec:" + s.Codec, Outcome: "error", Err: err.Error()}}
	}

	ctxCfg := pipeline.PipelineConfigV2{Name: "dry-run"}
	ctx := pipeline.AcquireContext(ctxCfg)
	ctx.Context = context.Background()
	ctx.Started()
	defer pipeline.ReleaseContext(ctx)
	ctx.Set(pipeline.RecordContextKey, rec)
	pipeline.EnableDryRun(ctx, sink, opts.SideEffects == "capture")

	var results []StepResult
	for i, p := range procs.List {
		before := snapshotFields(rec)
		start := time.Now()
		outcome, errMsg := "ok", ""
		var runErr error
		if bp, isBatch := p.(pipeline.BatchProcessor); isBatch {
			runErr = bp.ProcessBatch(ctx, []*event.Event{rec})
		} else {
			runErr = p.Process(ctx)
		}
		if runErr != nil {
			outcome, errMsg = "error", runErr.Error()
		} else if pipeline.IsDropped(rec) {
			outcome = "dropped"
		}
		after := snapshotFields(rec)
		results = append(results, StepResult{
			Step: i, Processor: p.Name(),
			Before: before, After: after,
			Diff:       diffMaps(before, after),
			DurationMs: time.Since(start).Milliseconds(),
			Outcome:    outcome, Err: errMsg,
		})
		if runErr != nil {
			break
		}
	}
	return results
}

func snapshotFields(rec *event.Event) util.MapStr {
	if rec == nil || rec.Fields == nil {
		return util.MapStr{}
	}
	b, err := util.ToJSONBytes(rec.Fields)
	if err != nil {
		return util.MapStr{}
	}
	m := util.MapStr{}
	_ = util.FromJSONBytes(b, &m)
	return m
}

// diffMaps walks two flat snapshots; nested maps recurse with dotted paths.
func diffMaps(before, after util.MapStr) FieldDiff {
	d := FieldDiff{Added: util.MapStr{}, Removed: util.MapStr{}, Changed: util.MapStr{}}
	seenAfter := map[string]bool{}
	for k, av := range after {
		seenAfter[k] = true
		bv, ok := before[k]
		if !ok {
			d.Added[k] = av
			continue
		}
		aSub, aIsMap := av.(map[string]interface{})
		bSub, bIsMap := bv.(map[string]interface{})
		if aIsMap && bIsMap {
			sub := diffMaps(util.MapStr(bSub), util.MapStr(aSub))
			for sk, sv := range sub.Added {
				d.Added[k+"."+sk] = sv
			}
			for sk, sv := range sub.Removed {
				d.Removed[k+"."+sk] = sv
			}
			for sk, sv := range sub.Changed {
				d.Changed[k+"."+sk] = sv
			}
			continue
		}
		if fmt.Sprint(av) != fmt.Sprint(bv) {
			d.Changed[k] = util.MapStr{"from": bv, "to": av}
		}
	}
	for k, bv := range before {
		if !seenAfter[k] {
			d.Removed[k] = bv
		}
	}
	return d
}

func toConfigs(chain []map[string]interface{}) ([]*config.Config, error) {
	c, err := config.NewConfigFrom(chain)
	if err != nil {
		return nil, err
	}
	var list []*config.Config
	if err := c.Unpack(&list); err != nil {
		return nil, err
	}
	return list, nil
}

func firstKey(m map[string]interface{}) string {
	for k := range m {
		return k
	}
	return "?"
}
