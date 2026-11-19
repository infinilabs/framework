/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"context"
	"sync"
	"testing"
	"time"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/lane"
	"infini.sh/framework/core/param"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
)

// ── test fixtures ──────────────────────────────────────────────────────────

// writingEncodeCodec simulates codecs whose Encode mutates the record
// (documentCodec stamps @timestamp): its Encode writes into Fields before
// serializing. Under the single-owner rule this is legal exactly once per
// owner — the host must not encode a record it has already delivered, or
// its write races with the receiver's.
type writingEncodeCodec struct{}

func (writingEncodeCodec) Name() string      { return "test_write_encode" }
func (writingEncodeCodec) Namespace() string { return "" }

func (writingEncodeCodec) Decode(data []byte) (*event.Event, error) {
	e := &event.Event{Fields: util.MapStr{}, Metadata: event.EventMetadata{Labels: util.MapStr{pipeline.CodecLabel: "test_write_encode"}}}
	_ = util.FromJSONBytes(data, &e.Fields)
	if e.Fields == nil {
		e.Fields = util.MapStr{}
	}
	e.Timestamp = time.Now().UTC()
	return e, nil
}

func (writingEncodeCodec) Encode(rec *event.Event) ([]byte, error) {
	rec.Fields["_encoded_at"] = time.Now().UnixNano() // the racing write
	return util.ToJSONBytes(rec.Fields)
}

func init() {
	pipeline.MustRegisterRecordCodec(writingEncodeCodec{})
}

// laneSink subscribes to a lane and collects the delivered records.
type laneSink struct {
	recs []*event.Event
	done chan struct{}
	// onRecord, when set, runs inside the single subscriber callback —
	// exercises receiver-side work (e.g. a writing Encode) without adding
	// a second competing subscriber that would steal deliveries.
	onRecord func(*event.Event)
}

func subscribe(t *testing.T, name string) *laneSink {
	t.Helper()
	s := &laneSink{done: make(chan struct{}, 64)}
	l := lane.GetOrCreate(lane.LaneConfig{Name: name})
	cancel := l.Subscribe(func(rec *event.Event) {
		if s.onRecord != nil {
			s.onRecord(rec)
		}
		s.recs = append(s.recs, rec)
		s.done <- struct{}{}
	})
	t.Cleanup(cancel)
	// lanes are process-global — a leaked lane surfaces as a carrier row
	// in the metrics-report sweep (laneStageStats walks lane.List())
	t.Cleanup(func() { _ = lane.Delete(name, true) })
	return s
}

func (s *laneSink) wait(t *testing.T, n int) []*event.Event {
	t.Helper()
	for i := 0; i < n; i++ {
		select {
		case <-s.done:
		case <-time.After(2 * time.Second):
			t.Fatalf("lane received only %d of %d records", len(s.recs), n)
		}
	}
	return s.recs
}

func forEachWith(t *testing.T, codec string, chain []interface{}) *ForEachProcessor {
	t.Helper()
	cfgMap := util.MapStr{"processor": chain}
	if codec != "" {
		cfgMap["codec"] = codec
	}
	c, err := config.NewConfigFrom(cfgMap)
	if err != nil {
		t.Fatal(err)
	}
	p, err := NewForEachProcessor(c)
	if err != nil {
		t.Fatalf("NewForEachProcessor: %v", err)
	}
	return p.(*ForEachProcessor)
}

func runForEach(t *testing.T, p *ForEachProcessor, bodies ...string) []queue.Message {
	t.Helper()
	msgs := make([]queue.Message, 0, len(bodies))
	for i, b := range bodies {
		msgs = append(msgs, queue.Message{Data: []byte(b), Size: len(b), Offset: queue.AcquireOffset(int64(i), int64(i)+1)})
	}
	ctx := &pipeline.Context{Context: t.Context()}
	ctx.Set(param.ParaKey("messages"), msgs)
	if err := p.Process(ctx); err != nil {
		t.Fatal(err)
	}
	out, _ := ctx.Get(param.ParaKey("messages")).([]queue.Message)
	return out
}

// ── delivery is terminal: successors skip, encode-back empties the slot ────

