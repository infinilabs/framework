// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software. For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello#infini.ltd
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

/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package lane_bulk

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/lane"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
)

// ─── fake in-memory queue backend ────────────────────────────────────────────

type memQueue struct {
	mu   sync.Mutex
	data map[string][][]byte
}

func (q *memQueue) Name() string { return "mem" }
func (q *memQueue) Init(string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.data == nil {
		q.data = map[string][][]byte{}
	}
	return nil
}
func (q *memQueue) Close(string) error           { return nil }
func (q *memQueue) Destroy(string) error         { return nil }
func (q *memQueue) GetStorageSize(string) uint64 { return 0 }
func (q *memQueue) GetQueues() []string          { return nil }

func (q *memQueue) Push(id string, v []byte) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.data == nil {
		q.data = map[string][][]byte{}
	}
	q.data[id] = append(q.data[id], v)
	return nil
}

func (q *memQueue) Pop(id string, timeout time.Duration) ([]byte, bool) {
	q.mu.Lock()
	if items := q.data[id]; len(items) > 0 {
		v := items[0]
		q.data[id] = items[1:]
		q.mu.Unlock()
		return v, false
	}
	q.mu.Unlock()
	d := timeout
	if d > 200*time.Millisecond {
		d = 200 * time.Millisecond
	}
	time.Sleep(d)
	return nil, true
}

func (q *memQueue) Depth(id string) int64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return int64(len(q.data[id]))
}

var memQueueOnce sync.Once

// memKV backs the queue-config registry in unit tests (GetOrInitConfig
// persists configs through the kv store; without a handler it panics).
type memKV struct {
	data map[string][]byte
}

func (m *memKV) Open() error  { return nil }
func (m *memKV) Close() error { return nil }
func (m *memKV) GetValue(bucket string, key []byte) ([]byte, error) {
	if v, ok := m.data[string(key)]; ok {
		return append([]byte(nil), v...), nil
	}
	return nil, nil
}
func (m *memKV) GetCompressedValue(bucket string, key []byte) ([]byte, error) {
	return m.GetValue(bucket, key)
}
func (m *memKV) AddValueCompress(bucket string, key, value []byte) error {
	return m.AddValue(bucket, key, value)
}
func (m *memKV) AddValue(bucket string, key, value []byte) error {
	m.data[string(key)] = append([]byte(nil), value...)
	return nil
}
func (m *memKV) ExistsKey(bucket string, key []byte) (bool, error) {
	_, ok := m.data[string(key)]
	return ok, nil
}
func (m *memKV) DeleteKey(bucket string, key []byte) error {
	delete(m.data, string(key))
	return nil
}

// installMemQueue wires the fake queue backend and kv store so
// GetOrInitConfig-created queues (empty type) resolve end to end.
func installMemQueue() *memQueue {
	q := &memQueue{}
	memQueueOnce.Do(func() {
		queue.RegisterDefaultHandler(q)
		kv.Register("lane-bulk-test-kv", &memKV{data: map[string][]byte{}})
	})
	return q
}

// ─── payload codec ───────────────────────────────────────────────────────────

func TestBulkPayloadCodecRoundTrip(t *testing.T) {
	payload := []byte("{\"index\":{}}\n{\"a\":1}\n")

	rec, err := bulkPayloadCodec{}.Decode(payload)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := pipeline.CodecHint(rec); got != "bulk" {
		t.Errorf("codec hint = %q, want bulk", got)
	}
	out, err := bulkPayloadCodec{}.Encode(rec)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if string(out) != string(payload) {
		t.Errorf("round-trip mismatch:\n got: %q\nwant: %q", out, payload)
	}

	if _, err := (bulkPayloadCodec{}).Decode(nil); err == nil {
		t.Error("empty payload should error")
	}
	bare := &event.Event{Fields: util.MapStr{"not_bulk": 1}}
	if _, err := (bulkPayloadCodec{}).Encode(bare); err == nil {
		t.Error("record without payload field should error")
	}
}

// ─── merge formatting helpers ────────────────────────────────────────────────

