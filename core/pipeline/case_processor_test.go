/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/util"
)

// buildCase compiles `body` as the content of a case step.
func buildCase(t *testing.T, body util.MapStr) *CaseProcessor {
	t.Helper()
	c, err := config.NewConfigFrom(util.MapStr{"case": body})
	if err != nil {
		t.Fatal(err)
	}
	p, err := NewCaseProcessor(c)
	if err != nil {
		t.Fatalf("NewCaseProcessor: %v", err)
	}
	return p
}

func mark(mark string) util.MapStr {
	return util.MapStr{"test_branch_marker": util.MapStr{"mark": mark}}
}

func levelSwitch(withDefault bool) util.MapStr {
	body := util.MapStr{
		"field": "level",
		"branches": []interface{}{
			util.MapStr{"values": []interface{}{"error", "fatal"}, "then": mark("sev1")},
			util.MapStr{"values": []interface{}{"warn"}, "then": mark("sev2")},
		},
	}
	if withDefault {
		body["default"] = mark("sev3")
	}
	return body
}

// Enumerated switch on a record field: each value set routes to its arm.
func TestCaseSwitchEnumRouting(t *testing.T) {
	p := buildCase(t, levelSwitch(true))

	for _, tc := range []struct {
		level string
		want  string
	}{
		{"error", "sev1"},
		{"fatal", "sev1"},
		{"warn", "sev2"},
		{"debug", "sev3"}, // no arm matches -> default
	} {
		ctx := newRecordContext(t, util.MapStr{"level": tc.level})
		if err := p.Process(ctx); err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, tc.want, branchOf(t, ctx), "level=%s", tc.level)
	}
}

// No match and no default: the case block is a no-op, nothing crashed.
func TestCaseNoMatchWithoutDefault(t *testing.T) {
	p := buildCase(t, levelSwitch(false))
	ctx := newRecordContext(t, util.MapStr{"level": "debug"})

	if err := p.Process(ctx); err != nil {
		t.Fatal(err)
	}
	rec, ok := CurrentRecord(ctx)
	assert.True(t, ok)
	assert.NotContains(t, rec.Fields, "branch")
	assert.Contains(t, ctx.GetFlowProcess(), "case:no-match")
}

// A record missing the switch field never matches a values arm.
func TestCaseFieldMissingFallsThrough(t *testing.T) {
	p := buildCase(t, levelSwitch(true))
	ctx := newRecordContext(t, util.MapStr{"other": 1})

	if err := p.Process(ctx); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "sev3", branchOf(t, ctx))
}

// Arms accept full conditions (`when`) with any operator the if/then/else
// block supports; evaluated against the current record inside sub-chains.
func TestCaseWhenArmsOnRecord(t *testing.T) {
	p := buildCase(t, util.MapStr{
		"branches": []interface{}{
			util.MapStr{"when": util.MapStr{"equals": util.MapStr{"svc": "api"}}, "then": mark("api")},
			util.MapStr{"when": util.MapStr{"prefix": util.MapStr{"svc": "db"}}, "then": mark("db")},
		},
		"default": mark("other"),
	})

	api := newRecordContext(t, util.MapStr{"svc": "api"})
	if err := p.Process(api); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "api", branchOf(t, api))

	db := newRecordContext(t, util.MapStr{"svc": "db-replica"})
	if err := p.Process(db); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "db", branchOf(t, db))

	other := newRecordContext(t, util.MapStr{"svc": "cache"})
	if err := p.Process(other); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "other", branchOf(t, other))
}

// First match wins even when several arms match.
func TestCaseFirstMatchWins(t *testing.T) {
	p := buildCase(t, util.MapStr{
		"field": "level",
		"branches": []interface{}{
			util.MapStr{"values": []interface{}{"warn"}, "then": mark("first")},
			util.MapStr{"values": []interface{}{"warn", "info"}, "then": mark("second")},
		},
	})
	ctx := newRecordContext(t, util.MapStr{"level": "warn"})
	if err := p.Process(ctx); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "first", branchOf(t, ctx))
}

// values + when on the same arm AND together; values alone on another arm
// still matches the same record — the mixed-style ladder stays coherent.
func TestCaseValuesAndWhenCombineAsAnd(t *testing.T) {
	p := buildCase(t, util.MapStr{
		"field": "level",
		"branches": []interface{}{
			util.MapStr{
				"values": []interface{}{"error"},
				"when":   util.MapStr{"exists": []interface{}{"svc"}},
				"then":   mark("both"),
			},
			util.MapStr{"values": []interface{}{"error"}, "then": mark("values-only")},
		},
	})

	withSvc := newRecordContext(t, util.MapStr{"level": "error", "svc": "api"})
	if err := p.Process(withSvc); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "both", branchOf(t, withSvc))

	withoutSvc := newRecordContext(t, util.MapStr{"level": "error"})
	if err := p.Process(withoutSvc); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "values-only", branchOf(t, withoutSvc))
}

