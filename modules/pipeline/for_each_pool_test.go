/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infini.ltd */

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

// pooledRawCodec is rawCodec with the record coming from the event pool —
// the production shape of coco's document/attachment codecs.
type pooledRawCodec struct{}

func (pooledRawCodec) Name() string      { return "raw_pooled" }
func (pooledRawCodec) Namespace() string { return "" }
func (pooledRawCodec) Decode(data []byte) (*event.Event, error) {
	e := event.AcquireEvent()
	e.Fields = util.MapStr{"message": string(data)}
	return e, nil
}
func (pooledRawCodec) Encode(rec *event.Event) ([]byte, error) {
	msg, _ := rec.Fields["message"].(string)
	return []byte(msg), nil
}

// dropper marks every record dropped so the wiring test covers the drop
// path — dropped records must be released too.
type dropper struct{}

func (dropper) Name() string { return "test_dropper" }
func (dropper) Process(c *pipeline.Context) error {
	if rec, ok := pipeline.CurrentRecord(c); ok {
		pipeline.MarkDropped(rec)
	}
	return nil
}

func init() {
	pipeline.RegisterProcessorPlugin("test_dropper", func(*config.Config) (pipeline.Processor, error) { return dropper{}, nil })
}

// TestForEach_PooledRecordsReleased: every pooled record must be back in
// the pool by the time Process returns — originals, transformed records
// and dropped records alike (leak detection gate for the pool wiring).
func TestForEach_PooledRecordsReleased(t *testing.T) {
	if err := RegisterRecordCodecOrReplace(pooledRawCodec{}); err != nil {
		t.Fatal(err)
	}
	p := buildForEach(t, map[string]interface{}{"codec": "raw_pooled"})

	_, r0 := event.PoolStats()
	for run := 0; run < 3; run++ {
		msgs := []queue.Message{
			{Data: []byte("keep-1"), Size: 6, Offset: queue.AcquireOffset(0, 1)},
			{Data: []byte("keep-2"), Size: 6, Offset: queue.AcquireOffset(0, 2)},
		}
		ctx := &pipeline.Context{Context: t.Context()}
		ctx.Set(param.ParaKey("messages"), msgs)
		if err := p.Process(ctx); err != nil {
			t.Fatal(err)
		}
		if string(msgs[0].Data) != "PROCESSED:keep-1" {
			t.Fatalf("record not transformed: %s", msgs[0].Data)
		}
	}
	_, r1 := event.PoolStats()
	if got := r1 - r0; got != 6 {
		t.Fatalf("all 6 pooled records must be released, got %d", got)
	}
}

func TestForEach_PooledDroppedRecordsReleased(t *testing.T) {
	if err := RegisterRecordCodecOrReplace(pooledRawCodec{}); err != nil {
		t.Fatal(err)
	}
	c, err := config.NewConfigFrom(util.MapStr{
		"codec":     "raw_pooled",
		"processor": []interface{}{util.MapStr{"test_dropper": util.MapStr{}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := NewForEachProcessor(c)
	if err != nil {
		t.Fatal(err)
	}

	_, r0 := event.PoolStats()
	msgs := []queue.Message{{Data: []byte("gone"), Size: 4, Offset: queue.AcquireOffset(0, 1)}}
	ctx := &pipeline.Context{Context: t.Context()}
	ctx.Set(param.ParaKey("messages"), msgs)
	if err := p.(*ForEachProcessor).Process(ctx); err != nil {
		t.Fatal(err)
	}
	if msgs[0].Data != nil || msgs[0].Size != 0 {
		t.Fatalf("dropped record must leave an empty slot: %q", msgs[0].Data)
	}
	_, r1 := event.PoolStats()
	if got := r1 - r0; got != 1 {
		t.Fatalf("dropped pooled record must be released, got %d", got)
	}
}