func TestFingerprintID(t *testing.T) {
	p := &MergeToBulkLaneProcessor{config: MergeConfig{KeyFields: []string{"file.path", "file.offset"}}}

	doc := []byte(`{"file":{"path":"/var/log/a.log","offset":42},"message":"x"}`)

	a := p.fingerprintID(doc)
	b := p.fingerprintID(doc)
	if a != b {
		t.Errorf("fingerprint not deterministic: %v vs %v", a, b)
	}

	// a different offset must yield a different id
	doc2 := []byte(`{"file":{"path":"/var/log/a.log","offset":43},"message":"x"}`)
	if p.fingerprintID(doc2) == a {
		t.Error("offset change should change the fingerprint")
	}

	// no key field matches: content-hash fallback, distinct docs stay distinct
	c1 := p.fingerprintID([]byte(`{"message":"one"}`))
	c2 := p.fingerprintID([]byte(`{"message":"two"}`))
	if c1 == c2 {
		t.Error("content fallback collapsed distinct documents")
	}
}

func TestTrimJSONString(t *testing.T) {
	if s, ok := trimJSONString([]byte(`"hello"`)); !ok || s != "hello" {
		t.Errorf("quoted token: %q %v", s, ok)
	}
	if _, ok := trimJSONString([]byte(`42`)); ok {
		t.Error("non-string token should not trim")
	}
}

func TestNormalizeDataStreamDoc(t *testing.T) {
	env := []byte(`{"metadata":{"file":{"path":"/a.log"}},"payload":{"message":"hi","timestamp":"2026-10-15T00:00:00Z"},"timestamp":"2026-10-15T00:00:00Z"}`)

	out := normalizeDataStreamDoc(env)
	m := util.MapStr{}
	if err := util.FromJSONBytes(out, &m); err != nil {
		t.Fatalf("normalized doc is not JSON: %v", err)
	}
	if m["message"] != "hi" {
		t.Errorf("payload.message not promoted: %v", m)
	}
	if m["@timestamp"] != "2026-10-15T00:00:00Z" {
		t.Errorf("@timestamp = %v", m["@timestamp"])
	}
	// @timestamp is the only real timestamp: the payload's timestamp folds
	// into @timestamp and is dropped — no top-level mirror is written.
	if _, ok := m["timestamp"]; ok {
		t.Errorf("timestamp mirror must not survive: %v", m["timestamp"])
	}
	// file origin no longer promotes to a top-level "file"
	// (collection provenance lives on @agent.file_*)
	if _, ok := m["file"]; ok {
		t.Errorf("top-level file promotion is gone: %v", m["file"])
	}

	// bare docs pass through with a stamped @timestamp
	bare := normalizeDataStreamDoc([]byte(`{"message":"plain"}`))
	m2 := util.MapStr{}
	if err := util.FromJSONBytes(bare, &m2); err != nil {
		t.Fatalf("bare doc: %v", err)
	}
	if m2["message"] != "plain" {
		t.Errorf("bare doc message lost: %v", m2)
	}
	if m2["@timestamp"] == nil {
		t.Error("bare doc missing stamped @timestamp")
	}
}

// ─── config validation ───────────────────────────────────────────────────────

func TestNewMergeValidation(t *testing.T) {
	cases := []struct {
		name   string
		cfg    map[string]interface{}
		errMsg string
	}{
		{"empty", map[string]interface{}{}, "exactly one of input_queue | input_lane"},
		{"both", map[string]interface{}{"input_queue": "q", "input_lane": "l"}, "exactly one of input_queue | input_lane"},
		{"no lane", map[string]interface{}{"input_queue": "q"}, "requires output_lane"},
		{"no index", map[string]interface{}{"input_queue": "q", "output_lane": "l"}, "requires index_name"},
	}
	for _, c := range cases {
		cfg, err := config.NewConfigFrom(c.cfg)
		if err != nil {
			t.Fatalf("%s: build config: %v", c.name, err)
		}
		_, err = NewMergeToBulkLane(cfg)
		if err == nil || !strings.Contains(err.Error(), c.errMsg) {
			t.Errorf("%s: err = %v, want contains %q", c.name, err, c.errMsg)
		}
	}
}

