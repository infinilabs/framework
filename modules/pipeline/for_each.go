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
//     processor:                  # sub-chain, executed per record
//   - dissect:
//     pattern: "%{log_level} %{message}"
type ForEachConfig struct {
	MessageField string           `config:"message_field"`
	Codec        string           `config:"codec"`
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

	// The record convention is only valid inside the sub-chain; save and
	// restore the previous value so the batch's last record does not leak
	// to processors downstream of for_each (and nested for_each keeps
	// working).
	previous := c.Get(pipeline.RecordContextKey)
	defer c.Set(pipeline.RecordContextKey, previous)

	for i := range msgs {
		if c.IsCanceled() || !c.ShouldContinue() {
			break
		}
		data := msgs[i].Data
		if len(data) == 0 {
			continue
		}
		rec, err := p.codec.Decode(data)
		if err != nil {
			log.Warnf("for_each: failed to decode message payload at offset %v: %v", msgs[i].Offset, err)
			continue
		}

		c.Set(pipeline.RecordContextKey, rec)
		if err := p.sub.Process(c); err != nil {
			log.Warnf("for_each: sub-chain error on record at offset %v: %v", msgs[i].Offset, err)
		}

		// drop_event marks: clear the payload so downstream stages
		// (otlp_export and friends) skip this record entirely.
		if pipeline.IsDropped(rec) {
			msgs[i].Data = nil
			msgs[i].Size = 0
			continue
		}

		encoded, err := p.codec.Encode(rec)
		if err != nil {
			log.Warnf("for_each: failed to encode record at offset %v: %v", msgs[i].Offset, err)
			continue
		}
		msgs[i].Data = encoded
		msgs[i].Size = len(encoded)
	}
	return nil
}