func TestDelivery_SuccessorsAndEncodeBackSkipped(t *testing.T) {
	sink := subscribe(t, t.Name()+"-lane")
	// output{lane} followed by a mutating processor in the SAME segment:
	// the successor must not run on the delivered record, and the host's
	// encode-back must empty the message slot (single delivery).
	p := forEachWith(t, "", []interface{}{
		util.MapStr{"output": util.MapStr{"lane": t.Name() + "-lane"}},
		util.MapStr{"test_upper": util.MapStr{}},
	})
	msgs := runForEach(t, p, `{"payload":{"message":"x"},"timestamp":"2026-08-18T10:00:00Z"}`)

	recs := sink.wait(t, 1)
	if got, _ := recs[0].Fields["message"]; got != "x" {
		t.Fatalf("successor ran on delivered record: message=%v", got)
	}
	if msgs[0].Data != nil {
		t.Fatalf("encode-back must empty the delivered record's slot, got %d bytes", len(msgs[0].Data))
	}
}

// The race that motivated the single-owner rule, as a regression gate:
// the host encodes each record after the sub-chain; a codec whose Encode
// writes (documentCodec stamps @timestamp) made that write race with the
// lane subscriber encoding the same record. With delivery-terminal
// semantics the host never touches a delivered record — -race stays quiet.
// Run with `go test -race` (CI default).
func TestDelivery_EncodeAfterEmitNoRace(t *testing.T) {
	sink := subscribe(t, t.Name()+"-lane")
	// the subscriber encodes with the same writing codec — the receiver
	// owns the record, its Encode write is legal
	sink.onRecord = func(rec *event.Event) {
		_, _ = writingEncodeCodec{}.Encode(rec)
	}

	p := forEachWith(t, "test_write_encode", []interface{}{
		util.MapStr{"output": util.MapStr{"lane": t.Name() + "-lane"}},
	})
	// enough records to overlap the subscriber with the host's post-emit
	// phase under scheduling jitter
	bodies := make([]string, 0, 64)
	for i := 0; i < 64; i++ {
		bodies = append(bodies, `{"payload":{"message":"m"},"timestamp":"2026-08-18T10:00:00Z"}`)
	}
	msgs := runForEach(t, p, bodies...)
	sink.wait(t, len(bodies))
	for i := range msgs {
		if msgs[i].Data != nil {
			t.Fatalf("record %d: encode-back must not run for delivered records", i)
		}
	}
}

// A second sink in the same chain is skipped with a warning — single
// delivery per record; fan out via lanes: [...] instead.
func TestDelivery_SecondSinkSkipped(t *testing.T) {
	a := subscribe(t, t.Name()+"-a")
	b := subscribe(t, t.Name()+"-b")
	p := forEachWith(t, "", []interface{}{
		util.MapStr{"output": util.MapStr{"lane": t.Name() + "-a"}},
		util.MapStr{"output": util.MapStr{"lane": t.Name() + "-b"}},
	})
	runForEach(t, p, `{"payload":{"message":"x"},"timestamp":"2026-08-18T10:00:00Z"}`)

	a.wait(t, 1)
	select {
	case <-b.done:
		t.Fatal("second sink must not receive the delivered record")
	case <-time.After(100 * time.Millisecond):
	}
}

// lanes: [a, b] fans out: the first lane receives the record itself, every
// further lane an independent copy — receivers own their records and never
// observe each other's mutations.
func TestDelivery_LanesFanOut(t *testing.T) {
	a := subscribe(t, t.Name()+"-a")
	b := subscribe(t, t.Name()+"-b")
	p := forEachWith(t, "", []interface{}{
		util.MapStr{"output": util.MapStr{"lanes": []string{t.Name() + "-a", t.Name() + "-b"}}},
	})
	runForEach(t, p, `{"payload":{"message":"x"},"timestamp":"2026-08-18T10:00:00Z"}`)

	recA := a.wait(t, 1)[0]
	recB := b.wait(t, 1)[0]

	// receiver-side ownership: A's mutation must not leak into B's record
	recA.Fields["touched_by_a"] = true
	if _, ok := recB.Fields["touched_by_a"]; ok {
		t.Fatal("fan-out receivers must hold independent records")
	}
}

