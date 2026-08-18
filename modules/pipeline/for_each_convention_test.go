/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"strings"
	"testing"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/param"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
)

// batchSampler counts records per batch (BatchProcessor fixture).
type batchSampler struct {
	batches  int
	perBatch int
	keep     int
}

func (b *batchSampler) Name() string { return "test_batch_sampler" }
func (b *batchSampler) Process(c *pipeline.Context) error { return nil }
func (b *batchSampler) ProcessBatch(c *pipeline.Context, records []*event.Event) error {
	b.batches++
	b.perBatch = len(records)
	// drop every other record via the marker convention (NOTE: do not
	// compact the slice in place — the caller owns the backing array)
	kept := 0
	for i, r := range records {
		if i%2 == 0 {
			kept++
		} else {
			pipeline.MarkDropped(r)
		}
	}
	b.keep = kept
	return nil
}

func init() {
	pipeline.RegisterProcessorPlugin("test_batch_sampler", func(*config.Config) (pipeline.Processor, error) {
		return &batchSampler{}, nil
	})
}

func buildForEachCfg(t *testing.T, cfgMap util.MapStr) *ForEachProcessor {
	t.Helper()
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

func otelMsg(body string) queue.Message {
	return queue.Message{Data: []byte(body), Size: len(body), Offset: queue.AcquireOffset(0, 0)}
}

func TestForEach_BatchProcessorPath(t *testing.T) {
	sam := &batchSampler{}
	p := buildForEachCfg(t, util.MapStr{
		"processor": []interface{}{util.MapStr{"test_upper": util.MapStr{}}},
	})
	// inject the batch processor into the compiled chain
	p.sub.List = append([]pipeline.Processor{sam}, p.sub.List...)

	ctx := &pipeline.Context{Context: t.Context()}
	var msgs []queue.Message
	for i := 0; i < 4; i++ {
		msgs = append(msgs, otelMsg(`{"payload":{"message":"m`+string(rune('0'+i))+`"}}`))
	}
	ctx.Set(param.ParaKey("messages"), msgs)

	if err := p.Process(ctx); err != nil {
		t.Fatal(err)
	}
	if sam.batches != 1 || sam.perBatch != 4 {
		t.Fatalf("batch processor ran %d times (want 1), saw %d records (want 4)", sam.batches, sam.perBatch)
	}
	// 2 of 4 dropped by the sampler
	kept := 0
	for _, m := range msgs {
		if len(m.Data) > 0 {
			kept++
		}
	}
	if kept != 2 {
		t.Fatalf("kept %d records, want 2 (batch-path drops honored)", kept)
	}
}

func TestForEach_OnFailureTag(t *testing.T) {
	// failing sub-processor
	pipeline.RegisterProcessorPlugin("test_always_fail", func(*config.Config) (pipeline.Processor, error) {
		return failProc{}, nil
	})
	p := buildForEachCfg(t, util.MapStr{
		"on_failure":  "tag",
		"failure_tag": "_my_tag",
		"processor":   []interface{}{util.MapStr{"test_always_fail": util.MapStr{}}},
	})

	var captured []string
	pipeline.RegisterProcessorPlugin("test_capture_tags", func(*config.Config) (pipeline.Processor, error) {
		return tagCapturer{&captured}, nil
	})
	p.sub.List = append(p.sub.List, tagCapturer{&captured})

	ctx := &pipeline.Context{Context: t.Context()}
	msgs := []queue.Message{otelMsg(`{"payload":{"message":"x"}}`)}
	ctx.Set(param.ParaKey("messages"), msgs)

	if err := p.Process(ctx); err != nil {
		t.Fatalf("tag strategy must not fail the batch: %v", err)
	}
	if len(captured) == 0 || !contains(captured[0], "_my_tag") {
		t.Fatalf("failure tag not visible downstream: %v", captured)
	}
	// record still present (tagged, not dropped)
	if len(msgs[0].Data) == 0 {
		t.Fatal("tagged record must be kept")
	}
}

func TestForEach_OnFailureFail(t *testing.T) {
	p := buildForEachCfg(t, util.MapStr{
		"on_failure": "fail",
		"processor":  []interface{}{util.MapStr{"test_always_fail": util.MapStr{}}},
	})
	ctx := &pipeline.Context{Context: t.Context()}
	msgs := []queue.Message{otelMsg(`{"payload":{"message":"x"}}`)}
	ctx.Set(param.ParaKey("messages"), msgs)

	if err := p.Process(ctx); err == nil {
		t.Fatal("fail strategy must abort the batch")
	}
}

func TestForEach_OnFailureInvalidRejected(t *testing.T) {
	c, _ := config.NewConfigFrom(util.MapStr{
		"on_failure": "explode",
		"processor":  []interface{}{util.MapStr{"test_upper": util.MapStr{}}},
	})
	if _, err := NewForEachProcessor(c); err == nil {
		t.Fatal("invalid on_failure must be rejected at construction")
	}
}

func TestForEach_Compat_PlainProcessorsUnchanged(t *testing.T) {
	// Regression: the historical path (no on_failure, plain processors,
	// no batch processors) behaves exactly as before the convention work.
	p := buildForEachCfg(t, util.MapStr{
		"processor": []interface{}{util.MapStr{"test_upper": util.MapStr{}}},
	})
	ctx := &pipeline.Context{Context: t.Context()}
	msgs := []queue.Message{otelMsg(`{"payload":{"message":"compat"}}`)}
	ctx.Set(param.ParaKey("messages"), msgs)

	if err := p.Process(ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(msgs[0].Data), "PROCESSED:compat") {
		t.Fatalf("plain per-record path broken: %s", msgs[0].Data)
	}
}

func TestForEach_UndecodablePassthrough(t *testing.T) {
	// Mixed batch: one decodable, one not — the bad payload must pass
	// through untouched rather than being dropped.
	p := buildForEachCfg(t, util.MapStr{
		"processor": []interface{}{util.MapStr{"test_upper": util.MapStr{}}},
	})
	ctx := &pipeline.Context{Context: t.Context()}
	bad := queue.Message{Data: []byte("not-json"), Size: 8, Offset: queue.AcquireOffset(0, 1)}
	msgs := []queue.Message{otelMsg(`{"payload":{"message":"good"}}`), bad}
	ctx.Set(param.ParaKey("messages"), msgs)

	if err := p.Process(ctx); err != nil {
		t.Fatal(err)
	}
	if string(msgs[1].Data) != "not-json" {
		t.Fatalf("undecodable payload must pass through unchanged, got %q", msgs[1].Data)
	}
}

type failProc struct{}

func (failProc) Name() string                       { return "test_always_fail" }
func (failProc) Process(c *pipeline.Context) error { return errBoom }

var errBoom = &simpleErr{}

type simpleErr struct{}

func (*simpleErr) Error() string { return "boom" }

type tagCapturer struct{ out *[]string }

func (tagCapturer) Name() string                      { return "test_capture_tags" }
func (t tagCapturer) Process(c *pipeline.Context) error {
	if tags, ok := c.Get(pipeline.FailureTagsKey).(*[]string); ok && tags != nil {
		*t.out = append(*t.out, strings.Join(*tags, ","))
	}
	return nil
}
