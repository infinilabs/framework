// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello#infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package pipeline

import (
	"fmt"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/param"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
)

// ForEachProcessor splits a batch of queue messages into individual
// records and runs a sub-chain of processors on each record.
//
// Each message payload is decoded into an *event.Event via the configured
// RecordCodec (default "otel" — the otel envelope shared with the agent
// LogEvent format; other payload formats register their own codec),
// exposed to the sub-chain under pipeline.RecordContextKey, mutated in
// place, then re-encoded back into the message. Per-record transform
// processors (dissect, field_standardize, ...) rely on this convention.
//
// Configuration:
//
//   - for_each:
//     message_field: messages     # where the consumer stored []queue.Message
//     codec: otel                # payload codec (see RecordCodec)
//     on_failure: ignore          # ignore | tag | fail (sub-chain error policy)
//     failure_tag: _processing_failed   # tag appended when on_failure=tag
//     processor:                  # sub-chain, executed per record
//   - dissect:
//     pattern: "%{log_level} %{message}"
//
// on_failure strategies (default: ignore — the historical behavior):
//
//	ignore  log a warning and keep the (possibly partially mutated) record
//	tag     append failure_tag to the record's failure tags and keep it;
//	        downstream processors can route on it
//	fail    abort the whole batch: Process returns the error, so the
//	        consumer leaves the offset uncommitted and the batch is
//	        redelivered (at-least-once)
//
// Sub-processors implementing pipeline.BatchProcessor are executed once
// per batch (after per-record decoding) instead of once per record;
// plain Processors keep the per-record semantics unchanged.
type ForEachConfig struct {
	MessageField string           `config:"message_field"`
	Codec        string           `config:"codec"`
	OnFailure    string           `config:"on_failure"`
	FailureTag   string           `config:"failure_tag"`
	Processors   []*config.Config `config:"processor"`
}

type ForEachProcessor struct {
	cfg   ForEachConfig
	codec RecordCodec
	sub   *pipeline.Processors
}

func NewForEachProcessor(c *config.Config) (pipeline.Processor, error) {
	cfg := ForEachConfig{MessageField: "messages"}
	if err := c.Unpack(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unpack the configuration of for_each processor: %s", err)
	}
	if len(cfg.Processors) == 0 {
		return nil, fmt.Errorf("for_each processor requires a non-empty processor sub-chain")
	}
	sub, err := pipeline.NewPipeline(cfg.Processors)
	if err != nil {
		return nil, fmt.Errorf("failed to compile for_each sub-chain: %s", err)
	}
	codec := GetRecordCodec(cfg.Codec)
	if cfg.Codec != "" && codec.Name() != cfg.Codec {
		return nil, fmt.Errorf("unknown for_each codec %q (registered: %v)", cfg.Codec, recordCodecNames())
	}
	switch cfg.OnFailure {
	case "", "ignore", "tag", "fail":
	default:
		return nil, fmt.Errorf("invalid for_each on_failure %q (ignore|tag|fail)", cfg.OnFailure)
	}
	if cfg.FailureTag == "" {
		cfg.FailureTag = "_processing_failed"
	}
	return &ForEachProcessor{cfg: cfg, codec: codec, sub: sub}, nil
}

func recordCodecNames() []string {
	codecsMu.RLock()
	defer codecsMu.RUnlock()
	names := make([]string, 0, len(codecs))
	for n := range codecs {
		names = append(names, n)
	}
	return names
}

func (p *ForEachProcessor) Name() string { return "for_each" }

func (p *ForEachProcessor) Process(c *pipeline.Context) error {
	v := c.Get(param.ParaKey(p.cfg.MessageField))
	msgs, ok := v.([]queue.Message)
	if !ok {
		log.Debugf("for_each: no batch found under context key [%s], skip", p.cfg.MessageField)
		return nil
	}

	// Decode pass: build the record slice; undecodable payloads keep their
	// original bytes and are passed through untouched (payload transparency
	// for mixed batches).
	records := make([]*event.Event, 0, len(msgs))
	decodedIdx := make([]int, 0, len(msgs)) // msgs index per record
	for i := range msgs {
		if len(msgs[i].Data) == 0 {
			continue
		}
		rec, err := p.codec.Decode(msgs[i].Data)
		if err != nil {
			log.Warnf("for_each: failed to decode message payload at offset %v: %v", msgs[i].Offset, err)
			continue
		}
		records = append(records, rec)
		decodedIdx = append(decodedIdx, i)
	}

	// Batch pass: sub-processors implementing BatchProcessor run once per
	// batch; the rest run per record below. Drop marks from the batch pass
	// are honored through the same per-record drop handling.
	for _, proc := range p.sub.List {
		if bp, ok := proc.(pipeline.BatchProcessor); ok {
			if err := bp.ProcessBatch(c, records); err != nil {
				if err2 := p.handleFailure(c, records, err); err2 != nil {
					return err2
				}
			}
		}
	}

	// Record pass: plain processors, per record. Batch-aware processors
	// already ran in the batch pass and are skipped here.
	perRecord := make([]pipeline.Processor, 0, len(p.sub.List))
	for _, proc := range p.sub.List {
		if _, isBatch := proc.(pipeline.BatchProcessor); !isBatch {
			perRecord = append(perRecord, proc)
		}
	}
	for j, rec := range records {
		if c.IsCanceled() || !c.ShouldContinue() {
			break
		}
		if pipeline.IsDropped(rec) {
			// dropped by the batch pass
			i := decodedIdx[j]
			msgs[i].Data = nil
			msgs[i].Size = 0
			continue
		}
		c.Set(pipeline.RecordContextKey, rec)
		// lazy failure-tag slice (on_failure: tag initializes on demand)
		var tags []string
		if p.cfg.OnFailure == "tag" {
			tags = []string{}
			c.Set(pipeline.FailureTagsKey, &tags)
		}
		for _, proc := range perRecord {
			if err := proc.Process(c); err != nil {
				if err2 := p.handleFailure(c, nil, err); err2 != nil {
					return err2
				}
				if p.cfg.OnFailure == "fail" {
					break
				}
			}
		}

		// drop_event marks: clear the payload so downstream stages
		// skip this record entirely.
		if pipeline.IsDropped(rec) {
			i := decodedIdx[j]
			msgs[i].Data = nil
			msgs[i].Size = 0
			continue
		}

		encoded, err := p.codec.Encode(rec)
		if err != nil {
			log.Warnf("for_each: failed to encode record at offset %v: %v", msgs[decodedIdx[j]].Offset, err)
			continue
		}
		i := decodedIdx[j]
		msgs[i].Data = encoded
		msgs[i].Size = len(encoded)
	}
	return nil
}

// handleFailure applies the configured on_failure strategy to a sub-chain
// error. Returns a non-nil error only for the fail strategy (abort batch).
func (p *ForEachProcessor) handleFailure(c *pipeline.Context, records []*event.Event, err error) error {
	switch p.cfg.OnFailure {
	case "fail":
		return fmt.Errorf("for_each: sub-chain failed (on_failure=fail): %w", err)
	case "tag":
		pipeline.AppendFailureTag(c, p.cfg.FailureTag)
		log.Warnf("for_each: sub-chain error tagged %q: %v", p.cfg.FailureTag, err)
		return nil
	default: // ignore (historical behavior)
		log.Warnf("for_each: sub-chain error ignored: %v", err)
		return nil
	}
}
