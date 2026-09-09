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

// branchMarker records which if/then/else branch ran: on the current record
// when one is bound, otherwise on the context parameters.
type branchMarker struct {
	Mark string `config:"mark"`
}

func (b *branchMarker) Name() string { return "test_branch_marker" }

func (b *branchMarker) Process(ctx *Context) error {
	if rec, ok := CurrentRecord(ctx); ok {
		if rec.Fields == nil {
			rec.Fields = util.MapStr{}
		}
		rec.Fields["branch"] = b.Mark
		return nil
	}
	ctx.Set("branch", b.Mark)
	return nil
}

func init() {
	RegisterProcessorPlugin("test_branch_marker", func(c *config.Config) (Processor, error) {
		m := &branchMarker{}
		if err := c.Unpack(m); err != nil {
			return nil, err
		}
		return m, nil
	})
}

func buildIfThenElse(t *testing.T, cond util.MapStr) *IfThenElseProcessor {
	t.Helper()
	c, err := config.NewConfigFrom(util.MapStr{
		"if":   cond,
		"then": util.MapStr{"test_branch_marker": util.MapStr{"mark": "then"}},
		"else": util.MapStr{"test_branch_marker": util.MapStr{"mark": "else"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := NewIfElseThenProcessor(c)
	if err != nil {
		t.Fatalf("NewIfElseThenProcessor: %v", err)
	}
	return p
}

func newRecordContext(t *testing.T, fields util.MapStr) *Context {
	ctx := &Context{Context: t.Context()}
	if fields != nil {
		ctx.Set(RecordContextKey, &event.Event{Fields: fields})
	}
	return ctx
}

func branchOf(t *testing.T, ctx *Context) string {
	t.Helper()
	rec, ok := CurrentRecord(ctx)
	assert.True(t, ok, "record should stay bound to the context")
	return util.ToString(rec.Fields["branch"])
}

// Inside a per-record sub-chain the condition must be evaluated against the
// record's own fields, not the pipeline parameters.
func TestIfThenElse_ConditionUsesCurrentRecord_ThenBranch(t *testing.T) {
	p := buildIfThenElse(t, util.MapStr{"equals": util.MapStr{"svc": "api"}})
	ctx := newRecordContext(t, util.MapStr{"svc": "api"})

	if err := p.Process(ctx); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "then", branchOf(t, ctx))
}

func TestIfThenElse_ConditionUsesCurrentRecord_ElseBranch(t *testing.T) {
	p := buildIfThenElse(t, util.MapStr{"equals": util.MapStr{"svc": "api"}})
	ctx := newRecordContext(t, util.MapStr{"svc": "db"})

	if err := p.Process(ctx); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "else", branchOf(t, ctx))
}

// A record missing the condition field goes to else — before the fix the
// condition silently evaluated against an empty value map inside sub-chains,
// so conditional routing never fired at all.
func TestIfThenElse_RecordFieldMissing_GoesElse(t *testing.T) {
	p := buildIfThenElse(t, util.MapStr{"exists": []string{"svc"}})
	ctx := newRecordContext(t, util.MapStr{"other": 1})

	if err := p.Process(ctx); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "else", branchOf(t, ctx))
}

// While a record is bound it takes precedence: the context parameter is not
// consulted even when it carries the same key.
func TestIfThenElse_RecordTakesPrecedenceOverContext(t *testing.T) {
	p := buildIfThenElse(t, util.MapStr{"equals": util.MapStr{"svc": "api"}})
	ctx := &Context{Context: t.Context()}
	ctx.Set("svc", "api") // the context alone would satisfy the condition
	ctx.Set(RecordContextKey, &event.Event{Fields: util.MapStr{"svc": "db"}})

	if err := p.Process(ctx); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "else", branchOf(t, ctx))

	p2 := buildIfThenElse(t, util.MapStr{"equals": util.MapStr{"svc": "db"}})
	if err := p2.Process(ctx); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "then", branchOf(t, ctx))
}

// Pipeline-level usage (no record bound) keeps evaluating against the
// context parameters.
func TestIfThenElse_ContextFallbackWithoutRecord(t *testing.T) {
	p := buildIfThenElse(t, util.MapStr{"equals": util.MapStr{"svc": "api"}})

	ctx := &Context{Context: t.Context()}
	ctx.Set("svc", "api")
	if err := p.Process(ctx); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "then", util.ToString(ctx.Get("branch")))

	ctx2 := &Context{Context: t.Context()}
	ctx2.Set("svc", "db")
	if err := p.Process(ctx2); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "else", util.ToString(ctx2.Get("branch")))
}

// Compound conditions evaluate their leaf matchers against the record too.
func TestIfThenElse_CompoundConditionOnRecord(t *testing.T) {
	cond := util.MapStr{"and": []interface{}{
		util.MapStr{"equals": util.MapStr{"env": "prod"}},
		util.MapStr{"contains": util.MapStr{"file.path": "nginx"}},
	}}

	ctx := newRecordContext(t, util.MapStr{
		"env":  "prod",
		"file": util.MapStr{"path": "/var/log/nginx/access.log"},
	})
	if err := buildIfThenElse(t, cond).Process(ctx); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "then", branchOf(t, ctx))

	ctx2 := newRecordContext(t, util.MapStr{
		"env":  "prod",
		"file": util.MapStr{"path": "/var/log/redis/redis.log"},
	})
	if err := buildIfThenElse(t, cond).Process(ctx2); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "else", branchOf(t, ctx2))
}