// Pipeline-level usage (no record bound) evaluates arms against the
// context parameters, mirroring if/then/else.
func TestCaseContextFallbackWithoutRecord(t *testing.T) {
	p := buildCase(t, util.MapStr{
		"field": "level",
		"branches": []interface{}{
			util.MapStr{"values": []interface{}{"error"}, "then": mark("err")},
		},
		"default": mark("rest"),
	})

	ctx := &Context{Context: t.Context()}
	ctx.Set("level", "error")
	if err := p.Process(ctx); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "err", util.ToString(ctx.Get("branch")))

	ctx2 := &Context{Context: t.Context()}
	ctx2.Set("level", "info")
	if err := p.Process(ctx2); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "rest", util.ToString(ctx2.Get("branch")))
}

// A bound record takes precedence over context parameters carrying the
// same key (same rule the if/then/else block applies).
func TestCaseRecordTakesPrecedenceOverContext(t *testing.T) {
	p := buildCase(t, util.MapStr{
		"field": "level",
		"branches": []interface{}{
			util.MapStr{"values": []interface{}{"error"}, "then": mark("err")},
		},
	})
	ctx := &Context{Context: t.Context()}
	ctx.Set("level", "error")
	ctx.Set(RecordContextKey, &event.Event{Fields: util.MapStr{"level": "info"}})

	if err := p.Process(ctx); err != nil {
		t.Fatal(err)
	}
	assert.Contains(t, ctx.GetFlowProcess(), "case:no-match")
}

// The step compiles through the generic chain compiler (the same path
// ValidatePipelineConfig takes) and records its routing in the flow trace.
func TestCaseStepCompilesViaNewPipeline(t *testing.T) {
	step := util.MapStr{"case": util.MapStr{
		"field": "level",
		"branches": []interface{}{
			util.MapStr{"values": []interface{}{"warn"}, "then": []interface{}{mark("sev2")}},
		},
	}}
	c, err := config.NewConfigFrom(step)
	if err != nil {
		t.Fatal(err)
	}
	procs, err := NewPipeline([]*config.Config{c})
	if err != nil {
		t.Fatalf("NewPipeline: %v", err)
	}
	assert.Equal(t, 1, len(procs.List))
	assert.Equal(t, "case", procs.List[0].Name())

	ctx := newRecordContext(t, util.MapStr{"level": "warn"})
	if err := procs.Process(ctx); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "sev2", branchOf(t, ctx))
	assert.Contains(t, ctx.GetFlowProcess(), "case:1")
	assert.Contains(t, ctx.GetFlowProcess(), "test_branch_marker")
}

func TestCaseCompileErrors(t *testing.T) {
	mk := func(body util.MapStr) *config.Config {
		c, err := config.NewConfigFrom(util.MapStr{"case": body})
		if err != nil {
			t.Fatal(err)
		}
		return c
	}

	for name, body := range map[string]util.MapStr{
		"no branches": {
			"field": "level",
		},
		"arm without values or when": {
			"field":    "level",
			"branches": []interface{}{util.MapStr{"then": mark("x")}},
		},
		"values without field": {
			"branches": []interface{}{
				util.MapStr{"values": []interface{}{"error"}, "then": mark("x")},
			},
		},
		"arm without then": {
			"field":    "level",
			"branches": []interface{}{util.MapStr{"values": []interface{}{"error"}}},
		},
		"unknown condition operator": {
			"branches": []interface{}{
				util.MapStr{"when": util.MapStr{"nope": util.MapStr{"x": 1}}, "then": mark("x")},
			},
		},
	} {
		_, err := NewCaseProcessor(mk(body))
		assert.Error(t, err, name)
	}
}

// Arm chains inherit the enclosing scope: inside a record host (strict
// scope) a batch_only processor nested in an arm is a compile error, the
// same placement rule if/then/else branches follow.
func TestCaseArmInheritsRecordScope(t *testing.T) {
	step := util.MapStr{"case": util.MapStr{
		"branches": []interface{}{
			util.MapStr{"when": util.MapStr{"equals": util.MapStr{"x": 1}}, "then": []interface{}{
				util.MapStr{"test_batch_only": util.MapStr{}},
			}},
		},
	}}
	c, err := config.NewConfigFrom([]util.MapStr{step})
	if err != nil {
		t.Fatal(err)
	}
	var list []*config.Config
	if err := c.Unpack(&list); err != nil {
		t.Fatal(err)
	}
	_, err = NewPipelineWithScope(list,
		ScopeInfo{Kind: ScopeRecord, Strict: true}, true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "batch_only")
}
