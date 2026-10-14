/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"fmt"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/lane"
	"infini.sh/framework/core/param"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
)

// OutputConfig declares the unified output step: exactly ONE target
// field (queue or lane) — config unpacking rejects zero or multiple.
type OutputConfig struct {
	Queue *queue.QueueConfig `config:"queue"`
	Lane  string             `config:"lane"`
	// Ack (lane target only): emit with an acknowledgment receipt attached
	// to the context's AckSet — the emitting consumer's commit gate waits
	// for settlement (cross-lane at-least-once).
	Ack          *bool         `config:"ack"`
	Codec        string        `config:"codec"`         // queue target: override codec resolution
	MessageField param.ParaKey `config:"message_field"` // batch scope queue target
}

type OutputProcessor struct {
	cfg      OutputConfig
	outQ     *queue.QueueConfig
	laneName string
	ack      bool
}

const outputName = "output"

func init() {
	pipeline.RegisterProcessorPluginWithConfigMetadata(outputName, NewOutputProcessor, OutputConfig{})
	// batch+record conditional by target — declared record-agnostic; the
	// batch-scope lane target errors at runtime with a clear pointer.
	pipeline.RegisterProcessorContract(outputName, pipeline.RecordContract{})
}

func NewOutputProcessor(c *config.Config) (pipeline.Processor, error) {
	cfg := OutputConfig{MessageField: "messages"}
	if err := c.Unpack(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unpack the configuration of %s processor: %s", outputName, err)
	}
	targets := 0
	if cfg.Queue != nil {
		targets++
	}
	if cfg.Lane != "" {
		targets++
	}
	if targets != 1 {
		return nil, fmt.Errorf("%s requires exactly one target field (queue: <name> or lane: <name>), found %d", outputName, targets)
	}
	p := &OutputProcessor{cfg: cfg, laneName: cfg.Lane}
	if cfg.Queue != nil {
		p.outQ = queue.SmartGetOrInitConfig(cfg.Queue)
	}
	if cfg.Ack != nil {
		p.ack = *cfg.Ack
	}
	return p, nil
}

func (p *OutputProcessor) Name() string { return outputName }

func (p *OutputProcessor) Process(ctx *pipeline.Context) error {
	if p.laneName != "" {
		return p.emitLane(ctx)
	}
	return p.pushQueue(ctx)
}

// capture records an intercepted write when the context runs a capturing
// dry run; returns true when intercepted (the caller must NOT deliver).
func (p *OutputProcessor) capture(ctx *pipeline.Context, kind, target string, preview []byte) bool {
	sink := pipeline.DryRunCapture(ctx)
	if sink == nil {
		return false
	}
	*sink = append(*sink, pipeline.DryRunWrite{Kind: kind, Target: target, Records: 1, Preview: truncate(preview, 512)})
	return true
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}

// emitLane: record scope only — the batch scope has no live records.
func (p *OutputProcessor) emitLane(ctx *pipeline.Context) error {
	rec, ok := pipeline.CurrentRecord(ctx)
	if !ok {
		return fmt.Errorf("%s{lane:%s} requires record scope (inside for_each/lane_consumer); for disk→lane bridging use consumer{codec, output_lane}", outputName, p.laneName)
	}
	if p.capture(ctx, "lane", p.laneName, encodePreview(rec)) {
		return nil
	}
	l := lane.Get(p.laneName)
	if l == nil {
		l = lane.GetOrCreate(lane.LaneConfig{Name: p.laneName})
	}
	if !p.ack {
		return l.Emit(rec)
	}
	r := l.EmitAck(rec)
	pipeline.AttachAckSet(ctx).AddReceipt(r)
	return nil
}

// pushQueue: batch scope pushes message bytes as-is (queue_output
// semantics); record scope encodes the current record once (durable
// bridge, 3-level codec resolution).
func (p *OutputProcessor) pushQueue(ctx *pipeline.Context) error {
	if rec, ok := pipeline.CurrentRecord(ctx); ok {
		codecName := p.cfg.Codec
		if codecName == "" {
			codecName = pipeline.CodecHint(rec)
		}
		b, err := GetRecordCodec(codecName).Encode(rec)
		if err != nil {
			return err
		}
		if p.capture(ctx, "queue", p.outQ.Name, b) {
			return nil
		}
		return queue.Push(p.outQ, b)
	}
	obj := ctx.Get(p.cfg.MessageField)
	msgs, ok := obj.([]queue.Message)
	if !ok {
		log.Warnf("processor [%s] no batch under [%s], skip", outputName, p.cfg.MessageField)
		return nil
	}
	for i := range msgs {
		if len(msgs[i].Data) == 0 {
			continue
		}
		if err := queue.Push(p.outQ, msgs[i].Data); err != nil {
			return err
		}
	}
	return nil
}

func encodePreview(rec *event.Event) []byte {
	b, err := GetRecordCodec(pipeline.CodecHint(rec)).Encode(rec)
	if err != nil {
		return nil
	}
	return b
}
