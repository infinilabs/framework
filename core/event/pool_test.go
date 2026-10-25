/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infini.ltd */

package event

import (
	"sync"
	"testing"
	"time"

	"infini.sh/framework/core/util"
)

func TestPoolAcquireReleaseBalance(t *testing.T) {
	a0, r0 := PoolStats()
	e := AcquireEvent()
	e.Fields = util.MapStr{"k": "v"}
	if !ReleaseEvent(e) {
		t.Fatal("releasing a pooled event must report true")
	}
	a1, r1 := PoolStats()
	if a1-a0 != 1 || r1-r0 != 1 {
		t.Fatalf("stats must balance: acquired +%d released +%d", a1-a0, r1-r0)
	}
}

func TestPoolReleaseNonPooledIsNoop(t *testing.T) {
	if ReleaseEvent(&Event{}) {
		t.Fatal("a plain event must not enter the pool")
	}
	if ReleaseEvent(nil) {
		t.Fatal("nil must not enter the pool")
	}
}

func TestPoolDoubleReleaseRejected(t *testing.T) {
	_, r0 := PoolStats()
	e := AcquireEvent()
	if !ReleaseEvent(e) {
		t.Fatal("first release must succeed")
	}
	if ReleaseEvent(e) {
		t.Fatal("second release of the same event must be rejected")
	}
	_, r1 := PoolStats()
	if r1-r0 != 1 {
		t.Fatalf("double release must not inflate the release count: +%d", r1-r0)
	}
}

func TestPoolResetClearsEverything(t *testing.T) {
	e := AcquireEvent()
	e.Fields = util.MapStr{"document": map[string]interface{}{"id": "x"}}
	e.Metadata = EventMetadata{
		Labels:   util.MapStr{"datasource": "d1"},
		Category: "cat", Name: "n", Version: "v", Datatype: "dt",
	}
	e.Meta = util.MapStr{"m": "1"}
	e.Agent = &AgentMeta{AgentID: "a"}
	e.Private = struct{}{}
	e.QueueName = "q"
	e.TimeSeries = true
	e.Timestamp = time.Now()

	e.resetForPool()

	if len(e.Fields) != 0 || len(e.Metadata.Labels) != 0 || len(e.Meta) != 0 {
		t.Fatal("maps must be emptied")
	}
	if e.Agent != nil || e.Private != nil || e.QueueName != "" || e.TimeSeries {
		t.Fatal("scalar/pointer state must be zeroed")
	}
	if !e.Timestamp.IsZero() || e.Metadata.Category != "" || e.Metadata.Name != "" ||
		e.Metadata.Version != "" || e.Metadata.Datatype != "" {
		t.Fatal("metadata and timestamp must be zeroed")
	}
	ReleaseEvent(e)
}

func TestPoolMapRecyclePolicy(t *testing.T) {
	small := util.MapStr{}
	for i := 0; i < maxRecycledEntries; i++ {
		small["k"+string(rune('a'+i%26))+string(rune('0'+i/26))] = i
	}
	if got := recycleMap(small); got == nil || len(got) != 0 {
		t.Fatal("small maps must be cleared and kept")
	}
	big := util.MapStr{}
	for i := 0; i <= maxRecycledEntries; i++ {
		big[string(rune('a'+i%26))+string(rune('0'+i/26))] = i
	}
	if got := recycleMap(big); got != nil {
		t.Fatal("oversized maps must be dropped, not recycled")
	}
}

// TestPoolConcurrentAcquireRelease is the -race gate: concurrent owners
// recycle disjoint events and the counters balance at the end.
func TestPoolConcurrentAcquireRelease(t *testing.T) {
	const workers = 8
	const rounds = 200
	_, r0 := PoolStats()
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < rounds; i++ {
				e := AcquireEvent()
				e.Fields = util.MapStr{"message": "x"}
				ReleaseEvent(e)
			}
		}()
	}
	wg.Wait()
	_, r1 := PoolStats()
	if got := r1 - r0; got != workers*rounds {
		t.Fatalf("every acquire must be released exactly once: released %d of %d", got, workers*rounds)
	}
}