// Pre-existing quirk fixed as a side effect of the runner guard: a
// processor chain [drop, mutate] used to run the mutate on the dropped
// record; the chain runner now stops at the first terminal state.
func TestDelivery_ChainRunnerStopsAfterDrop(t *testing.T) {
	p := forEachWith(t, "", []interface{}{
		util.MapStr{"test_dropper": util.MapStr{}},
		util.MapStr{"test_upper": util.MapStr{}},
	})
	msgs := runForEach(t, p, `{"payload":{"message":"x"},"timestamp":"2026-08-18T10:00:00Z"}`)
	if msgs[0].Data != nil {
		t.Fatalf("dropped record's slot must be empty, got %d bytes", len(msgs[0].Data))
	}
}

// lane→lane relay (lane_consumer sub-chain re-emitting to another lane):
// the delivered record is excluded from the consumer's remaining segments
// and its queue push — single delivery across the chain.
func TestDelivery_LaneToLaneSingleDelivery(t *testing.T) {
	relayOut := subscribe(t, t.Name()+"-out")

	cfgMap := util.MapStr{
		"lane":      t.Name() + "-in",
		"batch_max": 1,
		"processor": []interface{}{
			util.MapStr{"output": util.MapStr{"lane": t.Name() + "-out"}},
			util.MapStr{"test_upper": util.MapStr{}},
		},
	}
	c2, err := config.NewConfigFrom(cfgMap)
	if err != nil {
		t.Fatal(err)
	}
	proc, err := NewLaneConsumerProcessor(c2)
	if err != nil {
		t.Fatal(err)
	}
	lc := proc.(*LaneConsumerProcessor)
	t.Cleanup(func() { _ = lane.Delete(t.Name()+"-in", true) })

	ctx := &pipeline.Context{Context: t.Context()}
	rec := &event.Event{Fields: util.MapStr{"message": "relay"}}
	if err := lc.runBatch(ctx, buildSegments(lc.sub.List), []*event.Event{rec}); err != nil {
		t.Fatal(err)
	}

	relayRecs := relayOut.wait(t, 1)
	if got := relayRecs[0].Fields["message"]; got != "relay" {
		t.Fatalf("successor ran on delivered record: %v", got)
	}
}

// A record can arrive at lane_consumer carrying a stale record-borne
// terminal marker from a foreign emitter. Intake is an ownership transfer
// — the receiving chain must process the record, not skip it as "already
// left".
func TestDelivery_LaneConsumerIntakeTakesOwnership(t *testing.T) {
	cfgMap := util.MapStr{
		"lane":      t.Name() + "-lane",
		"processor": []interface{}{util.MapStr{"test_upper": util.MapStr{}}},
	}
	c, err := config.NewConfigFrom(cfgMap)
	if err != nil {
		t.Fatal(err)
	}
	proc, err := NewLaneConsumerProcessor(c)
	if err != nil {
		t.Fatal(err)
	}
	lc := proc.(*LaneConsumerProcessor)
	t.Cleanup(func() { _ = lane.Delete(t.Name()+"-lane", true) })

	ctx := &pipeline.Context{Context: t.Context()}
	rec := &event.Event{Fields: util.MapStr{"message": "x"}}
	pipeline.MarkDropped(rec) // a foreign emitter's stale record-borne marker

	if err := lc.runBatch(ctx, buildSegments(lc.sub.List), []*event.Event{rec}); err != nil {
		t.Fatal(err)
	}
	if got := rec.Fields["message"]; got != "PROCESSED:x" {
		t.Fatalf("upstream-marked record must be processed by the receiving chain, got %v", got)
	}
	if pipeline.IsDropped(rec) {
		t.Fatal("intake must clear the upstream marker: the record starts a fresh lifecycle here")
	}
}

