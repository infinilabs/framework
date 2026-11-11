package lane

import (
	"context"
	"errors"
	"testing"
	"time"

	"infini.sh/framework/core/event"
)

// Batch receipts retain both successful and failed processing results.
func TestBatchReceiptsPreserveProcessingResult(t *testing.T) {
	for _, failure := range []bool{false, true} {
		name := "success"
		var result error
		if failure {
			name, result = "failure", errors.New("durable output failed")
		}
		t.Run(name, func(t *testing.T) {
			l := mustCreate(t, LaneConfig{Name: t.Name()})
			defer func() {
				if err := Delete(t.Name(), true); err != nil {
					t.Error(err)
				}
			}()
			r := l.EmitAck(&event.Event{})
			cancel := l.SubscribeBatchWithError(1, 0, func([]*event.Event) error { return result })
			defer cancel()
			ctx, stop := context.WithTimeout(context.Background(), time.Second)
			defer stop()
			if err := r.Wait(ctx); !errors.Is(err, result) {
				t.Fatalf("receipt result = %v, want %v", err, result)
			}
		})
	}
}

// A name-only producer restores the config of a closed lane.
func TestClosedLaneKeepsConfigForNameOnlyProducer(t *testing.T) {
	cfg := LaneConfig{Name: t.Name(), Namespace: "document", Capacity: 7, Overflow: "drop"}
	old := mustCreate(t, cfg)
	defer func() {
		if err := Delete(t.Name(), true); err != nil {
			t.Error(err)
		}
	}()
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}
	fresh := GetOrCreate(LaneConfig{Name: cfg.Name})
	if fresh == old || fresh.Config() != cfg {
		t.Fatalf("restart changed config: %+v", fresh.Config())
	}
}

// Cancel waits for an active callback and remains safe to call again.
func TestSubscriberCancelWaitsForCallback(t *testing.T) {
	l := mustCreate(t, LaneConfig{Name: t.Name()})
	defer func() {
		if err := Delete(t.Name(), true); err != nil {
			t.Error(err)
		}
	}()
	started := make(chan struct{})
	release := make(chan struct{})
	cancel := l.SubscribeBatchWithError(1, 0, func([]*event.Event) error {
		close(started)
		<-release
		return nil
	})
	if err := l.Emit(&event.Event{}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		close(release)
		cancel()
		t.Fatal("subscriber did not start its callback")
	}
	returned := make(chan struct{})
	go func() {
		cancel()
		close(returned)
	}()
	select {
	case <-returned:
		close(release)
		t.Fatal("cancel returned while the callback was active")
	case <-time.After(10 * time.Millisecond):
	}
	close(release)
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("cancel did not return after the callback finished")
	}
	cancel()
}
