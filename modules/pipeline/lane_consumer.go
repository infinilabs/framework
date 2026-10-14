/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"context"
	"fmt"
	"time"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/lane"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
)

// LaneConsumerConfig declares the lane_consumer chain host: the record-
// convention counterpart of the queue consumer. Records arrive as live
// pointers (no codec decode), the sub-chain executes stage-wise in
// written order, and surviving records optionally encode once into the
// output_queue durable bridge.
type LaneConsumerConfig struct {
	Lane string `config:"lane"`

	BatchMax     int `config:"batch_max"`      // flush on size
	BatchFlushMs int `config:"batch_flush_ms"` // flush on idle

	OnFailure  string `config:"on_failure"` // tag (default) | dead_letter:<queue> | ignore
	FailureTag string `config:"failure_tag"`

	OutputQueue string `config:"output_queue"` // durable bridge (encode once)
	OutputCodec string `config:"output_codec"` // override the 3-level resolution

	Processors []*config.Config `config:"processor"`
}

type LaneConsumerProcessor struct {
	cfg   LaneConsumerConfig
	lane  lane.Lane
	sub   *pipeline.Processors
	deadQ *queue.QueueConfig
	outQ  *queue.QueueConfig
}

const laneConsumerName = "lane_consumer"

func init() {
	pipeline.RegisterProcessorPluginWithConfigMetadata(laneConsumerName, NewLaneConsumerProcessor, LaneConsumerConfig{})
	pipeline.RegisterProcessorContract(laneConsumerName, pipeline.RecordContract{Scope: pipeline.ScopeBatch})
}

func NewLaneConsumerProcessor(c *config.Config) (pipeline.Processor, error) {
	cfg := LaneConsumerConfig{BatchMax: 64, BatchFlushMs: 100, OnFailure: "tag"}
	if err := c.Unpack(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unpack the configuration of %s processor: %s", laneConsumerName, err)
	}
	if cfg.Lane == "" {
		return nil, fmt.Errorf("%s requires a lane", laneConsumerName)
	}
	if len(cfg.Processors) == 0 {
		return nil, fmt.Errorf("%s requires a non-empty processor sub-chain", laneConsumerName)
	}
	if cfg.BatchMax <= 0 {
		cfg.BatchMax = 64
	}
	if cfg.FailureTag == "" {
		cfg.FailureTag = "_processing_failed"
	}
	l := lane.Get(cfg.Lane)
	if l == nil {
		return nil, fmt.Errorf("%s: lane %q not found (define it under lane: or create it dynamically)", laneConsumerName, cfg.Lane)
	}
	sub, err := pipeline.NewPipelineWithScope(cfg.Processors, pipeline.ScopeInfo{
		Kind:      pipeline.ScopeRecord,
		Namespace: l.Config().Namespace,
	}, true)
	if err != nil {
		return nil, fmt.Errorf("failed to compile %s sub-chain: %s", laneConsumerName, err)
	}
	p := &LaneConsumerProcessor{cfg: cfg, lane: l, sub: sub}
	if cfg.OutputQueue != "" {
		p.outQ = queue.SmartGetOrInitConfig(&queue.QueueConfig{Name: cfg.OutputQueue})
	}
	if len(cfg.OnFailure) > len("dead_letter:") && cfg.OnFailure[:len("dead_letter:")] == "dead_letter:" {
		p.deadQ = queue.SmartGetOrInitConfig(&queue.QueueConfig{Name: cfg.OnFailure[len("dead_letter:"):], Labels: nil})
	}
	return p, nil
}

func (p *LaneConsumerProcessor) Name() string { return laneConsumerName }