// End-to-end ownership hop: for_each{output{lane}} delivers marked
// records; the lane_consumer's chain processes them and re-delivers to a
// second lane. Without the intake reset the consumer would skip every
// record and the second lane would never receive anything.
func TestDelivery_LaneToLaneChainProcessesDelivered(t *testing.T) {
	out := subscribe(t, t.Name()+"-out")

	lcCfg := util.MapStr{
		"lane":           t.Name() + "-in",
		"batch_max":      1,
		"batch_flush_ms": 5,
		"processor": []interface{}{
			util.MapStr{"test_upper": util.MapStr{}},
			util.MapStr{"output": util.MapStr{"lane": t.Name() + "-out"}},
		},
	}
	c2, err := config.NewConfigFrom(lcCfg)
	if err != nil {
		t.Fatal(err)
	}
	proc, err := NewLaneConsumerProcessor(c2)
	if err != nil {
		t.Fatal(err)
	}
	lc := proc.(*LaneConsumerProcessor)
	t.Cleanup(func() { _ = lane.Delete(t.Name()+"-in", true) })

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	go func() { _ = lc.Process(&pipeline.Context{Context: ctx}) }()

	p := forEachWith(t, "", []interface{}{
		util.MapStr{"output": util.MapStr{"lane": t.Name() + "-in"}},
	})
	runForEach(t, p, `{"payload":{"message":"hop"},"timestamp":"2026-08-18T10:00:00Z"}`)

	recs := out.wait(t, 1)
	if got := recs[0].Fields["message"]; got != "PROCESSED:hop" {
		t.Fatalf("receiving chain must process the upstream-delivered record, got %v", got)
	}
}

// captureObserver collects StepEvents for one test.
type captureObserver struct {
	mu     sync.Mutex
	events []pipeline.StepEvent
}

func (o *captureObserver) OnStep(e *pipeline.StepEvent) {
	o.mu.Lock()
	o.events = append(o.events, *e)
	o.mu.Unlock()
}

// Delivery surfaces in the step stream: the output step's outcome is
// "delivered" (terminal success — the flight recorder retains non-ok
// outcomes, stats rows show it as last_outcome), and a successor
// processor emits NO step event at all: it never ran, the single-owner
// guard skipped it (visible via the total_skipped counter instead).
func TestDelivery_StepOutcomeDelivered(t *testing.T) {
	sink := subscribe(t, t.Name()+"-lane")

	obs := &captureObserver{}
	pipeline.RegisterObserver(obs)
	t.Cleanup(func() { pipeline.UnregisterObserver(obs) })

	p := forEachWith(t, "", []interface{}{
		util.MapStr{"output": util.MapStr{"lane": t.Name() + "-lane"}},
		util.MapStr{"test_upper": util.MapStr{}},
	})
	runForEach(t, p, `{"payload":{"message":"x"},"timestamp":"2026-08-18T10:00:00Z"}`)
	sink.wait(t, 1)

	obs.mu.Lock()
	defer obs.mu.Unlock()
	delivered := 0
	for _, e := range obs.events {
		switch e.Processor {
		case "output":
			if e.Outcome != pipeline.OutcomeDelivered {
				t.Fatalf("output step outcome = %q, want %q", e.Outcome, pipeline.OutcomeDelivered)
			}
			delivered++
		case "test_upper":
			t.Fatal("skipped successor must not emit a step event — it never ran")
		}
	}
	if delivered == 0 {
		t.Fatal("no step event observed for the output processor")
	}
}

// A failed Emit keeps sender ownership: the record is NOT marked
// delivered, stays in its batch slot, and the error path decides — a
// record the sink never accepted must not vanish from the flow.
func TestDelivery_FailedEmitKeepsRecord(t *testing.T) {
	// capacity 1 and no subscriber: the first Emit fills the channel, the
	// output step's Emit blocks; closing the lane mid-block fails it —
	// the deterministic path to a failed single-target send
	l := lane.GetOrCreate(lane.LaneConfig{Name: t.Name() + "-lane", Capacity: 1})
	if err := l.Emit(&event.Event{Fields: util.MapStr{"filler": true}}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lane.Delete(t.Name()+"-lane", true) })

	p := forEachWith(t, "", []interface{}{
		util.MapStr{"output": util.MapStr{"lane": t.Name() + "-lane"}},
	})
	var msgs []queue.Message
	done := make(chan struct{})
	go func() {
		defer close(done)
		msgs = runForEach(t, p, `{"payload":{"message":"x"},"timestamp":"2026-08-18T10:00:00Z"}`)
	}()
	// let the output step reach the blocking Emit, then fail it
	time.Sleep(10 * time.Millisecond)
	_ = l.Close()
	<-done

	if msgs[0].Data == nil {
		t.Fatal("record lost: a failed Emit must keep the record in its slot")
	}
}
