package queue

import (
	"errors"
	"infini.sh/framework/core/stats"
	"sync"
	"testing"
	"time"
)

type acquireConsumerTestHandler struct {
	acquireFunc func(k *QueueConfig, consumer *ConsumerConfig) (ConsumerAPI, error)
}

func (h *acquireConsumerTestHandler) Name() string                     { return "test" }
func (h *acquireConsumerTestHandler) Init(string) error                { return nil }
func (h *acquireConsumerTestHandler) Close(string) error               { return nil }
func (h *acquireConsumerTestHandler) GetStorageSize(string) uint64     { return 0 }
func (h *acquireConsumerTestHandler) Destroy(string) error             { return nil }
func (h *acquireConsumerTestHandler) GetQueues() []string              { return nil }
func (h *acquireConsumerTestHandler) Push(string, []byte) error        { return nil }
func (h *acquireConsumerTestHandler) LatestOffset(*QueueConfig) Offset { return Offset{} }
func (h *acquireConsumerTestHandler) GetOffset(*QueueConfig, *ConsumerConfig) (Offset, error) {
	return Offset{}, nil
}
func (h *acquireConsumerTestHandler) DeleteOffset(*QueueConfig, *ConsumerConfig) error { return nil }
func (h *acquireConsumerTestHandler) CommitOffset(*QueueConfig, *ConsumerConfig, Offset) (bool, error) {
	return true, nil
}
func (h *acquireConsumerTestHandler) AcquireConsumer(k *QueueConfig, consumer *ConsumerConfig) (ConsumerAPI, error) {
	if h.acquireFunc != nil {
		return h.acquireFunc(k, consumer)
	}
	return &acquireConsumerTestConsumer{}, nil
}
func (h *acquireConsumerTestHandler) ReleaseConsumer(*QueueConfig, *ConsumerConfig, ConsumerAPI) error {
	return nil
}
func (h *acquireConsumerTestHandler) AcquireProducer(*QueueConfig) (ProducerAPI, error) {
	return nil, nil
}
func (h *acquireConsumerTestHandler) ReleaseProducer(*QueueConfig, ProducerAPI) error { return nil }

type acquireConsumerTestConsumer struct{}

func (c *acquireConsumerTestConsumer) Close() error                   { return nil }
func (c *acquireConsumerTestConsumer) ResetOffset(int64, int64) error { return nil }
func (c *acquireConsumerTestConsumer) FetchMessages(*Context, int) ([]Message, bool, error) {
	return nil, false, nil
}
func (c *acquireConsumerTestConsumer) CommitOffset(Offset) error { return nil }

type acquireConsumerTestStats struct {
	mu         sync.Mutex
	timestamps map[string]time.Time
}

func (s *acquireConsumerTestStats) Increment(string, string)          {}
func (s *acquireConsumerTestStats) IncrementBy(string, string, int64) {}
func (s *acquireConsumerTestStats) Decrement(string, string)          {}
func (s *acquireConsumerTestStats) DecrementBy(string, string, int64) {}
func (s *acquireConsumerTestStats) Absolute(string, string, int64)    {}
func (s *acquireConsumerTestStats) Timing(string, string, int64)      {}
func (s *acquireConsumerTestStats) Gauge(string, string, int64)       {}
func (s *acquireConsumerTestStats) Stat(string, string) int64         { return 0 }
func (s *acquireConsumerTestStats) StatsAll() string                  { return "" }
func (s *acquireConsumerTestStats) RecordTimestamp(category, key string, value time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.timestamps[category+"."+key] = value
}
func (s *acquireConsumerTestStats) GetTimestamp(category, key string) (time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.timestamps[category+"."+key]
	if !ok {
		return time.Time{}, errors.New("not found")
	}
	return v, nil
}
func (s *acquireConsumerTestStats) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.timestamps = map[string]time.Time{}
}

var acquireConsumerStatsOnce sync.Once
var acquireConsumerStatsHandler = &acquireConsumerTestStats{timestamps: map[string]time.Time{}}

func withTestQueueHandler(t *testing.T, handler AdvancedQueueAPI) {
	t.Helper()
	previousDefaultHandler := defaultHandler
	previousConsumersInFighting := consumersInFighting
	acquireConsumerStatsOnce.Do(func() {
		stats.Register(acquireConsumerStatsHandler)
	})
	acquireConsumerStatsHandler.reset()
	defaultHandler = handler
	consumersInFighting = syncMapZero()
	t.Cleanup(func() {
		defaultHandler = previousDefaultHandler
		consumersInFighting = previousConsumersInFighting
	})
}

func syncMapZero() sync.Map {
	return sync.Map{}
}