func TestNewSenderValidation(t *testing.T) {
	cfg, err := config.NewConfigFrom(map[string]interface{}{})
	if err != nil {
		t.Fatalf("build config: %v", err)
	}
	if _, err := NewBulkIndexingLane(cfg); err == nil || !strings.Contains(err.Error(), "requires lane") {
		t.Errorf("empty config: err = %v", err)
	}

	// failure_queue without max_retries is rejected
	cfg2, _ := config.NewConfigFrom(map[string]interface{}{
		"lane": "missing", "elasticsearch": "es", "failure_queue": "dead",
	})
	if _, err := NewBulkIndexingLane(cfg2); err == nil || !strings.Contains(err.Error(), "max_retries") {
		t.Errorf("failure queue without budget: err = %v", err)
	}

	// a missing lane is auto-created: the old "not found" rejection wedged
	// task restarts (consumer exits delete the lane; a restart racing its
	// producers had nothing to bind and the pipeline died)
	cfg3, _ := config.NewConfigFrom(map[string]interface{}{
		"lane": "test-sender-auto-lane", "elasticsearch": "es",
	})
	if _, err := NewBulkIndexingLane(cfg3); err != nil {
		t.Errorf("auto-create lane: err = %v", err)
	}
	if l := lane.Get("test-sender-auto-lane"); l == nil {
		t.Error("auto-created lane not registered")
	}
	_ = lane.Delete("test-sender-auto-lane", true)
}

// ─── merge end-to-end over a lane ────────────────────────────────────────────

func TestMergeToBulkLaneEndToEnd(t *testing.T) {
	installMemQueue()

	const laneName = "test-merge-lane"
	const queueName = "test-merge-queue"
	l, err := lane.Create(lane.LaneConfig{Name: laneName, Capacity: 16})
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}
	defer lane.Delete(laneName, true)

	qcfg := queue.GetOrInitConfig(queueName)
	docs := []string{
		`{"id":"1","message":"alpha"}`,
		`{"id":"2","message":"beta"}`,
		`{"id":"3","message":"gamma"}`,
	}
	for _, d := range docs {
		if err := queue.Push(qcfg, []byte(d)); err != nil {
			t.Fatalf("push: %v", err)
		}
	}

	cfg, err := config.NewConfigFrom(map[string]interface{}{
		"input_queue":             queueName,
		"output_lane":             laneName,
		"index_name":              "test-logs",
		"key_field":               "id",
		"idle_timeout_in_seconds": 1,
		"bulk_size_in_kb":         1,
	})
	if err != nil {
		t.Fatalf("build config: %v", err)
	}
	proc, err := NewMergeToBulkLane(cfg)
	if err != nil {
		t.Fatalf("new processor: %v", err)
	}

	stdCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pctx := &pipeline.Context{Context: stdCtx}

	type delivery struct {
		payload string
		traceID string
		codec   string
	}
	bulks := make(chan delivery, 8)
	l.SubscribeBatch(4, 50*time.Millisecond, func(recs []*event.Event) {
		for _, rec := range recs {
			if b, ok := bulkPayloadOf(rec); ok {
				bulks <- delivery{payload: string(b), traceID: pipeline.TraceID(rec), codec: pipeline.CodecHint(rec)}
			}
		}
	})

	processDone := make(chan error, 1)
	go func() { processDone <- proc.Process(pctx) }()

	var combined string
	var first delivery
	deadline := time.After(10 * time.Second)
