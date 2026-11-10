package lane_bulk

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/lane"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/stats"
	"infini.sh/framework/core/util"
)

type pr433Stats struct {
	stats.StatsInterface
	skipped atomic.Int64
}

func (s *pr433Stats) IncrementBy(category, key string, n int64) {
	if category == "bulk_indexing_lane" && key == "skipped" {
		s.skipped.Add(n)
	}
}

// sendBatch must DEAL records across workers — the suffix slice
// recs[w:] overlapped workers and double-sent every record after the
// first (duplicate documents, repeated updates; PR #433 review).
func TestSendBatchDealsRecordsExactlyOnce(t *testing.T) {
	s := &pr433Stats{}
	stats.Register(s)
	p := &BulkIndexingLaneProcessor{config: SenderConfig{WorkerSize: 2}}
	recs := []*event.Event{{}, {}, {}, {}}
	p.sendBatch(&pipeline.Context{Context: context.Background()}, nil, recs)
	// every record lacks a bulk payload, so sendRecord skips (and counts)
	// each one — exactly len(recs) sends must happen
	if n := s.skipped.Load(); n != 4 {
		t.Fatalf("sendRecord called %d times for 4 records with 2 workers", n)
	}
}

// A lone buffered record below bulk size must flush on the idle timeout
// without waiting for the next input (PR #433 review).
func TestInputLaneFlushesOnIdleTimeout(t *testing.T) {
	installMemQueue()
	input, output := t.Name()+"-in", t.Name()+"-out"
	in := lane.GetOrCreate(lane.LaneConfig{Name: input})
	out := lane.GetOrCreate(lane.LaneConfig{Name: output})
	defer lane.Delete(input, true)
	defer lane.Delete(output, true)
	cfg, _ := config.NewConfigFrom(map[string]interface{}{"input_lane": input, "output_lane": output, "index_name": "test", "idle_timeout_in_seconds": 1})
	p, err := NewMergeToBulkLane(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.Process(&pipeline.Context{Context: ctx}) }()
	defer func() { cancel(); <-done }()
	in.Emit(&event.Event{Fields: util.MapStr{"message": "one"}})
	got := make(chan struct{}, 1)
	out.Subscribe(func(*event.Event) { got <- struct{}{} })
	select {
	case <-got:
	case <-time.After(2 * time.Second):
		t.Fatal("single input_lane record did not flush after the 1s idle timeout")
	}
}

// overflow:spill:<queue> must spill to the queue backend instead of
// blocking the producer (PR #433 review).
func TestSpillOverflowPolicyResolvesQueueTarget(t *testing.T) {
	installMemQueue()
	l, err := lane.Create(lane.LaneConfig{Name: t.Name(), Capacity: 1, Overflow: "spill:" + t.Name() + "-queue"})
	if err != nil {
		t.Fatal(err)
	}
	defer lane.Delete(t.Name(), true)
	if err := l.Emit(&event.Event{Fields: util.MapStr{"message": "first"}}); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- l.Emit(&event.Event{Fields: util.MapStr{"message": "overflow"}}) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(100 * time.Millisecond):
		l.Close()
		<-done
		t.Fatal("overflow:spill:<queue> blocked instead of spilling to the queue backend")
	}
}
