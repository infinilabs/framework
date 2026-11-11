package consumer

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/lane"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
	modpipeline "infini.sh/framework/modules/pipeline"
)

type ackTestKV struct {
	kv.KVStore
	data map[string][]byte
}

func (k *ackTestKV) ExistsKey(b string, key []byte) (bool, error) {
	_, ok := k.data[b+string(key)]
	return ok, nil
}
func (k *ackTestKV) GetValue(b string, key []byte) ([]byte, error) {
	return k.data[b+string(key)], nil
}
func (k *ackTestKV) AddValue(b string, key, v []byte) error {
	k.data[b+string(key)] = v
	return nil
}

type ackTestQueue struct {
	queue.AdvancedQueueAPI
	committed []queue.Offset
	fetched   bool
	pushes    int
	pushErr   error
}

func (q *ackTestQueue) GetOffset(*queue.QueueConfig, *queue.ConsumerConfig) (queue.Offset, error) {
	return queue.NewOffset(0, 0), nil
}
func (q *ackTestQueue) CommitOffset(_ *queue.QueueConfig, _ *queue.ConsumerConfig, o queue.Offset) (bool, error) {
	q.committed = append(q.committed, o)
	return true, nil
}
func (q *ackTestQueue) AcquireConsumer(*queue.QueueConfig, *queue.ConsumerConfig) (queue.ConsumerAPI, error) {
	return &ackTestReader{q: q}, nil
}
func (q *ackTestQueue) ReleaseConsumer(*queue.QueueConfig, *queue.ConsumerConfig, queue.ConsumerAPI) error {
	return nil
}

type ackTestReader struct {
	queue.ConsumerAPI
	q *ackTestQueue
}

func (r *ackTestReader) FetchMessages(c *queue.Context, n int) ([]queue.Message, bool, error) {
	if r.q.fetched {
		return nil, true, nil
	}
	r.q.fetched = true
	c.NextOffset = queue.NewOffset(0, 2)
	return []queue.Message{{Data: []byte(`{"payload":{"message":"one"}}`)}, {Data: []byte(`{"payload":{"message":"two"}}`)}}, false, nil
}

// Both serial and parallel record execution wait before committing offsets.
func TestConsumerWaitsForParallelLaneReceipts(t *testing.T) {
	for _, tc := range []struct {
		parallel int
		settled  bool
	}{{1, false}, {2, false}, {2, true}} {
		t.Run(fmt.Sprintf("parallel_%d/settled_%t", tc.parallel, tc.settled), func(t *testing.T) {
			kv.Register(t.Name(), &ackTestKV{data: map[string][]byte{}})
			q := &ackTestQueue{}
			queue.RegisterDefaultHandler(q)
			name := t.Name()
			l := lane.GetOrCreate(lane.LaneConfig{Name: name})
			defer func() {
				if err := lane.Delete(name, true); err != nil {
					t.Error(err)
				}
			}()
			if tc.settled {
				cancel := l.Subscribe(func(*event.Event) {})
				defer cancel()
			}
			cfg, err := config.NewConfigFrom(map[string]interface{}{"parallel": tc.parallel, "processor": []interface{}{map[string]interface{}{"output": map[string]interface{}{"lane": name, "ack": true}}}})
			if err != nil {
				t.Fatal(err)
			}
			fe, err := modpipeline.NewForEachProcessor(cfg)
			if err != nil {
				t.Fatal(err)
			}
			ackTimeout := int64(10)
			if tc.settled {
				ackTimeout = 1000
			}
			p := &QueueConsumerProcessor{config: &Config{NumOfSlices: 1, MessageField: "messages", QueueField: "queue", Consumer: &queue.ConsumerConfig{Group: name, Name: name}, AutoCommitOffset: true, QuitOnEOFQueue: true, AckTimeoutInMs: ackTimeout}, processors: &pipeline.Processors{List: []pipeline.Processor{fe}}}
			ctx := &pipeline.Context{}
			ctx.ResetContext()
			p.wg.Add(1)
			p.NewSlicedWorker(ctx, &queue.QueueConfig{ID: name, Name: name}, "worker", 0, 1, (*pipeline.Context)(nil))
			t.Logf("parallel=%d, lane depth=%d, committed offsets=%v, failed=%v", tc.parallel, l.Depth(), q.committed, ctx.IsFailed())
			if tc.settled {
				if len(q.committed) != 1 || ctx.IsFailed() {
					t.Fatalf("settled receipts must allow commit: offsets=%v failed=%v", q.committed, ctx.IsFailed())
				}
				return
			}
			if l.Depth() != 2 {
				t.Fatal("expected both receipts remain unprocessed")
			}
			if len(q.committed) != 0 || !ctx.IsFailed() {
				t.Fatalf("unsettled lane receipts must fail the batch without commit: offsets=%v failed=%v", q.committed, ctx.IsFailed())
			}
		})
	}
}