collect:
	for {
		select {
		case d := <-bulks:
			if first.payload == "" {
				first = d
			}
			combined += d.payload
			if strings.Count(combined, `"message"`) >= len(docs) {
				break collect
			}
		case <-deadline:
			t.Fatalf("timed out waiting for merged bulk, got: %q", combined)
		}
	}

	// action lines target the right index and carry the key_field-derived ids
	if !strings.Contains(combined, `"_index" : "test-logs"`) {
		t.Errorf("action line missing index:\n%s", combined)
	}
	for _, id := range []string{`"_id":"1"`, `"_id":"2"`, `"_id":"3"`} {
		if !strings.Contains(combined, id) {
			t.Errorf("action line missing %s:\n%s", id, combined)
		}
	}
	// every doc body survived the merge
	for _, d := range docs {
		if !strings.Contains(combined, d) {
			t.Errorf("doc body missing from bulk:\n%s\nwant: %s", combined, d)
		}
	}
	// record metadata: codec hint routes spill encoding, trace id correlates
	if first.codec != "bulk" {
		t.Errorf("codec hint = %q, want bulk", first.codec)
	}
	if first.traceID == "" {
		t.Error("merged record missing trace id")
	}

	// cancellation stops the merge host
	cancel()
	select {
	case <-processDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Process did not return after cancel")
	}

	// nothing hit the salvage path: the failure queue stays empty
	if d := queue.Depth(queue.GetOrInitConfig(queueName + "_failure")); d != 0 {
		t.Errorf("failure queue depth = %d, want 0", d)
	}
}

// ─── consumer stop/restart cycle over the lane ──────────────────────────────

// TestBulkLaneSurvivesConsumerRestart pins the gateway wedge fix: when the
// sink task (bulk_indexing_lane) exits it DELETES the lane; the merge host
// must re-create it on the next flush and keep flowing (no "closed" emit
// errors, nothing salvaged), and a restarted consumer binds the fresh
// instance. Before the fix every flush after a sink-task stop salvaged to
// the failure queue with "lane closed" errors, forever.
func TestBulkLaneSurvivesConsumerRestart(t *testing.T) {
	installMemQueue()

	const laneName = "test-restart-lane"
	const queueName = "test-restart-queue"
	if _, err := lane.Create(lane.LaneConfig{Name: laneName, Capacity: 16}); err != nil {
		t.Fatalf("create lane: %v", err)
	}
	defer lane.Delete(laneName, true)

	qcfg := queue.GetOrInitConfig(queueName)

	cfg, err := config.NewConfigFrom(map[string]interface{}{
		"input_queue":             queueName,
		"output_lane":             laneName,
		"index_name":              "test-logs",
		"idle_timeout_in_seconds": 1,
		"bulk_size_in_kb":         1,
	})
	if err != nil {
		t.Fatalf("build config: %v", err)
	}
	proc, err := NewMergeToBulkLane(cfg)
	if err != nil {
		t.Fatalf("new processor: %v", err)
	}

	stdCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	processDone := make(chan error, 1)
	go func() { processDone <- proc.Process(&pipeline.Context{Context: stdCtx}) }()

	bulks := make(chan string, 8)
	subscribe := func(l lane.Lane) {
		l.SubscribeBatch(4, 50*time.Millisecond, func(recs []*event.Event) {
			for _, rec := range recs {
				if b, ok := bulkPayloadOf(rec); ok {
					bulks <- string(b)
				}
			}
		})
	}
	subscribe(lane.Get(laneName))

	// phase 1: normal flow through the first lane instance
	if err := queue.Push(qcfg, []byte(`{"id":"1","message":"before-restart"}`)); err != nil {
		t.Fatal(err)
	}
	waitBulk(t, bulks, "before-restart")

	// the sink task exiting: bulk_indexing_lane's Process deletes the lane
	// on ctx.Done — exactly this call
	if err := lane.Delete(laneName, true); err != nil {
		t.Fatalf("delete lane: %v", err)
	}

	// phase 2: the merge's next flush re-creates the lane and emits into it
	if err := queue.Push(qcfg, []byte(`{"id":"2","message":"after-restart"}`)); err != nil {
		t.Fatal(err)
	}

	// the "restarted consumer": wait for the producer to re-create the lane
	// instance, then bind it
	var fresh lane.Lane
	deadline := time.Now().Add(10 * time.Second)
	for fresh = lane.Get(laneName); fresh == nil && time.Now().Before(deadline); fresh = lane.Get(laneName) {
		time.Sleep(50 * time.Millisecond)
	}
	if fresh == nil {
		t.Fatal("lane was not re-created after consumer exit")
	}
	subscribe(fresh)
	waitBulk(t, bulks, "after-restart")

	cancel()
	select {
	case <-processDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Process did not return after cancel")
	}

	// nothing hit the salvage path — the wedge symptom was every flush
	// salvaging with "lane closed" errors
	if d := queue.Depth(queue.GetOrInitConfig(queueName + "_failure")); d != 0 {
		t.Errorf("failure queue depth = %d, want 0", d)
	}
}

