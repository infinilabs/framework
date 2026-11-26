// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package kafka_queue

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/queue"
)

// integrationKVOnce installs the shared test store before any consumer acquires a lease.
var integrationKVOnce sync.Once

// testMemoryKV provides isolated lease storage for the Kafka integration cases.
//
// These tests exercise Kafka recovery without requiring a separate framework KV
// service. The mutex protects concurrent reads, confirmations, and cleanup.
type testMemoryKV struct {
	kv.KVStore
	mu     sync.Mutex
	values map[string][]byte
}

// GetValue returns a copy so callers cannot mutate stored lease bytes.
func (m *testMemoryKV) GetValue(bucket string, key []byte) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]byte(nil), m.values[bucket+string(key)]...), nil
}

// AddValue stores a copy because the caller retains ownership of its input bytes.
func (m *testMemoryKV) AddValue(bucket string, key, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[bucket+string(key)] = append([]byte(nil), value...)
	return nil
}

// DeleteKey removes the lease when consumer cleanup releases ownership.
func (m *testMemoryKV) DeleteKey(bucket string, key []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.values, bucket+string(key))
	return nil
}

// ExistsKey reports whether the test store currently contains the requested key.
func (m *testMemoryKV) ExistsKey(bucket string, key []byte) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.values[bucket+string(key)]
	return ok, nil
}

