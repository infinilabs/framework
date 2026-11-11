/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/param"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
)

// ---- stage-wise test doubles ------------------------------------------------

// orderRecorder logs execution order across processors.
type orderRecorder struct {
	name  string
	order *[]string
}

func (p orderRecorder) Name() string { return p.name }
func (p orderRecorder) Process(ctx *pipeline.Context) error {
	*p.order = append(*p.order, p.name)
	return nil
}

// stageRecorder implements BatchProcessor: records the batch size and
// whether the field written by a preceding record processor is visible.
type stageBatchProc struct {
	name     string
	order    *[]string
	batchLen *[]int
	sawUpper *[]string
}

func (p *stageBatchProc) Name() string { return p.name }
func (p *stageBatchProc) Process(ctx *pipeline.Context) error {
	*p.order = append(*p.order, p.name)
	rec, _ := pipeline.CurrentRecord(ctx)
	if rec != nil {
		v, _ := rec.Fields.GetValue("upper")
		*p.sawUpper = append(*p.sawUpper, util.ToString(v))
	}
	return nil
}
func (p *stageBatchProc) ProcessBatch(ctx *pipeline.Context, records []*event.Event) error {
	*p.order = append(*p.order, p.name)
	for _, rec := range records {
		v, _ := rec.Fields.GetValue("upper")
		*p.sawUpper = append(*p.sawUpper, util.ToString(v))
	}
	*p.batchLen = append(*p.batchLen, len(records))
	return nil
}

// upperProc upper-cases the message field (a record-processor edit the
// later batch segment must observe).
type upperProc struct {
	name  string
	order *[]string
}

func (p upperProc) Name() string { return p.name }
func (p upperProc) Process(ctx *pipeline.Context) error {
	*p.order = append(*p.order, p.name)
	rec, ok := pipeline.CurrentRecord(ctx)
	if !ok {
		return nil
	}
	rec.Fields["upper"] = "yes"
	return nil
}

func TestForEach_StageWiseOrderIsWrittenOrder(t *testing.T) {
	var order []string
	var batchLen []int
	var sawUpper []string

	upper := upperProc{"upper", &order}
	batch := &stageBatchProc{"batch", &order, &batchLen, &sawUpper}
	lower := orderRecorder{"lower", &order}

	p := buildForEachCfg(t, util.MapStr{"processor": []interface{}{
		util.MapStr{"test_upper": util.MapStr{}},
	}})
	p.sub.List = []pipeline.Processor{upper, batch, lower}

	ctx := &pipeline.Context{Context: t.Context()}
	msgs := []queue.Message{otelMsg(`{"payload":{"message":"a"}}`), otelMsg(`{"payload":{"message":"b"}}`)}
	ctx.Set(param.ParaKey("messages"), msgs)

	if err := p.Process(ctx); err != nil {
		t.Fatal(err)
	}

	// execution order: upper(per record a,b) -> batch(once) -> lower(a,b)
	want := []string{"upper", "upper", "batch", "lower", "lower"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("execution order = %v, want %v", order, want)
	}
	if len(batchLen) != 1 || batchLen[0] != 2 {
		t.Fatalf("batch called %v with sizes %v, want once with 2", batchLen, batchLen)
	}
	// the batch segment must SEE the record processor's edits (the core
	// semantic the two-phase behavior broke)
	for _, saw := range sawUpper {
		if saw != "yes" {
			t.Fatalf("batch segment saw stale record state: %q", saw)
		}
	}
}

type dropFirstProc struct{ dropped *int }

func (p dropFirstProc) Name() string { return "test_drop_first" }
func (p dropFirstProc) Process(ctx *pipeline.Context) error {
	rec, _ := pipeline.CurrentRecord(ctx)
	if msg, _ := rec.Fields.GetValue("message"); util.ToString(msg) == "drop-me" {
		pipeline.MarkDropped(rec)
		*p.dropped++
	}
	return nil
}

func TestForEach_DropExcludesFromLaterSegments(t *testing.T) {
	var batchLen []int
	var sawUpper []string
	var order []string
	dropped := 0

	p := buildForEachCfg(t, util.MapStr{"processor": []interface{}{
		util.MapStr{"test_upper": util.MapStr{}},
	}})
	p.sub.List = []pipeline.Processor{
		dropFirstProc{&dropped},
		&stageBatchProc{"batch", &order, &batchLen, &sawUpper},
		orderRecorder{"lower", &order},
	}

	ctx := &pipeline.Context{Context: t.Context()}
	msgs := []queue.Message{
		otelMsg(`{"payload":{"message":"drop-me"}}`),
		otelMsg(`{"payload":{"message":"keep-me"}}`),
	}
	ctx.Set(param.ParaKey("messages"), msgs)

	if err := p.Process(ctx); err != nil {
		t.Fatal(err)
	}
	if dropped != 1 {
		t.Fatalf("dropped %d, want 1", dropped)
	}
	if batchLen[0] != 1 {
		t.Fatalf("batch saw %d records, want 1 (dropped must be excluded)", batchLen[0])
	}
	if len(msgs[0].Data) != 0 {
		t.Fatal("dropped record still encoded into message")
	}
	if len(msgs[1].Data) == 0 {
		t.Fatal("kept record lost")
	}
}

// cpuProc simulates CPU-bound per-record work for the parallel benchmark.
type cpuProc struct{ n int }

func (p *cpuProc) Name() string { return "bench_cpu" }
func (p *cpuProc) Process(ctx *pipeline.Context) error {
	rec, _ := pipeline.CurrentRecord(ctx)
	if rec == nil {
		return nil
	}
	h := sha256.New()
	for i := 0; i < p.n; i++ {
		h.Write([]byte("x"))
	}
	rec.Fields["digest"] = hex.EncodeToString(h.Sum(nil))[:8]
	return nil
}

func parallelForEach(b *testing.B, parallel int) {
	p := buildForEachCfgB(b, util.MapStr{
		"processor": []interface{}{util.MapStr{"test_upper": util.MapStr{}}},
		"parallel":  parallel,
	})
	p.sub.List = []pipeline.Processor{&cpuProc{2048}}
	msgs := make([]queue.Message, 64)
	for i := range msgs {
		msgs[i] = otelMsg(`{"payload":{"message":"m"}}`)
	}
	ctx := &pipeline.Context{Context: context.Background()}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.Set(param.ParaKey("messages"), msgs)
		if err := p.Process(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkForEach_Parallel1(b *testing.B) { parallelForEach(b, 1) }
func BenchmarkForEach_Parallel4(b *testing.B) { parallelForEach(b, 4) }
func BenchmarkForEach_Parallel8(b *testing.B) { parallelForEach(b, 8) }

func buildForEachCfgB(b *testing.B, cfgMap util.MapStr) *ForEachProcessor {
	b.Helper()
	c, err := config.NewConfigFrom(cfgMap)
	if err != nil {
		b.Fatal(err)
	}
	p, err := NewForEachProcessor(c)
	if err != nil {
		b.Fatalf("NewForEachProcessor: %v", err)
	}
	return p.(*ForEachProcessor)
}
