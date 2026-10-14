/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"testing"
	"time"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/lane"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/util"
)

func buildOutput(t *testing.T, cfgMap util.MapStr) pipeline.Processor {
	t.Helper()
	c, err := config.NewConfigFrom(cfgMap)
	if err != nil {
		t.Fatal(err)
	}
	p, err := NewOutputProcessor(c)
	if err != nil {
		t.Fatalf("NewOutputProcessor: %v", err)
	}
	return p
}

func outCtx(t *testing.T, rec *event.Event) *pipeline.Context {
	t.Helper()
	cfg := pipeline.PipelineConfigV2{Name: "out-test"}
	ctx := pipeline.AcquireContext(cfg)
	ctx.Context = t.Context()
	ctx.Started()
	if rec != nil {
		ctx.Set(pipeline.RecordContextKey, rec)
	}
	return ctx
}

// one-of target validation: zero or multiple targets reject at unpack
func TestOutput_OneOfTargetValidation(t *testing.T) {
	if _, err := NewOutputProcessor(mustCfg(t, util.MapStr{})); err == nil {
		t.Fatal("zero targets must reject")
	}
	if _, err := NewOutputProcessor(mustCfg(t, util.MapStr{"queue": map[string]interface{}{"name": "q"}, "lane": "l"})); err == nil {
		t.Fatal("two targets must reject")
	}
}

func mustCfg(t *testing.T, m util.MapStr) *config.Config {
	t.Helper()
	c, err := config.NewConfigFrom(m)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// lane target in record scope emits; ack attaches to the context AckSet
func TestOutput_LaneTargetRecordScope(t *testing.T) {
	ln, err := lane.Create(lane.LaneConfig{Name: "out-lane", Capacity: 4})
	if err != nil {
		t.Fatal(err)
	}
	defer lane.Delete("out-lane", true)

	p := buildOutput(t, util.MapStr{"lane": "out-lane", "ack": true})
	rec, _ := GetRecordCodec("").Decode([]byte(`{"payload":{"message":"m"}}`))
	ctx := outCtx(t, rec)

	if err := p.Process(ctx); err != nil {
		t.Fatal(err)
	}
	set := pipeline.AckSetOf(ctx)
	if set == nil || set.Pending() != 1 {
		t.Fatalf("ack receipt not attached: %v", set)
	}
	// settle via a batch subscriber and confirm the gate drains
	got := make(chan int, 1)
	cancel := ln.SubscribeBatch(1, 10*time.Millisecond, func(recs []*event.Event) { got <- len(recs) })
	defer cancel()
	deadline := time.After(2 * time.Second)
	for set.Pending() > 0 {
		select {
		case <-deadline:
			t.Fatal("receipt never settled")
		case <-got:
		case <-time.After(time.Millisecond):
		}
	}
}

// lane target in batch scope errors with the bridge pointer
func TestOutput_LaneTargetBatchScopeRejected(t *testing.T) {
	_, err := lane.Create(lane.LaneConfig{Name: "out-lane2", Capacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer lane.Delete("out-lane2", true)
	p := buildOutput(t, util.MapStr{"lane": "out-lane2"})
	ctx := outCtx(t, nil) // no current record = batch scope
	if err := p.Process(ctx); err == nil {
		t.Fatal("lane target in batch scope must error")
	}
}

// queue-target construction requires the disk-queue subsystem (kv store);
// the unpack-level one-of validation is covered above, byte-push semantics
// ride the consumer integration tests.
