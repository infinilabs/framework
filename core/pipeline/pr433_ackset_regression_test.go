package pipeline_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"infini.sh/framework/core/event"
	"infini.sh/framework/core/lane"
	"infini.sh/framework/core/pipeline"
)

// The caller's ack timeout must bound the wait over EXTERNAL receipts
// too (lane receipts): deriving the deadline only after the external
// loop let a stuck downstream outlive ack_timeout_ms forever
// (regression: PR #433 review).
func TestAckSetWaitTimeoutBoundsExternalReceipts(t *testing.T) {
	l := lane.GetOrCreate(lane.LaneConfig{Name: t.Name()})
	defer lane.Delete(t.Name(), true)
	c := &pipeline.Context{Context: context.Background()}
	s := pipeline.AttachAckSet(c)
	s.AddReceipt(l.EmitAck(&event.Event{})) // no subscriber: never settles
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	err := s.Wait(ctx, 10*time.Millisecond)
	if !errors.Is(err, pipeline.ErrAckTimeout) {
		t.Fatalf("10ms ack timeout ignored: got %v after outer 150ms deadline", err)
	}
}

// Pending() must only PEEK at settle signals: consuming them ate
// settlement results, hid lane-close errors from the commit gate and
// hung a later Wait on an already-settled receipt (PR #433 review).
func TestAckSetPendingDoesNotConsumeSettlement(t *testing.T) {
	l := lane.GetOrCreate(lane.LaneConfig{Name: t.Name()})
	defer lane.Delete(t.Name(), true)
	s := pipeline.AttachAckSet(&pipeline.Context{Context: context.Background()})
	s.AddReceipt(l.EmitAck(&event.Event{}))
	l.Close() // pending receipt settles with an error
	if n := s.Pending(); n != 0 {
		t.Fatalf("closed lane still reports %d pending receipts", n)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := s.Wait(ctx, time.Second); errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("Pending consumed the settlement; Wait blocked until the context deadline")
	}
}

// Mixed settlements: a successful receipt peeked at by Pending() must
// still be readable by Wait once everything settles.
func TestAckSetWaitMixedSettlementsCompletes(t *testing.T) {
	l := lane.GetOrCreate(lane.LaneConfig{Name: t.Name()})
	defer lane.Delete(t.Name(), true)
	release := make(chan struct{})
	secondEntered := make(chan struct{})
	first, second := &event.Event{}, &event.Event{}
	l.Subscribe(func(e *event.Event) {
		if e == second {
			close(secondEntered)
			<-release
		}
	})
	s := pipeline.AttachAckSet(&pipeline.Context{Context: context.Background()})
	s.AddReceipt(l.EmitAck(first))
	s.AddReceipt(l.EmitAck(second))
	<-secondEntered // first handler and its receipt settlement completed
	if n := s.Pending(); n != 1 {
		close(release)
		t.Fatalf("want one unsettled receipt, got %d", n)
	}
	close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := s.Wait(ctx, time.Second); err != nil {
		t.Fatalf("Wait did not observe the settlement consumed by Pending: %v", err)
	}
}

// A receipt settled WITH AN ERROR must fail the gate: the old Wait
// returned nil once nothing was pending, so a failed lane delivery
// silently committed its offset (PR #433 review).
func TestAckSetWaitSurfacesSettlementError(t *testing.T) {
	l := lane.GetOrCreate(lane.LaneConfig{Name: t.Name()})
	defer lane.Delete(t.Name(), true)
	s := pipeline.AttachAckSet(&pipeline.Context{Context: context.Background()})
	s.AddReceipt(l.EmitAck(&event.Event{}))
	l.Close()
	// the fixed consumer commit gate: Wait runs even when Pending()==0
	if werr := s.Wait(context.Background(), time.Second); werr == nil {
		t.Fatal("commit gate accepted a failed delivery: settlement error not surfaced")
	}
}
