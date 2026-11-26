package kafka_queue

import (
	"context"
	"reflect"
	"testing"
	"unsafe"

	"github.com/twmb/franz-go/pkg/kgo"
	"infini.sh/framework/core/queue"
)

// Helper function to create a processing session without an external broker.
func newTestConsumer() *Consumer {
	ctx, cancel := context.WithCancel(context.Background())
	return &Consumer{qCfg: &queue.QueueConfig{ID: "test-topic", Name: "test-topic"}, cCfg: &queue.ConsumerConfig{Group: "commit-tests"}, phase: consumerProcessing, version: -1, ioCtx: ctx, cancel: cancel, assigned: true, generation: -1, order: deliveryOrder{budget: defaultOrderMemoryBudget}}
}

// Helper function to deliver an explicit range to multiple acknowledgement tests.
func appendRecords(t *testing.T, c *Consumer, partition int32, start, end int64) {
	t.Helper()
	records := make([]*kgo.Record, 0)
	for n := start; n < end; n++ {
		records = append(records, &kgo.Record{Topic: c.qCfg.ID, Partition: partition, Offset: n})
	}
	if err := c.order.append(records); err != nil {
		t.Fatal(err)
	}
}

// Acknowledgements resolve the exact delivery prefix, including across fetches and gaps.
func TestDeliveryOrderPrefixes(t *testing.T) {
	for _, tc := range []struct {
		name      string
		partition int32
		next      int64
		want      map[int32]int64
		invalid   bool
	}{
		{"inside-first-run", 1, 5, map[int32]int64{1: 5}, false},
		{"end-first-run", 1, 10, map[int32]int64{1: 10}, false},
		{"cross-partition", 3, 5, map[int32]int64{1: 10, 3: 5}, false},
		{"end-second-run", 3, 10, map[int32]int64{1: 10, 3: 10}, false},
		{"later-fetch", 1, 21, map[int32]int64{1: 21, 3: 10}, false},
		{"gap", 1, 15, nil, true},
		{"unknown-partition", 2, 1, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestConsumer()
			defer closeTestConsumer(t, c)
			appendRecords(t, c, 1, 0, 10)
			appendRecords(t, c, 3, 0, 10)
			appendRecords(t, c, 1, 20, 21)
			got, covered, err := c.order.target(tc.partition, tc.next)
			if (err != nil) != tc.invalid {
				t.Fatalf("error=%v", err)
			}
			if tc.invalid {
				return
			}
			if covered || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v covered=%t, want %v", got, covered, tc.want)
			}
		})
	}
}

// Successful chunk commits retain fetched positions needed by a later cross-partition commit.
func TestDeliveryOrderRetireAndRepeat(t *testing.T) {
	c := newTestConsumer()
	defer closeTestConsumer(t, c)
	appendRecords(t, c, 1, 0, 10)
	appendRecords(t, c, 3, 0, 10)
	first, _, err := c.order.target(1, 5)
	if err != nil || len(first) != 1 || first[1] != 5 {
		t.Fatal(first, err)
	}
	c.order.retire(1, 5, first)
	got, _, err := c.order.target(3, 10)
	if err != nil || got[1] != 10 || got[3] != 10 {
		t.Fatalf("target=%v error=%v", got, err)
	}
	c.order.retire(3, 10, got)
	if c.order.head != nil || c.order.used != 0 {
		t.Fatal("completed blocks retained")
	}
	if _, covered, err := c.order.target(1, 5); err != nil || !covered {
		t.Fatal("completed acknowledgement not covered", err)
	}
	appendRecords(t, c, 1, 10, 12)
	got, _, err = c.order.target(1, 11)
	if err != nil || got[1] != 11 || got[3] != 10 {
		t.Fatal(got, err)
	}
}

// Adjacent records merge, while a finite budget fails without silently dropping history.
func TestDeliveryOrderBudget(t *testing.T) {
	c := newTestConsumer()
	defer closeTestConsumer(t, c)
	c.order.budget = uint64(unsafe.Sizeof(deliveryBlock{}))
	appendRecords(t, c, 1, 0, 10000)
	if c.order.head.end != 1 {
		t.Fatal("contiguous records did not merge")
	}
	for n := int64(0); n < deliveryBlockRuns-1; n++ {
		if n%2 == 0 {
			appendRecords(t, c, 3, n, n+1)
		} else {
			appendRecords(t, c, 1, 10000+n, 10001+n)
		}
	}
	if err := c.order.append([]*kgo.Record{{Topic: c.qCfg.ID, Partition: 3, Offset: deliveryBlockRuns + 10}}); err == nil {
		t.Fatal("budget did not fail")
	}
}

// A repeated or decreasing partition position cannot enter the delivery history.
func TestDeliveryOrderRejectsBackwards(t *testing.T) {
	c := newTestConsumer()
	defer closeTestConsumer(t, c)
	appendRecords(t, c, 1, 10, 11)
	if err := c.order.append([]*kgo.Record{{Partition: 1, Offset: 10}}); err == nil {
		t.Fatal("duplicate accepted")
	}
}

// Delivery compression and prefix lookup must not allocate per message.
func BenchmarkDeliveryOrder(b *testing.B) {
	records := make([]*kgo.Record, 1000)
	for i := range records {
		records[i] = &kgo.Record{Partition: 1, Offset: int64(i)}
	}
	b.ReportAllocs()
	for b.Loop() {
		o := deliveryOrder{budget: defaultOrderMemoryBudget}
		if err := o.append(records); err != nil {
			b.Fatal(err)
		}
		targets, _, err := o.target(1, 1000)
		if err != nil {
			b.Fatal(err)
		}
		o.retire(1, 1000, targets)
	}
}

// Concurrent acquisition and Stop cannot leave a returned instance outside the shutdown registry.
func TestAcquireDuringStop(t *testing.T) {
	for i := 0; i < 100; i++ {
		q := &KafkaQueue{}
		cfg := &queue.QueueConfig{ID: "test-topic"}
		cc := &queue.ConsumerConfig{Group: "group"}
		done := make(chan queue.ConsumerAPI, 1)
		go func() { instance, _ := q.AcquireConsumer(cfg, cc); done <- instance }()
		if err := q.Stop(); err != nil {
			t.Fatal(err)
		}
		if instance := <-done; instance != nil {
			if _, _, err := instance.FetchMessages(&queue.Context{}, 1); err == nil {
				t.Fatal("acquired instance survived Stop")
			}
			if err := instance.Close(); err != nil {
				t.Fatal(err)
			}
		}
		q.instances.Range(func(_, _ interface{}) bool { t.Error("Stop retained an instance"); return false })
	}
}

// Helper function to report consumer cleanup errors in multiple tests.
func closeTestConsumer(t *testing.T, c *Consumer) {
	t.Helper()
	if err := c.Close(); err != nil {
		t.Error(err)
	}
}