// Process subscribes (once) and blocks until the pipeline task cancels —
// the host drives the lane for its whole lifetime.
func (p *LaneConsumerProcessor) Process(ctx *pipeline.Context) error {
	segments := buildSegments(p.sub.List)
	batchFn := func(recs []*event.Event) {
		p.runBatch(ctx, segments, recs)
	}
	_ = p.lane.SubscribeBatch(p.cfg.BatchMax, time.Duration(p.cfg.BatchFlushMs)*time.Millisecond, batchFn)
	<-ctx.Done()
	p.lane.Close()
	return nil
}

// runBatch executes the stage-wise segments over one micro-batch, then
// encodes survivors into the durable bridge. Receipts of the batch settle
// when this returns (SubscribeBatch contract) — processing completion is
// the settlement point.
func (p *LaneConsumerProcessor) runBatch(ctx *pipeline.Context, segments []execSegment, recs []*event.Event) {
	active := recs
	for _, seg := range segments {
		if ctx.IsCanceled() || !ctx.ShouldContinue() {
			return
		}
		if seg.batch {
			live := make([]*event.Event, 0, len(active))
			for _, rec := range active {
				if !pipeline.IsDropped(rec) {
					live = append(live, rec)
				}
			}
			if len(live) == 0 {
				continue
			}
			if err := seg.proc.(pipeline.BatchProcessor).ProcessBatch(ctx, live); err != nil {
				if p.cfg.OnFailure == "ignore" {
					log.Warnf("lane_consumer: batch segment error ignored: %v", err)
				} else {
					for _, rec := range live {
						mergeFailureTags(rec, []string{p.cfg.FailureTag})
					}
					log.Warnf("lane_consumer: batch segment error tagged %q: %v", p.cfg.FailureTag, err)
				}
			}
			continue
		}
		next := active[:0]
		for _, rec := range active {
			if pipeline.IsDropped(rec) {
				continue
			}
			c := &pipeline.Context{ParentContext: ctx, Context: ctx.Context, Config: ctx.Config}
			c.Set(pipeline.RecordContextKey, rec)
			if p.cfg.OnFailure == "tag" {
				var tags []string
				c.Set(pipeline.FailureTagsKey, &tags)
				for _, proc := range seg.chain.List {
					if err := proc.Process(c); err != nil {
						pipeline.AppendFailureTag(c, p.cfg.FailureTag)
						log.Warnf("lane_consumer: sub-chain error tagged %q: %v", p.cfg.FailureTag, err)
					}
				}
				mergeFailureTags(rec, tags)
			} else if err := seg.chain.Process(c); err != nil {
				log.Warnf("lane_consumer: record error (on_failure=%s): %v", p.cfg.OnFailure, err)
				if p.deadQ != nil {
					p.pushEncoded(p.deadQ, rec)
					continue // diverted: dead_letter is a settlement
				}
			}
			if !pipeline.IsDropped(rec) {
				next = append(next, rec)
			}
		}
		active = next
	}
	if p.outQ != nil {
		for _, rec := range active {
			if !pipeline.IsDropped(rec) {
				p.pushEncoded(p.outQ, rec)
			}
		}
	}
}

// pushEncoded applies the 3-level codec resolution and pushes to a queue.
func (p *LaneConsumerProcessor) pushEncoded(q *queue.QueueConfig, rec *event.Event) {
	codecName := p.cfg.OutputCodec
	if codecName == "" {
		codecName = pipeline.CodecHint(rec)
	}
	codec := GetRecordCodec(codecName)
	if codecName == "" && p.lane.Config().Namespace != "" {
		for _, ci := range pipeline.ListRecordCodecs() {
			if ci.Namespace == p.lane.Config().Namespace {
				codec = GetRecordCodec(ci.Name)
				break
			}
		}
	}
	b, err := codec.Encode(rec)
	if err != nil {
		log.Errorf("lane_consumer: encode for output failed: %v", err)
		return
	}
	if err := queue.Push(q, b); err != nil {
		log.Errorf("lane_consumer: push to queue %q failed: %v", q.Name, err)
	}
}

var _ = context.Background // keep context import for future cancellation plumbing
