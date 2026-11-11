/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"fmt"
	"math/rand"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/lane"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
)

// TapConfig declares the tap step: copy a sampled fraction of records to
// a side target; the main flow is never affected (in-flight copy, not a
// divert). Target is one-of queue/lane — the sanctioned side-channel for
// debug indices, shadow runs and observation sampling.
type TapConfig struct {
	SampleRatio float64            `config:"sample_ratio"` // 0~1, 1.0 = all
	Queue       *queue.QueueConfig `config:"queue"`
	Lane        string             `config:"lane"`
}

type TapProcessor struct {
	cfg  TapConfig
	outQ *queue.QueueConfig
}

const tapName = "tap"

func init() {
	pipeline.RegisterProcessorPluginWithConfigMetadata(tapName, NewTapProcessor, TapConfig{})
	pipeline.RegisterProcessorContract(tapName, pipeline.RecordContract{Scope: pipeline.ScopeRecord})
}

func NewTapProcessor(c *config.Config) (pipeline.Processor, error) {
	cfg := TapConfig{SampleRatio: 1.0}
	if err := c.Unpack(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unpack the configuration of %s processor: %s", tapName, err)
	}
	targets := 0
	if cfg.Queue != nil {
		targets++
	}
	if cfg.Lane != "" {
		targets++
	}
	if targets != 1 {
		return nil, fmt.Errorf("%s requires exactly one target field (queue: <name> or lane: <name>), found %d", tapName, targets)
	}
	if cfg.SampleRatio <= 0 || cfg.SampleRatio > 1 {
		return nil, fmt.Errorf("%s sample_ratio must be within (0, 1]", tapName)
	}
	p := &TapProcessor{cfg: cfg}
	if cfg.Queue != nil {
		p.outQ = queue.SmartGetOrInitConfig(cfg.Queue)
	}
	return p, nil
}

func (p *TapProcessor) Name() string { return tapName }

// Process copies the current record to the side target by sample ratio;
// the original keeps flowing regardless of the copy's fate.
func (p *TapProcessor) Process(ctx *pipeline.Context) error {
	rec, ok := pipeline.CurrentRecord(ctx)
	if !ok {
		return nil
	}
	if p.cfg.SampleRatio < 1 && rand.Float64() > p.cfg.SampleRatio {
		return nil
	}
	// deep copy: later main-flow mutations must not leak into the tapped
	// copy (and vice versa — Fields is shared authority in the original)
	cp := copyRecord(rec)
	if sink := pipeline.DryRunCapture(ctx); sink != nil {
		if b, err := GetRecordCodec(pipeline.CodecHint(cp)).Encode(cp); err == nil {
			kind, target := "lane", p.cfg.Lane
			if p.outQ != nil {
				kind, target = "queue", p.outQ.Name
			}
			*sink = append(*sink, pipeline.DryRunWrite{Kind: kind, Target: target, Records: 1, Preview: string(b)})
		}
		return nil
	}
	if p.outQ != nil {
		b, err := GetRecordCodec(pipeline.CodecHint(cp)).Encode(cp)
		if err != nil {
			log.Warnf("tap: encode failed: %v", err)
			return nil
		}
		if err := queue.Push(p.outQ, b); err != nil {
			log.Warnf("tap: push to queue %q failed: %v", p.outQ.Name, err)
		}
		return nil
	}
	l := lane.Get(p.cfg.Lane)
	if l == nil {
		l = lane.GetOrCreate(lane.LaneConfig{Name: p.cfg.Lane})
	}
	if err := l.Emit(cp); err != nil {
		log.Warnf("tap: emit to lane %q failed: %v", p.cfg.Lane, err)
	}
	return nil
}

func copyRecord(rec *event.Event) *event.Event {
	b, err := util.ToJSONBytes(map[string]interface{}{"fields": rec.Fields, "labels": rec.Metadata.Labels})
	if err != nil {
		return rec // fallback: share (best effort)
	}
	wrap := map[string]interface{}{}
	if err := util.FromJSONBytes(b, &wrap); err != nil {
		return rec
	}
	cp := &event.Event{Timestamp: rec.Timestamp}
	cp.Fields, _ = wrap["fields"].(map[string]interface{})
	if lbl, ok := wrap["labels"].(map[string]interface{}); ok {
		cp.Metadata.Labels = lbl
	}
	if cp.Fields == nil {
		cp.Fields = util.MapStr{}
	}
	return cp
}