// Helper function to create an isolated topic using an explicitly configured test broker.
func integrationQueue(t *testing.T) (*KafkaQueue, *queue.QueueConfig, *queue.ConsumerConfig) {
	t.Helper()
	brokers := os.Getenv("FRAMEWORK_KAFKA_BROKERS")
	if brokers == "" {
		t.Skip("set FRAMEWORK_KAFKA_BROKERS to run real Kafka tests")
	}
	integrationKVOnce.Do(func() {
		global.Env()
		kv.Register("kafka-integration", &testMemoryKV{values: map[string][]byte{}})
	})
	name := fmt.Sprintf("framework-prefix-%d", time.Now().UnixNano())
	q := &KafkaQueue{cfg: &Config{Brokers: strings.Split(brokers, ","), NumOfPartition: 3, NumOfReplica: 1}}
	q.adminClient = kadm.NewClient(q.newClient(nil))
	t.Cleanup(func() {
		if err := q.Stop(); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := q.adminClient.CreateTopic(ctx, 3, 1, nil, name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cleanupClient := kadm.NewClient(q.newClient(nil))
		defer cleanupClient.Close()
		if _, err := cleanupClient.DeleteTopic(ctx, name); err != nil {
			t.Error(err)
		}
	})
	qc := &queue.QueueConfig{ID: name, Name: name, Type: "kafka"}
	cc := queue.NewConsumerConfig(name, "prefix", "worker")
	cc.AutoResetOffset = "earliest"
	cc.FetchMaxWaitMs = 5000
	cc.FetchMaxMessages = 100
	return q, qc, cc
}

// Helper function to write known physical partitions for the integration cases.
func produceIntegration(t *testing.T, q *KafkaQueue, topic string, counts map[int32]int) {
	t.Helper()
	client := q.newClient([]kgo.Opt{kgo.RecordPartitioner(kgo.ManualPartitioner())})
	defer client.Close()
	records := []*kgo.Record{}
	for partition, count := range counts {
		for n := 0; n < count; n++ {
			records = append(records, &kgo.Record{Topic: topic, Partition: partition, Value: []byte(fmt.Sprintf("%d:%d", partition, n))})
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := client.ProduceSync(ctx, records...).FirstErr(); err != nil {
		t.Fatal(err)
	}
}

// Helper function to acquire and clean up an integration session even after a fatal assertion.
func acquireIntegration(t *testing.T, q *KafkaQueue, qc *queue.QueueConfig, cc *queue.ConsumerConfig) *Consumer {
	t.Helper()
	instance, err := q.AcquireConsumer(qc, cc)
	if err != nil {
		t.Fatal(err)
	}
	c := instance.(*Consumer)
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
	})
	return c
}

// Real Kafka persists exact prefixes across multiple fetches and rejects stale session tokens.
func TestKafkaIntegrationPrefixRecovery(t *testing.T) {
	q, qc, cc := integrationQueue(t)
	produceIntegration(t, q, qc.ID, map[int32]int{0: 10, 2: 10})
	c := acquireIntegration(t, q, qc, cc)
	messages := []queue.Message{}
	for i := 0; i < 3; i++ {
		batch, _, err := c.FetchMessages(&queue.Context{}, 5)
		if err != nil {
			t.Fatal(err)
		}
		if len(batch) != 5 {
			t.Fatalf("got %d records", len(batch))
		}
		messages = append(messages, batch...)
	}
	anchor := messages[7].NextOffset
	expected := map[int32]int64{}
	for _, m := range messages[:8] {
		expected[int32(m.NextOffset.Segment)] = m.NextOffset.Position
	}
	if ok, err := q.CommitOffset(qc, cc, anchor); err != nil || !ok {
		t.Fatal(ok, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	offsets, err := q.adminClient.FetchOffsetsForTopics(ctx, getGroupForKafka(cc.Group, qc.ID), qc.ID)
	if err != nil {
		t.Fatal(err)
	}
	for part := int32(0); part < 3; part++ {
		got, ok := offsets.Lookup(qc.ID, part)
		want, present := expected[part]
		if present && (!ok || got.Err != nil || got.At != want) {
			t.Fatalf("p%d=%v, want %d", part, got, want)
		}
		if !present && ok && got.At >= 0 {
			t.Fatalf("unconfirmed partition committed: %v", got)
		}
	}
	if err := c.CommitOffset(messages[len(messages)-1].NextOffset); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	replacement := acquireIntegration(t, q, qc, cc)
	recovered, _, err := replacement.FetchMessages(&queue.Context{}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(recovered) != 5 {
		t.Fatalf("remaining records=%d, want 5", len(recovered))
	}
	if ok, err := q.CommitOffset(qc, cc, anchor); ok || err == nil {
		t.Fatal("old acknowledgement entered replacement session")
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if value, ok := q.consumers.Load(getGroupForKafka(cc.Group, qc.ID)); !ok || value != replacement {
		t.Fatal("old close removed replacement")
	}
}

// Administrative commits target only the requested partition and Kafka rejects nonempty groups.
func TestKafkaIntegrationAdminAndProgress(t *testing.T) {
	q, qc, cc := integrationQueue(t)
	produceIntegration(t, q, qc.ID, map[int32]int{0: 2, 2: 4})
	if q.Depth(qc.ID) != 6 {
		t.Fatal("depth does not cover all partitions")
	}
	lag, _ := q.GetOffset(qc, cc)
	if lag.Position != 0 {
		t.Fatal("uninitialized group was reported caught up")
	}
	if ok, err := q.CommitOffset(qc, cc, queue.NewOffsetWithVersion(2, 2, 1)); !ok || err != nil {
		t.Fatal(ok, err)
	}
	c := acquireIntegration(t, q, qc, cc)
	records, _, err := c.FetchMessages(&queue.Context{}, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 4 {
		t.Fatalf("records=%d", len(records))
	}
	if ok, err := q.CommitOffset(qc, cc, queue.NewOffsetWithVersion(2, 0, 1)); ok || err == nil {
		t.Fatal("local active group reset succeeded")
	}
	// A second backend has no local registry; the broker must enforce the group restriction.
	other := &KafkaQueue{adminClient: q.adminClient}
	if ok, err := other.CommitOffset(qc, cc, queue.NewOffsetWithVersion(2, 0, 1)); ok || err == nil {
		t.Fatal("broker accepted reset of a remote active group")
	}
	if err := c.CommitOffset(records[len(records)-1].NextOffset); err != nil {
		t.Fatal(err)
	}
	// An empty partition needs no artificial administrative acknowledgement.
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	offset, _ := q.GetOffset(qc, cc)
	latest := q.LatestOffset(qc)
	if offset != latest {
		t.Fatalf("got %v, want caught-up %v", offset, latest)
	}
	produceIntegration(t, q, qc.ID, map[int32]int{2: 1})
	offset, _ = q.GetOffset(qc, cc)
	if offset == latest {
		t.Fatal("nonzero partition lag was ignored")
	}
	if err := q.DeleteOffset(qc, cc); err != nil {
		t.Fatal(err)
	}
	offset, _ = q.GetOffset(qc, cc)
	if offset == latest {
		t.Fatal("deleted progress was reported caught up")
	}
}

// Browsing before the first assignment neither joins nor commits the business group.
func TestKafkaIntegrationBrowse(t *testing.T) {
	q, qc, cc := integrationQueue(t)
	produceIntegration(t, q, qc.ID, map[int32]int{2: 5})
	c := acquireIntegration(t, q, qc, cc)
	if err := c.ResetOffset(2, 3); err != nil {
		t.Fatal(err)
	}
	batch, _, err := c.FetchMessages(&queue.Context{}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch) != 2 || batch[0].Offset.Segment != 2 || batch[0].Offset.Position != 3 {
		t.Fatalf("unexpected browse: %v", batch)
	}
	if err := c.CommitOffset(batch[1].NextOffset); err == nil {
		t.Fatal("browse committed progress")
	}
	if _, ok := q.consumers.Load(getGroupForKafka(cc.Group, qc.ID)); ok {
		t.Fatal("browse registered a business session")
	}
	if err := c.ResetOffset(2, 0); err == nil {
		t.Fatal("reset after fetch succeeded")
	}
}

// Nested auto-commit cannot advance Kafka offsets without an explicit business acknowledgement.
func TestKafkaIntegrationAutoCommitAndClose(t *testing.T) {
	q, qc, cc := integrationQueue(t)
	produceIntegration(t, q, qc.ID, map[int32]int{0: 2})
	cc.AutoCommitOffset = true
	c := acquireIntegration(t, q, qc, cc)
	if _, _, err := c.FetchMessages(&queue.Context{}, 1); err != nil {
		t.Fatal(err)
	}
	if got := c.client.OptValue(kgo.DisableAutoCommit); got != true {
		t.Fatalf("SDK DisableAutoCommit=%v", got)
	}
	if _, _, err := c.FetchMessages(&queue.Context{}, 1); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	offsets, err := q.adminClient.FetchOffsetsForTopics(ctx, getGroupForKafka(cc.Group, qc.ID), qc.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range offsets[qc.ID] {
		if p.At >= 0 {
			t.Fatal("SDK advanced unconfirmed progress")
		}
	}
	cc.FetchMaxWaitMs = 30000
	c.cCfg.FetchMaxWaitMs = 30000
	done := make(chan error, 1)
	go func() { _, _, err := c.FetchMessages(&queue.Context{}, 1); done <- err }()
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("closed poll did not fail")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("close did not unblock poll")
	}
}

// A second Kafka member expires the old session before its delivery history can be reused.
func TestKafkaIntegrationRebalanceExpiresSession(t *testing.T) {
	q, qc, cc := integrationQueue(t)
	produceIntegration(t, q, qc.ID, map[int32]int{0: 2, 2: 2})
	c := acquireIntegration(t, q, qc, cc)
	messages, _, err := c.FetchMessages(&queue.Context{}, 1)
	if err != nil || len(messages) != 1 {
		t.Fatalf("initial delivery: %v, %v", messages, err)
	}
	other := q.newClient([]kgo.Opt{
		kgo.ConsumerGroup(getGroupForKafka(cc.Group, qc.ID)),
		kgo.ConsumeTopics(qc.ID), kgo.DisableAutoCommit(),
		kgo.Balancers(kgo.RangeBalancer()),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	done := make(chan struct{})
	go func() { other.PollRecords(ctx, 1); close(done) }()
	t.Cleanup(func() { cancel(); other.Close(); <-done })
	select {
	case <-c.ioCtx.Done():
	case <-ctx.Done():
		t.Fatal("rebalance did not expire the original session")
	}
	if err := c.CommitOffset(messages[0].NextOffset); err == nil {
		t.Fatal("revoked instance accepted its old acknowledgement")
	}
	if ok, err := q.CommitOffset(qc, cc, messages[0].NextOffset); ok || err == nil {
		t.Fatal("queue accepted an acknowledgement after rebalance")
	}
}

// The public simple-queue entry point accepts its negative wait and commits exactly one message.
func TestKafkaIntegrationPublicPop(t *testing.T) {
	q, qc, _ := integrationQueue(t)
	produceIntegration(t, q, qc.ID, map[int32]int{0: 2})
	qc.Type = qc.ID
	queue.Register(qc.Type, q)
	if _, err := queue.RegisterConfig(qc); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { queue.RemoveConfig(qc) })
	for i := 0; i < 2; i++ {
		value, err := queue.Pop(qc)
		if err != nil || string(value) != fmt.Sprintf("0:%d", i) {
			t.Fatalf("value=%q error=%v", value, err)
		}
		q.instances.Range(func(_, _ interface{}) bool { t.Error("Pop retained an instance"); return false })
		q.consumers.Range(func(_, _ interface{}) bool { t.Error("Pop retained a lease"); return false })
	}
	start := time.Now()
	value, timeout := q.Pop(qc.ID, 100*time.Millisecond)
	if len(value) != 0 || !timeout || time.Since(start) > 3*time.Second {
		t.Fatal("finite Pop did not time out")
	}
}

// Module Stop cancels a direct browse client and prevents every client factory from reopening it.
func TestKafkaIntegrationStopBrowse(t *testing.T) {
	q, qc, cc := integrationQueue(t)
	c := acquireIntegration(t, q, qc, cc)
	if err := c.ResetOffset(1, 0); err != nil {
		t.Fatal(err)
	}
	c.cCfg.FetchMaxWaitMs = 25
	if _, timeout, err := c.FetchMessages(&queue.Context{}, 1); err != nil || !timeout {
		t.Fatal(timeout, err)
	}
	if err := q.Stop(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if !c.client.PollRecords(ctx, 1).IsClientClosed() {
		t.Fatal("Stop retained the browse client")
	}
	if _, _, err := c.FetchMessages(&queue.Context{}, 1); err == nil {
		t.Fatal("browse continued after Stop")
	}
	if _, err := q.AcquireConsumer(qc, cc); err == nil {
		t.Fatal("consumer opened after Stop")
	}
	if _, err := q.AcquireProducer(qc); err == nil {
		t.Fatal("producer opened after Stop")
	}
	if err := q.Init(qc.ID); err == nil {
		t.Fatal("topic client opened after Stop")
	}
}