func (q *ackTestQueue) Push(string, []byte) error {
	q.pushes++
	return q.pushErr
}
func (q *ackTestQueue) Init(string) error { return nil }

type ackTestNoop struct{}

func (ackTestNoop) Name() string                    { return "test_ack_noop" }
func (ackTestNoop) Process(*pipeline.Context) error { return nil }

type ackFailureProcessor struct{}

func (ackFailureProcessor) Name() string                    { return "test_ack_failure" }
func (ackFailureProcessor) Process(*pipeline.Context) error { return errors.New("processing failed") }

type ackTestBadCodec struct{}

func (ackTestBadCodec) Name() string                        { return "test_ack_bad" }
func (ackTestBadCodec) Namespace() string                   { return "" }
func (ackTestBadCodec) Decode([]byte) (*event.Event, error) { return nil, errors.New("unused") }
func (ackTestBadCodec) Encode(*event.Event) ([]byte, error) {
	return nil, errors.New("encoder rejected record")
}

// Durable output permits commit only after encoding and queue writes succeed.
func TestConsumerKeepsOffsetOnBridgeFailure(t *testing.T) {
	pipeline.RegisterProcessorPlugin("test_ack_noop", func(*config.Config) (pipeline.Processor, error) { return ackTestNoop{}, nil })
	pipeline.RegisterProcessorPlugin("test_ack_failure", func(*config.Config) (pipeline.Processor, error) { return ackFailureProcessor{}, nil })
	if err := pipeline.RegisterRecordCodecOrReplace(ackTestBadCodec{}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		codec      string
		pushErr    error
		deadLetter bool
	}{
		{name: "output_success", codec: "otel"},
		{name: "output_push_failure", codec: "otel", pushErr: errors.New("durable backend rejected record")},
		{name: "output_encode_failure", codec: "test_ack_bad"},
		{name: "dead_letter_success", codec: "otel", deadLetter: true},
		{name: "dead_letter_failure", codec: "otel", pushErr: errors.New("dead letter rejected record"), deadLetter: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kv.Register(t.Name(), &ackTestKV{data: map[string][]byte{}})
			q := &ackTestQueue{pushErr: tc.pushErr}
			queue.RegisterDefaultHandler(q)
			name := t.Name()
			lane.GetOrCreate(lane.LaneConfig{Name: name})
			defer func() {
				if err := lane.Delete(name, true); err != nil {
					t.Error(err)
				}
			}()
			processor := "test_ack_noop"
			onFailure := "tag"
			if tc.deadLetter {
				processor = "test_ack_failure"
				onFailure = "dead_letter:" + name + "-dead"
			}
			cfg, err := config.NewConfigFrom(map[string]interface{}{"lane": name, "batch_max": 1, "on_failure": onFailure, "output_queue": name + "-out", "output_codec": tc.codec, "processor": []interface{}{map[string]interface{}{processor: map[string]interface{}{}}}})
			if err != nil {
				t.Fatal(err)
			}
			lc, err := modpipeline.NewLaneConsumerProcessor(cfg)
			if err != nil {
				t.Fatal(err)
			}
			lcCtx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() {
				done <- lc.Process(&pipeline.Context{Context: lcCtx})
				close(done)
			}()
			defer func() {
				cancel()
				if err := <-done; err != nil {
					t.Error(err)
				}
			}()
			p := &QueueConsumerProcessor{config: &Config{NumOfSlices: 1, MessageField: "messages", QueueField: "queue", Consumer: &queue.ConsumerConfig{Group: name, Name: name}, AutoCommitOffset: true, QuitOnEOFQueue: true, AckTimeoutInMs: 1000, OutputLane: name, Codec: "otel"}}
			ctx := &pipeline.Context{}
			ctx.ResetContext()
			p.wg.Add(1)
			p.NewSlicedWorker(ctx, &queue.QueueConfig{ID: name, Name: name}, "worker", 0, 1, (*pipeline.Context)(nil))
			// Stop the subscriber before reading counters after a failed ACK.
			// Another record can still be in its callback when the gate fails.
			cancel()
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			t.Logf("codec=%s, durable push calls=%d, committed offsets=%v, failed=%v", tc.codec, q.pushes, q.committed, ctx.IsFailed())
			if tc.pushErr == nil && tc.codec == "otel" {
				if len(q.committed) != 1 || q.pushes != 2 || ctx.IsFailed() {
					t.Fatalf("successful durable output must commit: offsets=%v pushes=%d failed=%v", q.committed, q.pushes, ctx.IsFailed())
				}
				return
			}
			if len(q.committed) != 0 || !ctx.IsFailed() {
				t.Fatalf("failed output must fail the batch without commit: offsets=%v failed=%v", q.committed, ctx.IsFailed())
			}
		})
	}
}
