/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"fmt"
	"strings"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/lane"
	"infini.sh/framework/core/param"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/stats"
)

// OutputConfig declares the unified output step: exactly ONE target
// field (queue, lane or lanes) — config unpacking rejects zero or multiple.
type OutputConfig struct {
	// Queue accepts either a bare queue name (string) or a full config
	// object; the custom Unpack below handles both forms.
	Queue interface{} `config:"queue"`
	Lane  string      `config:"lane"`
	// Lanes fans the record out to several lanes (explicit fork): the
	// first lane receives the record itself (zero-copy for codec-decoded
	// records), every further lane an independent copy — each receiver
	// owns its record.
	Lanes []string `config:"lanes"`
	// Ack (lane target only): emit with an acknowledgment receipt attached
	// to the context's AckSet — the emitting consumer's commit gate waits
	// for settlement (cross-lane at-least-once).
	Ack          *bool         `config:"ack"`
	Codec        string        `config:"codec"`         // queue target: override codec resolution
	MessageField param.ParaKey `config:"message_field"` // batch scope queue target
}

type OutputProcessor struct {
	cfg       OutputConfig
	outQ      *queue.QueueConfig
	laneNames []string // resolved fan-out targets (single lane included)
	ack       bool
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
	queueName := ""
	if cfg.Queue != nil {
		targets++
		// resolve: bare string or object with name
		if s, ok := cfg.Queue.(string); ok {
			queueName = s
		} else if m, ok := cfg.Queue.(map[string]interface{}); ok {
			if n, ok := m["name"].(string); ok {
				queueName = n
			}
		}
	}
	if cfg.Lane != "" {
		targets++
	}
	if len(cfg.Lanes) > 0 {
		targets++
	}
	if targets != 1 {
		return nil, fmt.Errorf("%s requires exactly one target field (queue: <name>, lane: <name> or lanes: [<name>...]), found %d", outputName, targets)
	}
	p := &OutputProcessor{cfg: cfg}
	if cfg.Lane != "" {
		p.laneNames = []string{cfg.Lane}
	} else {
		p.laneNames = cfg.Lanes
	}
	if queueName != "" {
		qc := &queue.QueueConfig{Name: queueName}
		p.outQ = queue.SmartGetOrInitConfig(qc)
	}
	if cfg.Ack != nil {
		p.ack = *cfg.Ack
	}
	return p, nil
}

func (p *OutputProcessor) Name() string { return outputName }

func (p *OutputProcessor) Process(ctx *pipeline.Context) error {
	if len(p.laneNames) > 0 {
		return p.emitLanes(ctx)
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

// emitLanes: record scope only — the batch scope has no live records.
//
// Delivery is ownership transfer (single-owner rule, see
// core/pipeline/record.go): MarkDelivered sets the CONTEXT flag before any
// handoff so the channel's happens-before settles the record for the
// receiver, and nothing on this side touches the record afterwards — the
// chain runner stops the remaining processors of the record's chain, the
// host's encode-back pass empties the message slot, and a second sink in
// the same chain is skipped with a warning (fan out via lanes: [...],
// fork via clone).
func (p *OutputProcessor) emitLanes(ctx *pipeline.Context) error {
	rec, ok := pipeline.CurrentRecord(ctx)
	if !ok {
		return fmt.Errorf("%s{lane:%v} requires record scope (inside for_each/lane_consumer); for disk→lane bridging use consumer{codec, output_lane}", outputName, p.laneNames)
	}
	if pipeline.IsDelivered(ctx) {
		log.Warnf("%s: record already delivered to a sink; skipping lanes %v (use lanes: [...] for fan-out or the clone processor for forks)", outputName, p.laneNames)
		stats.Increment("pipeline", ctx.Config.Name, "processor", outputName, "total_deliver_skip")
		return nil
	}
	if p.capture(ctx, "lane", strings.Join(p.laneNames, ","), encodePreview(rec)) {
		return nil
	}
	// Prepare EVERY fan-out copy before the first hand-off: lane #0's
	// subscriber may mutate its record the moment Emit returns, and a
	// CloneEvent iterating rec.Fields afterwards would race with it (and
	// could snapshot lane #0's mutations into the other branches).
	outs := make([]*event.Event, len(p.laneNames))
	for i := range p.laneNames {
		// ownership hand-off: the emitting host (for_each) recycles pooled
		// records once this call returns, but the lane delivers the pointer
		// asynchronously — a pooled record must cross the boundary as a
		// pool-independent copy, or the subscriber reads a reset event.
		// Extra fan-out lanes get an independent copy even for non-pooled
		// records: every receiver owns its record.
		if i == 0 {
			outs[i] = event.DetachFromPool(rec)
		} else {
			outs[i] = event.CloneEvent(rec)
		}
	}
	// The FIRST successful hand-off settles the sender: that lane's
	// receiver may mutate the record from the moment the send returns, so
	// the sender must not touch it again — regardless of what the
	// remaining fan-out targets do. Marking only after the whole loop
	// (as an earlier iteration did) would leave sender and receiver
	// sharing the record on a partial fan-out failure. A failed later
	// target surfaces as the step error; the record never re-enters the
	// sender's batch (recovery is the ack/receipt layer's redelivery).
	delivered := false
	for i, name := range p.laneNames {
		l := lane.Get(name)
		if l == nil {
			l = lane.GetOrCreate(lane.LaneConfig{Name: name})
		}
		if !p.ack {
			if err := l.Emit(outs[i]); err != nil {
				return err
			}
		} else {
			r := l.EmitAck(outs[i])
			pipeline.AttachAckSet(ctx).AddReceipt(r)
		}
		if !delivered {
			delivered = true
			pipeline.MarkDelivered(ctx)
		}
	}
	// fan-out counts once per record (lane-side emit counters live on the
	// lanes themselves — laneStageStats reports them)
	stats.Increment("pipeline", ctx.Config.Name, "processor", outputName, "total_delivered")
	return nil
}

// pushQueue: batch scope pushes message bytes as-is (queue_output
// semantics); record scope encodes the current record once (durable
// bridge, 3-level codec resolution).
func (p *OutputProcessor) pushQueue(ctx *pipeline.Context) error {
	if rec, ok := pipeline.CurrentRecord(ctx); ok {
		if pipeline.IsDelivered(ctx) {
			log.Warnf("%s: record already delivered to a sink; skipping queue %v (use the clone processor for forks)", outputName, p.outQ.Name)
			stats.Increment("pipeline", ctx.Config.Name, "processor", outputName, "total_deliver_skip")
			return nil
		}
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
		if err := queue.Push(p.outQ, b); err != nil {
			return err
		}
		// byte-plane delivery: the queue owns the pushed bytes, and the
		// context flag keeps the host's encode-back from re-delivering
		// the record through the batch (single delivery)
		pipeline.MarkDelivered(ctx)
		stats.Increment("pipeline", ctx.Config.Name, "processor", outputName, "total_delivered")
		return nil
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
