/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package lane

import (
	"context"
	"testing"
	"time"

	"infini.sh/framework/core/event"
	"infini.sh/framework/core/util"
)

func rec(msg string) *event.Event {
	return &event.Event{Fields: util.MapStr{"message": msg}}
}

func mustCreate(t *testing.T, cfg LaneConfig) Lane {
	t.Helper()
	l, err := Create(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestLane_BlockBackpressure(t *testing.T) {
	l := mustCreate(t, LaneConfig{Name: "bp", Capacity: 2})
	// no subscriber yet: fills up deterministically
	for i := 0; i < 2; i++ {
		if err := l.Emit(rec("m")); err != nil {
			t.Fatal(err)
		}
	}
	if l.TryEmit(rec("overflow")) {
		t.Fatal("TryEmit should fail when full")
	}
	delivered := make(chan struct{}, 4)
	cancel := l.Subscribe(func(r *event.Event) { delivered <- struct{}{} })
	defer cancel()
	// drain one → room appears
	<-delivered
	deadline := time.Now().Add(2 * time.Second)
	for !l.TryEmit(rec("next")) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	<-delivered
	s := l.Snapshot()
	if s.Emitted != 3 || s.Drops != 0 {
		t.Fatalf("stats: %+v", s)
	}
}

func TestLane_DropPolicy(t *testing.T) {
	l := mustCreate(t, LaneConfig{Name: "dp", Capacity: 1, Overflow: "drop"})
	if err := l.Emit(rec("a")); err != nil {
		t.Fatal(err)
	}
	if err := l.Emit(rec("b")); err != nil {
		t.Fatal(err) // drop policy: not an error
	}
	s := l.Snapshot()
	if s.Drops != 1 || s.Depth != 1 {
		t.Fatalf("drop stats: %+v", s)
	}
}

func TestLane_BlockingEmitUnblocksOnDrain(t *testing.T) {
	l := mustCreate(t, LaneConfig{Name: "blk", Capacity: 1})
	if err := l.Emit(rec("first")); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- l.Emit(rec("second")) }()
	select {
	case err := <-done:
		t.Fatalf("second Emit should block: %v", err)
	case <-time.After(80 * time.Millisecond):
	}
	cancel := l.Subscribe(func(r *event.Event) {})
	defer cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Emit never unblocked after drain")
	}
	if s := l.Snapshot(); s.BlockedWaits == 0 {
		t.Fatal("blocked_waits counter not incremented")
	}
}

