/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"context"
	"fmt"
	"testing"

	"infini.sh/framework/core/event"
	"infini.sh/framework/core/util"
)

type obsNoopProc struct{ name string }

func (p obsNoopProc) Name() string               { return p.name }
func (p obsNoopProc) Process(ctx *Context) error { return nil }

type obsFailProc struct{ name string }

func (p obsFailProc) Name() string               { return p.name }
func (p obsFailProc) Process(ctx *Context) error { return fmt.Errorf("boom") }

type capture struct{ events []*StepEvent }

func (c *capture) OnStep(e *StepEvent) {
	cp := *e
	c.events = append(c.events, &cp)
}

func newTestContext(t *testing.T) *Context {
	t.Helper()
	cfg := PipelineConfigV2{Name: "observer-test"}
	ctx := AcquireContext(cfg)
	ctx.Context = context.Background()
	ctx.Started()
	return ctx
}

func TestObserver_CapturesBatchStepsInOrder(t *testing.T) {
	chain := &Processors{List: []Processor{obsNoopProc{"a"}, obsNoopProc{"b"}}}
	ctx := newTestContext(t)

	cap := &capture{}
	RegisterObserver(cap)
	defer UnregisterObserver(cap)

	if err := chain.Process(ctx); err != nil {
		t.Fatal(err)
	}
	if len(cap.events) != 2 {
		t.Fatalf("want 2 step events, got %d", len(cap.events))
	}
	for i, e := range cap.events {
		if e.Step != i || e.Pipeline != "observer-test" || e.Processor != string(rune('a'+i)) {
			t.Fatalf("event %d mismatch: %+v", i, e)
		}
		if e.Scope.Kind != ScopeBatch || e.Outcome != OutcomeOK {
			t.Fatalf("event %d scope/outcome: %+v", i, e)
		}
	}
}

func TestObserver_RecordScopeAndTraceID(t *testing.T) {
	chain := &Processors{List: []Processor{obsNoopProc{"r"}}}
	ctx := newTestContext(t)
	rec := &event.Event{Fields: util.MapStr{}}
	SetTraceID(rec, "tr-123")
	ctx.Set(RecordContextKey, rec)

	cap := &capture{}
	RegisterObserver(cap)
	defer UnregisterObserver(cap)

	if err := chain.Process(ctx); err != nil {
		t.Fatal(err)
	}
	if len(cap.events) != 1 {
		t.Fatalf("want 1 event, got %d", len(cap.events))
	}
	e := cap.events[0]
	if e.Scope.Kind != ScopeRecord {
		t.Fatalf("scope = %v, want record", e.Scope)
	}
	if e.TraceID != "tr-123" {
		t.Fatalf("trace id = %q", e.TraceID)
	}
}

func TestObserver_ErrorOutcome(t *testing.T) {
	chain := &Processors{List: []Processor{obsFailProc{"bad"}}}
	ctx := newTestContext(t)

	cap := &capture{}
	RegisterObserver(cap)
	defer UnregisterObserver(cap)

	if err := chain.Process(ctx); err == nil {
		t.Fatal("expected chain error")
	}
	if len(cap.events) != 1 || cap.events[0].Outcome != OutcomeError || cap.events[0].Err != "boom" {
		t.Fatalf("error event mismatch: %+v", cap.events)
	}
}

func TestObserver_UnregisterStopsDelivery(t *testing.T) {
	chain := &Processors{List: []Processor{obsNoopProc{"x"}}}
	ctx := newTestContext(t)
	cap := &capture{}
	RegisterObserver(cap)
	UnregisterObserver(cap)
	if err := chain.Process(ctx); err != nil {
		t.Fatal(err)
	}
	if len(cap.events) != 0 {
		t.Fatalf("unregistered observer still received %d events", len(cap.events))
	}
}

// BenchmarkProcess_NoObservers guards the empty-registry fast path: the
// observer hook must add no allocations when nothing is installed
// (compare against removing the hasObservers guards).
func BenchmarkProcess_NoObservers(b *testing.B) {
	chain := &Processors{List: []Processor{obsNoopProc{"a"}, obsNoopProc{"b"}, obsNoopProc{"c"}}}
	cfg := PipelineConfigV2{Name: "bench"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := AcquireContext(cfg)
		ctx.Context = context.Background()
		ctx.Started()
		_ = chain.Process(ctx)
		ReleaseContext(ctx)
	}
}

// The flow trace must stay bounded on long-lived contexts — an uncapped
// history makes per-step logging cost grow with total steps ever run.
func TestAddFlowProcess_Bounded(t *testing.T) {
	cfg := PipelineConfigV2{Name: "t"}
	ctx := AcquireContext(cfg)
	defer ReleaseContext(ctx)
	for i := 0; i < 10000; i++ {
		ctx.AddFlowProcess("p")
	}
	if got := len(ctx.GetFlowProcess()); got > maxProcessHistory {
		t.Fatalf("history grew to %d, want <= %d", got, maxProcessHistory)
	}
	// most-recent preserved
	h := ctx.GetFlowProcess()
	if len(h) > 0 && h[len(h)-1] != "p" {
		t.Fatal("latest entry lost")
	}
}