func TestAcquireConsumerStoresReservation(t *testing.T) {
	withTestQueueHandler(t, &acquireConsumerTestHandler{})

	q := &QueueConfig{ID: "queue-1", Name: "queue-1"}
	c := &ConsumerConfig{Group: "group", Name: "consumer"}
	c.ID = "consumer-1"

	instance, err := AcquireConsumer(q, c, "client-1")
	if err != nil {
		t.Fatalf("expected acquire to succeed, got %v", err)
	}
	if instance == nil {
		t.Fatal("expected consumer instance to be returned")
	}
	if owner, ok := consumersInFighting.Load(q.ID + c.Key()); !ok || owner != "client-1" {
		t.Fatalf("expected fighting list reservation to be stored, got owner=%v exists=%v", owner, ok)
	}
}

func TestAcquireConsumerRollsBackReservationOnError(t *testing.T) {
	withTestQueueHandler(t, &acquireConsumerTestHandler{
		acquireFunc: func(k *QueueConfig, consumer *ConsumerConfig) (ConsumerAPI, error) {
			return nil, errors.New("boom")
		},
	})

	q := &QueueConfig{ID: "queue-1", Name: "queue-1"}
	c := &ConsumerConfig{Group: "group", Name: "consumer"}
	c.ID = "consumer-1"

	_, err := AcquireConsumer(q, c, "client-1")
	if err == nil {
		t.Fatal("expected acquire to fail")
	}
	if _, ok := consumersInFighting.Load(q.ID + c.Key()); ok {
		t.Fatal("expected fighting list reservation to be rolled back")
	}
}

func TestAcquireConsumerRollsBackReservationOnPanic(t *testing.T) {
	withTestQueueHandler(t, &acquireConsumerTestHandler{
		acquireFunc: func(k *QueueConfig, consumer *ConsumerConfig) (ConsumerAPI, error) {
			panic("boom")
		},
	})

	q := &QueueConfig{ID: "queue-1", Name: "queue-1"}
	c := &ConsumerConfig{Group: "group", Name: "consumer"}
	c.ID = "consumer-1"

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected acquire to panic")
		}
		if _, ok := consumersInFighting.Load(q.ID + c.Key()); ok {
			t.Fatal("expected fighting list reservation to be rolled back after panic")
		}
	}()

	_, _ = AcquireConsumer(q, c, "client-1")
}

func TestAcquireConsumerBlocksCompetingClient(t *testing.T) {
	withTestQueueHandler(t, &acquireConsumerTestHandler{})

	q := &QueueConfig{ID: "queue-1", Name: "queue-1"}
	c := &ConsumerConfig{Group: "group", Name: "consumer"}
	c.ID = "consumer-1"
	consumersInFighting.Store(q.ID+c.Key(), "client-1")

	_, err := AcquireConsumer(q, c, "client-2")
	if err == nil || err.Error() != "the consumer is in fighting list" {
		t.Fatalf("expected fighting list error, got %v", err)
	}
}

func TestAcquireConsumerAllowsSameClientReentry(t *testing.T) {
	withTestQueueHandler(t, &acquireConsumerTestHandler{})

	q := &QueueConfig{ID: "queue-1", Name: "queue-1"}
	c := &ConsumerConfig{Group: "group", Name: "consumer"}
	c.ID = "consumer-1"
	consumersInFighting.Store(q.ID+c.Key(), "client-1")

	instance, err := AcquireConsumer(q, c, "client-1")
	if err != nil {
		t.Fatalf("expected same client to re-enter, got %v", err)
	}
	if instance == nil {
		t.Fatal("expected consumer instance for same-client reentry")
	}
}

func TestAcquireConsumerRetriesExpiredReservation(t *testing.T) {
	withTestQueueHandler(t, &acquireConsumerTestHandler{})

	q := &QueueConfig{ID: "queue-1", Name: "queue-1"}
	c := &ConsumerConfig{
		Group:                   "group",
		Name:                    "consumer",
		ConsumeTimeoutInSeconds: 1,
	}
	c.ID = "consumer-1"
	c.KeepActive()
	stale := time.Now().Add(-3 * time.Second)
	stats.Timestamp("consumer", c.ID+".last_active", stale)
	consumersInFighting.Store(q.ID+c.Key(), "client-2")

	instance, err := AcquireConsumer(q, c, "client-1")
	if err != nil {
		t.Fatalf("expected expired reservation to be retried, got %v", err)
	}
	if instance == nil {
		t.Fatal("expected consumer instance after expired reservation cleanup")
	}
	if owner, ok := consumersInFighting.Load(q.ID + c.Key()); !ok || owner != "client-1" {
		t.Fatalf("expected ownership to move to client-1, got owner=%v exists=%v", owner, ok)
	}
}
