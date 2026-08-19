/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"testing"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/param"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
)

// upperCaser is a test sub-chain processor that mutates the record.
type upperCaser struct{}

func (upperCaser) Name() string { return "test_upper" }
func (upperCaser) Process(c *pipeline.Context) error {
	rec, _ := pipeline.CurrentRecord(c)
	if rec == nil {
		return nil
	}
	if msg, ok := rec.Fields["message"].(string); ok {
		rec.Fields["message"] = "PROCESSED:" + msg
	}
	return nil
}

func init() {
	pipeline.RegisterProcessorPlugin("test_upper", func(*config.Config) (pipeline.Processor, error) { return upperCaser{}, nil })
}

func buildForEach(t *testing.T, extra map[string]interface{}) *ForEachProcessor {
	t.Helper()
	cfgMap := util.MapStr{
		"processor": []interface{}{util.MapStr{"test_upper": util.MapStr{}}},
	}
	for k, v := range extra {
		cfgMap[k] = v
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

func TestForEach_OtelCodecRoundTrip(t *testing.T) {
	// envelope JSON compatible with the agent LogEvent shape
	body := `{"metadata":{"log_type":"text"},"payload":{"message":"hello"},"timestamp":"2026-08-18T10:00:00Z"}`

	p := buildForEach(t, nil)
	ctx := &pipeline.Context{Context: t.Context()}
	msgs := []queue.Message{{Data: []byte(body), Size: len(body), Offset: queue.AcquireOffset(0, 1)}}
	ctx.Set(param.ParaKey("messages"), msgs)

	if err := p.Process(ctx); err != nil {
		t.Fatal(err)
	}
	if got := string(msgs[0].Data); got == body {
		t.Fatal("payload should have been re-encoded after processing")
	}
	if !contains(string(msgs[0].Data), "PROCESSED:hello") {
		t.Fatalf("record not transformed: %s", msgs[0].Data)
	}
}

func TestForEach_UnknownCodecRejected(t *testing.T) {
	c, err := config.NewConfigFrom(util.MapStr{
		"codec":     "nope",
		"processor": []interface{}{util.MapStr{"test_upper": util.MapStr{}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewForEachProcessor(c); err == nil {
		t.Fatal("unknown codec must be rejected at construction")
	}
}

func TestForEach_CustomCodec(t *testing.T) {
	RegisterRecordCodec(rawCodec{})
	p := buildForEach(t, map[string]interface{}{"codec": "raw"})
	ctx := &pipeline.Context{Context: t.Context()}
	msgs := []queue.Message{{Data: []byte("raw-bytes"), Size: 9, Offset: queue.AcquireOffset(0, 7)}}
	ctx.Set(param.ParaKey("messages"), msgs)

	if err := p.Process(ctx); err != nil {
		t.Fatal(err)
	}
	if !contains(string(msgs[0].Data), "PROCESSED:raw-bytes") {
		t.Fatalf("custom codec round-trip failed: %s", msgs[0].Data)
	}
}

func TestForEach_EmptyBatchNoop(t *testing.T) {
	p := buildForEach(t, nil)
	ctx := &pipeline.Context{Context: t.Context()} // no messages key
	if err := p.Process(ctx); err != nil {
		t.Fatal(err)
	}
}

// The record convention is only valid inside the sub-chain: after Process
// returns, the shared context must not hand out the batch's last record to
// downstream processors.
func TestForEach_RecordClearedAfterProcess(t *testing.T) {
	body := `{"payload":{"message":"x"},"timestamp":"2026-08-18T10:00:00Z"}`
	p := buildForEach(t, nil)
	ctx := &pipeline.Context{Context: t.Context()}
	msgs := []queue.Message{{Data: []byte(body), Size: len(body), Offset: queue.AcquireOffset(0, 1)}}
	ctx.Set(param.ParaKey("messages"), msgs)

	if err := p.Process(ctx); err != nil {
		t.Fatal(err)
	}
	if rec, ok := pipeline.CurrentRecord(ctx); ok {
		t.Fatalf("record leaked out of for_each: %v", rec)
	}
}

// rawCodec decodes arbitrary bytes as message-only events (test fixture).
type rawCodec struct{}

func (rawCodec) Name() string { return "raw" }
func (rawCodec) Decode(data []byte) (*event.Event, error) {
	e := &event.Event{Fields: util.MapStr{"message": string(data)}}
	return e, nil
}
func (rawCodec) Encode(rec *event.Event) ([]byte, error) {
	msg, _ := rec.Fields["message"].(string)
	return []byte(msg), nil
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
