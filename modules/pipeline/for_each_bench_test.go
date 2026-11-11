/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infini.ltd */

package pipeline

import (
	"encoding/json"
	"hash/fnv"
	"testing"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/param"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
)

// P4 benchmark suite (#75): record-mode data movement through for_each.
//
//	BenchmarkForEach_1MB_K4_*   — 1MB document tree, K=4 record processors:
//	                              pooled vs fresh allocation (the pool gate)
//	BenchmarkForEach_1KB_Parall — 1KB events, parallel 1 vs 4 (the ≥3×
//	                              throughput gate on CPU-shaped chains)
//
// The codec decodes the whole JSON tree into the record Fields — the
// document-codec shape — so the numbers track the real data-movement cost,
// not framework scaffolding.

// benchTreeCodec decodes a JSON envelope into the record fields tree.
type benchTreeCodec struct{ pooled bool }

func (benchTreeCodec) Name() string      { return "bench_tree" }
func (benchTreeCodec) Namespace() string { return "" }
func (c benchTreeCodec) Decode(data []byte) (*event.Event, error) {
	var tree util.MapStr
	if err := json.Unmarshal(data, &tree); err != nil {
		return nil, err
	}
	var e *event.Event
	if c.pooled {
		e = event.AcquireEvent()
	} else {
		e = &event.Event{}
	}
	e.Fields = tree
	return e, nil
}
func (benchTreeCodec) Encode(rec *event.Event) ([]byte, error) {
	return json.Marshal(rec.Fields)
}

// benchHasher burns CPU proportional to the content — the CPU-shaped
// chain the parallel gate measures (a no-op chain scales nothing).
type benchHasher struct{}

func (benchHasher) Name() string { return "test_bench_hasher" }
func (benchHasher) Process(c *pipeline.Context) error {
	rec, _ := pipeline.CurrentRecord(c)
	if rec == nil {
		return nil
	}
	doc, _ := rec.Fields["document"].(map[string]interface{})
	if doc == nil {
		return nil
	}
	content, _ := doc["content"].(string)
	h := fnv.New64a()
	for i := 0; i < 256; i++ { // burn enough CPU that the chain, not the codec, dominates
		h.Reset()
		h.Write([]byte(content))
	}
	doc["hash"] = h.Sum64()
	return nil
}

// benchPayload builds a document-shaped payload of roughly size bytes.
func benchPayload(size int) []byte {
	content := make([]byte, 0, size)
	unit := []byte("The quick brown fox jumps over the lazy dog. ")
	for len(content) < size {
		content = append(content, unit...)
	}
	body := map[string]interface{}{
		"document": map[string]interface{}{
			"id":      "bench-doc",
			"title":   "Benchmark Document",
			"content": string(content[:size]),
			"source":  map[string]interface{}{"id": "bench-src", "type": "test"},
		},
	}
	b, _ := json.Marshal(body)
	return b
}

func init() {
	pipeline.RegisterProcessorPlugin("test_bench_hasher", func(*config.Config) (pipeline.Processor, error) { return benchHasher{}, nil })
}

func buildForBench(b *testing.B, extra map[string]interface{}) *ForEachProcessor {
	b.Helper()
	cfgMap := util.MapStr{
		"codec": "bench_tree",
		"processor": []interface{}{
			util.MapStr{benchProc: util.MapStr{}},
			util.MapStr{benchProc: util.MapStr{}},
			util.MapStr{benchProc: util.MapStr{}},
			util.MapStr{benchProc: util.MapStr{}},
		},
	}
	for k, v := range extra {
		cfgMap[k] = v
	}
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

func runForEachBench(b *testing.B, p *ForEachProcessor, payload []byte, parallel int) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msgs := []queue.Message{{Data: payload, Size: len(payload), Offset: queue.AcquireOffset(0, int64(i))}}
		ctx := &pipeline.Context{Context: b.Context()}
		ctx.Set(param.ParaKey("messages"), msgs)
		if err := p.Process(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkForEach_1MB_K4_Pooled(b *testing.B) {
	if err := RegisterRecordCodecOrReplace(benchTreeCodec{pooled: true}); err != nil {
		b.Fatal(err)
	}
	runForEachBench(b, buildForBench(b, nil), benchPayload(1<<20), 1)
}

func BenchmarkForEach_1MB_K4_Fresh(b *testing.B) {
	if err := RegisterRecordCodecOrReplace(benchTreeCodec{pooled: false}); err != nil {
		b.Fatal(err)
	}
	runForEachBench(b, buildForBench(b, nil), benchPayload(1<<20), 1)
}

// benchProc selects the K=4 chain shape: the data-movement benches use the
// cheap mutator, the parallel gates use the CPU burner.
var benchProc = "test_upper"

// runForEachBatchBench drives a 32-record batch — parallel workers need
// records to steal; a single-record batch cannot scale.
func runForEachBatchBench(b *testing.B, p *ForEachProcessor, payload []byte) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msgs := make([]queue.Message, 0, 32)
		for j := 0; j < 32; j++ {
			msgs = append(msgs, queue.Message{Data: payload, Size: len(payload), Offset: queue.AcquireOffset(0, int64(i*32+j))})
		}
		ctx := &pipeline.Context{Context: b.Context()}
		ctx.Set(param.ParaKey("messages"), msgs)
		if err := p.Process(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkForEach_1KB_CPUChain_Parallel1(b *testing.B) {
	if err := RegisterRecordCodecOrReplace(benchTreeCodec{pooled: true}); err != nil {
		b.Fatal(err)
	}
	benchProc = "test_bench_hasher"
	defer func() { benchProc = "test_upper" }()
	runForEachBatchBench(b, buildForBench(b, map[string]interface{}{"parallel": 1}), benchPayload(1<<10))
}

func BenchmarkForEach_1KB_CPUChain_Parallel4(b *testing.B) {
	if err := RegisterRecordCodecOrReplace(benchTreeCodec{pooled: true}); err != nil {
		b.Fatal(err)
	}
	benchProc = "test_bench_hasher"
	defer func() { benchProc = "test_upper" }()
	runForEachBatchBench(b, buildForBench(b, map[string]interface{}{"parallel": 4}), benchPayload(1<<10))
}
