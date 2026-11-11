package lane_bulk

import (
	"sync"
	"testing"

	"infini.sh/framework/core/event"
	"infini.sh/framework/core/lane"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
)

// Concurrent spill initializes its queue safely before and after warmup.
func TestConcurrentSpillInitialization(t *testing.T) {
	for _, mode := range []string{"cold", "warm"} {
		t.Run(mode, func(t *testing.T) {
			installMemQueue()
			target := t.Name() + "-queue"
			queue.GetOrInitConfig(target)
			l, err := lane.Create(lane.LaneConfig{Name: t.Name(), Capacity: 1, Overflow: "spill:" + target})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := lane.Delete(t.Name(), true); err != nil {
					t.Error(err)
				}
			}()
			rec := func() *event.Event { return &event.Event{Fields: util.MapStr{"message": "data"}} }
			if err := l.Emit(rec()); err != nil {
				t.Fatal(err)
			}
			if mode == "warm" {
				if err := l.Emit(rec()); err != nil {
					t.Fatal(err)
				}
			}
			var wg sync.WaitGroup
			start := make(chan struct{})
			for i := 0; i < 16; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					if err := l.Emit(rec()); err != nil {
						t.Error(err)
					}
				}()
			}
			close(start)
			wg.Wait()
			if n := l.Snapshot().Spills; n < 16 {
				t.Fatalf("spills=%d", n)
			}
		})
	}
}