func TestRegistry_Lifecycle(t *testing.T) {
	if _, err := Create(LaneConfig{Name: "dup"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(LaneConfig{Name: "dup"}); err == nil {
		t.Fatal("duplicate Create must error")
	}
	// GetOrCreate: same instance
	if GetOrCreate(LaneConfig{Name: "dup"}) != Get("dup") {
		t.Fatal("GetOrCreate returned a different instance")
	}
	// dynamic capacity default
	dyn := GetOrCreate(LaneConfig{Name: "dyn", Owner: "stream:x"})
	if dyn.Config().Capacity != dynamicCapacity {
		t.Fatalf("dynamic capacity = %d", dyn.Config().Capacity)
	}
	// delete refuses with depth
	if err := dyn.Emit(rec("in-flight")); err != nil {
		t.Fatal(err)
	}
	if err := Delete("dyn", false); err == nil {
		t.Fatal("Delete must refuse with in-flight records")
	}
	if err := Delete("dyn", true); err != nil {
		t.Fatal(err)
	}
	if Get("dyn") != nil {
		t.Fatal("deleted lane still registered")
	}
	// owner cascade
	GetOrCreate(LaneConfig{Name: "s1", Owner: "stream:y"})
	GetOrCreate(LaneConfig{Name: "s2", Owner: "stream:y"})
	if n := DeleteOwner("stream:y"); n != 2 {
		t.Fatalf("cascade deleted %d, want 2", n)
	}
	_ = Delete("dup", true)
}

func TestLane_CloseSemantics(t *testing.T) {
	l := mustCreate(t, LaneConfig{Name: "cl"})
	if err := l.Emit(rec("x")); err != nil {
		t.Fatal(err)
	}
	got := make(chan *event.Event, 1)
	cancel := l.Subscribe(func(r *event.Event) { got <- r })
	defer cancel()
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	// drain delivers the in-flight record
	select {
	case r := <-got:
		if r == nil {
			t.Fatal("nil record")
		}
	case <-time.After(time.Second):
		t.Fatal("in-flight record lost on close")
	}
	if err := l.Emit(rec("after")); err == nil {
		t.Fatal("Emit after close must error")
	}
}

// EmitAck receipts settle when the subscriber finishes processing the
// record (handler return = settlement).
func TestLane_EmitAckSettlesOnProcessing(t *testing.T) {
	l := mustCreate(t, LaneConfig{Name: "ack", Capacity: 4})
	processed := make(chan struct{})
	release := make(chan struct{})
	cancel := l.Subscribe(func(r *event.Event) {
		<-release // hold processing
		close(processed)
	})
	defer cancel()

	r := l.EmitAck(rec("payload"))
	// receipt must stay unsettled while processing is held
	ctx, cancelW := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelW()
	if err := r.Wait(ctx); err == nil {
		t.Fatal("receipt settled before processing finished")
	}
	close(release)
	<-processed
	ctx2, cancelW2 := context.WithTimeout(context.Background(), time.Second)
	defer cancelW2()
	if err := r.Wait(ctx2); err != nil {
		t.Fatalf("receipt not settled after processing: %v", err)
	}
}

// close with an in-flight acked record settles the receipt with an error
func TestLane_CloseSettlesPendingWithError(t *testing.T) {
	l := mustCreate(t, LaneConfig{Name: "ackcl", Capacity: 1})
	// no subscriber: record stays in flight
	r := l.EmitAck(rec("stuck"))
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancelW := context.WithTimeout(context.Background(), time.Second)
	defer cancelW()
	if err := r.Wait(ctx); err == nil {
		t.Fatal("pending receipt must settle with error on close")
	}
}

// batch mode: receipts settle only after the BATCH completes (not per
// record), and batches flush on size or idle timeout.
func TestLane_SubscribeBatchReceiptSettlesAfterBatch(t *testing.T) {
	l := mustCreate(t, LaneConfig{Name: "batch", Capacity: 8})
	processing := make(chan struct{}, 4)
	release := make(chan struct{})
	cancel := l.SubscribeBatch(2, 50*time.Millisecond, func(recs []*event.Event) {
		processing <- struct{}{}
		<-release
	})
	defer cancel()

	r1 := l.EmitAck(rec("a"))
	r2 := l.EmitAck(rec("b"))
	// batch of 2 formed; processing held — both receipts must be unsettled
	<-processing
	ctx, cancelW := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelW()
	if r1.Wait(ctx) == nil || r2.Wait(ctx) == nil {
		t.Fatal("receipts settled before batch completed")
	}
	close(release)
	ctx2, cancelW2 := context.WithTimeout(context.Background(), time.Second)
	defer cancelW2()
	if r1.Wait(ctx2) != nil || r2.Wait(ctx2) != nil {
		t.Fatal("receipts not settled after batch completed")
	}
}

// idle timeout flush: a single record flushes after flushInterval without
// reaching max.
func TestLane_SubscribeBatchIdleFlush(t *testing.T) {
	l := mustCreate(t, LaneConfig{Name: "idle", Capacity: 4})
	got := make(chan int, 4)
	cancel := l.SubscribeBatch(10, 30*time.Millisecond, func(recs []*event.Event) {
		got <- len(recs)
	})
	defer cancel()
	_ = l.Emit(rec("solo"))
	select {
	case n := <-got:
		if n != 1 {
			t.Fatalf("flushed %d records, want 1", n)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("idle flush never fired")
	}
}
