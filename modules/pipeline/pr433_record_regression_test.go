package pipeline

import (
	"context"
	"fmt"
	"testing"
	"time"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/lane"
	"infini.sh/framework/core/param"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
)

type pr433FailProcessor struct{}

func (pr433FailProcessor) Name() string                    { return "pr433_fail" }
func (pr433FailProcessor) Process(*pipeline.Context) error { return fmt.Errorf("record failed") }

// on_failure=fail must abort the batch in parallel mode too — the worker
// loop used to only log sub-chain errors (PR #433 review).
func TestForEachParallelOnFailureFailPropagates(t *testing.T) {
	for _, n := range []int{1, 2} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			p := buildForEach(t, map[string]interface{}{"parallel": n, "on_failure": "fail"})
			p.sub.List = []pipeline.Processor{pr433FailProcessor{}}
			c := &pipeline.Context{Context: context.Background()}
			c.Set(param.ParaKey("messages"), []queue.Message{{Data: []byte(`{"payload":{"message":"x"}}`)}})
			if err := p.Process(c); err == nil {
				t.Fatalf("parallel=%d on_failure=fail returned nil", n)
			}
		})
	}
}

// A pooled record emitted into a lane must survive the emitting host's
// pool release: the lane delivers asynchronously, so the pointer crosses
// the boundary as a pool-independent copy (PR #433 review).
func TestForEachPooledRecordSurvivesLaneEmit(t *testing.T) {
	RegisterRecordCodecOrReplace(pooledRawCodec{})
	name := t.Name()
	l := lane.GetOrCreate(lane.LaneConfig{Name: name})
	defer func() {
		if lane.Get(name) != nil {
			_ = lane.Delete(name, true)
		}
	}()
	p := buildForEach(t, map[string]interface{}{"codec": "raw_pooled"})
	cfg, _ := config.NewConfigFrom(map[string]interface{}{"lane": name, "ack": true})
	out, err := NewOutputProcessor(cfg)
	if err != nil {
		t.Fatal(err)
	}
	p.sub.List = []pipeline.Processor{out}
	c := &pipeline.Context{Context: context.Background()}
	c.Set(param.ParaKey("messages"), []queue.Message{{Data: []byte("original")}})
	if err := p.Process(c); err != nil {
		t.Fatal(err)
	}
	got := make(chan string, 1)
	l.Subscribe(func(e *event.Event) { got <- fmt.Sprint(e.Fields["message"]) })
	select {
	case s := <-got:
		if s != "original" {
			t.Fatalf("lane got %q after for_each returned; pooled record already reset", s)
		}
	case <-time.After(time.Second):
		t.Fatal("no lane delivery")
	}
}

// A restarted lane_consumer must re-bind the live lane: the task's
// _start/_resume reuses the compiled processor, and the previous run's
// exit deleted the old instance (PR #433 review).
func TestLaneConsumerProcessRebindsFreshLane(t *testing.T) {
	name := t.Name()
	defer func() {
		if lane.Get(name) != nil {
			_ = lane.Delete(name, true)
		}
	}()
	cfg, _ := config.NewConfigFrom(map[string]interface{}{"lane": name, "batch_max": 1, "processor": []interface{}{map[string]interface{}{"test_upper": map[string]interface{}{}}}})
	proc, err := NewLaneConsumerProcessor(cfg)
	if err != nil {
		t.Fatal(err)
	}
	p := proc.(*LaneConsumerProcessor)
	first, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.Process(&pipeline.Context{Context: first}) }()
	cancel()
	<-done
	fresh := lane.GetOrCreate(lane.LaneConfig{Name: name})
	second, cancel2 := context.WithCancel(context.Background())
	go func() { done <- p.Process(&pipeline.Context{Context: second}) }()
	defer func() { cancel2(); <-done }()
	rec := fresh.EmitAck(&event.Event{})
	wait, cancelW := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancelW()
	if err := rec.Wait(wait); err != nil {
		t.Fatalf("restarted consumer did not consume the fresh lane: %v", err)
	}
}