func waitBulk(t *testing.T, bulks chan string, want string) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case b := <-bulks:
			if strings.Contains(b, want) {
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for bulk containing %q", want)
		}
	}
}

// ─── block-on-full backpressure ──────────────────────────────────────────────

func TestMergeToBulkLaneBlocksWhenLaneFull(t *testing.T) {
	installMemQueue()

	const laneName = "test-block-lane"
	const queueName = "test-block-queue"
	l, err := lane.Create(lane.LaneConfig{Name: laneName, Capacity: 1})
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}
	defer lane.Delete(laneName, true)

	// big bodies force frequent flushes: ~600B docs against a 1KB bulk
	// threshold yields a merged bulk roughly every two docs
	qcfg := queue.GetOrInitConfig(queueName)
	bigBody := strings.Repeat("x", 600)
	const docCount = 20
	for i := 0; i < docCount; i++ {
		doc := `{"id":"` + fmt.Sprint(i) + `","message":"` + bigBody + `"}`
		if err := queue.Push(qcfg, []byte(doc)); err != nil {
			t.Fatalf("push: %v", err)
		}
	}

	cfg, err := config.NewConfigFrom(map[string]interface{}{
		"input_queue":             queueName,
		"output_lane":             laneName,
		"index_name":              "test-logs",
		"idle_timeout_in_seconds": 1,
		"bulk_size_in_kb":         1,
	})
	if err != nil {
		t.Fatalf("build config: %v", err)
	}
	proc, err := NewMergeToBulkLane(cfg)
	if err != nil {
		t.Fatalf("new processor: %v", err)
	}

	stdCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pctx := &pipeline.Context{Context: stdCtx}

	// the sole subscriber holds its first delivery until released: the
	// lane (capacity 1) saturates and the merge must BLOCK in Emit
	release := make(chan struct{})
	var deliveries int32
	l.SubscribeBatch(1, 0, func(recs []*event.Event) {
		atomic.AddInt32(&deliveries, 1)
		<-release
	})

	processDone := make(chan error, 1)
	go func() { processDone <- proc.Process(pctx) }()

	// saturation point: one record held by the subscriber, one in the
	// channel, further emissions blocked — input queue consumption stalls
	saturated := waitFor(time.Second*20, func() bool {
		return atomic.LoadInt32(&deliveries) >= 1 && l.Depth() >= 1
	})
	if !saturated {
		t.Fatalf("lane did not saturate (deliveries=%d depth=%d)", atomic.LoadInt32(&deliveries), l.Depth())
	}
	time.Sleep(1500 * time.Millisecond) // idle window passes while blocked
	if d := queue.Depth(qcfg); d == 0 {
		t.Error("input queue drained while the lane was saturated — blocking backpressure not engaged")
	}

	// release the subscriber: flow resumes, the input queue drains, and
	// only then does cancellation stop the host
	close(release)
	if !waitFor(20*time.Second, func() bool { return queue.Depth(qcfg) == 0 }) {
		t.Fatalf("input queue did not drain after release, depth=%d", queue.Depth(qcfg))
	}
	cancel()
	select {
	case <-processDone:
	case <-time.After(15 * time.Second):
		t.Fatal("Process did not return after cancel")
	}
}

func waitFor(timeout time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return cond()
}

// ─── sender guard path ───────────────────────────────────────────────────────

func TestSenderSkipsPayloadlessRecord(t *testing.T) {
	p := &BulkIndexingLaneProcessor{}
	pctx := &pipeline.Context{Context: context.Background()}

	// a record without the bulk payload skips before any ES interaction
	// (meta stays nil): the guard must hold without panicking
	rec := &event.Event{Fields: util.MapStr{"something": "else"}}
	p.sendRecord(pctx, nil, rec)
}
