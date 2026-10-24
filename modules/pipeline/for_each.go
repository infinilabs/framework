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
	"sync"
	"sync/atomic"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/param"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
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
//	ignore  log a warning and skip the rest of the sub-chain for that
//	        record, keeping the (possibly partially mutated) record
//	tag     append failure_tag to the record's failure tags and keep
//	        processing it; accumulated tags are persisted into the
//	        record's Fields["tags"] before re-encoding, so downstream
//	        stages can route on them (Beats/Vector convention)
//	fail    abort the whole batch: Process returns the error, so the
//	        consumer leaves the offset uncommitted and the batch is
//	        redelivered (at-least-once)
//
// Sub-processors implementing pipeline.BatchProcessor are executed once
// per batch (after per-record decoding), before any per-record processor
// regardless of their position in the sub-chain; plain Processors keep
// the per-record semantics unchanged.
type ForEachConfig struct {
	MessageField string `config:"message_field"`
	Codec        string `config:"codec"`
	// Parallel runs each record segment with N workers (default 1 =
	// serial). Processors must be safe for concurrent Process calls on
	// DISTINCT records (they mutate the record, never shared state) —
	// every in-repo record processor satisfies this.
	Parallel   int              `config:"parallel"`
	OnFailure  string           `config:"on_failure"`
	FailureTag string           `config:"failure_tag"`
	Processors []*config.Config `config:"processor"`
}

// execSegment is one stage of the stage-wise (stage) executor: a run of
// consecutive per-record processors, or a single batch processor. Segments
// execute in WRITTEN order; a batch processor therefore sees the edits of
// the record processors declared before it (and vice versa).
type execSegment struct {
	batch bool
	proc  pipeline.Processor   // batch segments: the single processor
	chain *pipeline.Processors // record segments: per-record sub-chain
}

type ForEachProcessor struct {
	cfg   ForEachConfig
	codec RecordCodec
	sub   *pipeline.Processors
}

// buildSegments splits a compiled sub-chain into alternating segments.
func buildSegments(list []pipeline.Processor) []execSegment {
	var segs []execSegment
	for _, p := range list {
		if _, isBatch := p.(pipeline.BatchProcessor); isBatch {
			segs = append(segs, execSegment{batch: true, proc: p})
		} else if n := len(segs); n > 0 && !segs[n-1].batch {
			segs[n-1].chain.List = append(segs[n-1].chain.List, p)
		} else {
			segs = append(segs, execSegment{chain: &pipeline.Processors{List: []pipeline.Processor{p}}})
		}
	}
	return segs
}

func NewForEachProcessor(c *config.Config) (pipeline.Processor, error) {
	cfg := ForEachConfig{MessageField: "messages"}
	if err := c.Unpack(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unpack the configuration of for_each processor: %s", err)
	}
	if len(cfg.Processors) == 0 {
		return nil, fmt.Errorf("for_each processor requires a non-empty processor sub-chain")
	}
	codec := GetRecordCodec(cfg.Codec)
	if cfg.Codec != "" && codec.Name() != cfg.Codec {
		return nil, fmt.Errorf("unknown for_each codec %q (registered: %v)", cfg.Codec, recordCodecNames())
	}
	sub, err := pipeline.NewPipelineWithScope(cfg.Processors, pipeline.ScopeInfo{
		Kind:      pipeline.ScopeRecord,
		Namespace: codec.Namespace(),
	}, true)
	if err != nil {
		return nil, fmt.Errorf("failed to compile for_each sub-chain: %s", err)
	}
	switch cfg.OnFailure {
	case "", "ignore", "tag", "fail":
	default:
		return nil, fmt.Errorf("invalid for_each on_failure %q (ignore|tag|fail)", cfg.OnFailure)
	}
	if cfg.FailureTag == "" {
		cfg.FailureTag = "_processing_failed"
	}
	if cfg.Parallel <= 0 {
		cfg.Parallel = 1
	}
	return &ForEachProcessor{cfg: cfg, codec: codec, sub: sub}, nil
}

func recordCodecNames() []string {
	infos := ListRecordCodecs()
	names := make([]string, 0, len(infos))
	for _, i := range infos {
		names = append(names, i.Name)
	}
	return names
}

func (p *ForEachProcessor) Name() string { return "for_each" }

// ApplyScope recompiles the sub-chain under the enclosing compile scope:
// the registry constructor takes no scope, so the initial compile is
// warn-only — a strict parent demands the strict view of the sub-chain
// (pipeline.ScopeAwareProcessor; idempotent by replacement).
func (p *ForEachProcessor) ApplyScope(scope pipeline.ScopeInfo) error {
	if !scope.Strict {
		return nil
	}
	sub, err := pipeline.NewPipelineWithScope(p.cfg.Processors, pipeline.ScopeInfo{
		Kind:      pipeline.ScopeRecord,
		Namespace: p.codec.Namespace(),
		Strict:    true,
	}, true)
	if err != nil {
		return fmt.Errorf("failed to compile for_each sub-chain: %s", err)
	}
	p.sub = sub
	return nil
}

func (p *ForEachProcessor) Process(c *pipeline.Context) error {
	// The record convention is only valid inside the sub-chain; save and
	// restore the previous values so the batch's last record (and its
	// failure tags) does not leak to processors downstream of for_each,
	// and nested for_each restores the outer record scope.
	previous := c.Get(pipeline.RecordContextKey)
	previousTags := c.Get(pipeline.FailureTagsKey)
	defer func() {
		c.Set(pipeline.RecordContextKey, previous)
		c.Set(pipeline.FailureTagsKey, previousTags)
	}()

	v := c.Get(param.ParaKey(p.cfg.MessageField))
	msgs, ok := v.([]queue.Message)
	if !ok {
		// Dry-run replay: the harness binds one decoded record instead of
		// a message batch. Adopt it as the single input record — the same
		// zero-decode typed entry lane_consumer uses at runtime — so the
		// sub-chain really executes and step diffs show its effect. There
		// is no message to encode back into; the harness owns the record
		// (runOnRecords skips release and encode for adopted records).
		if pipeline.InDryRun(c) {
			if rec, has := pipeline.CurrentRecord(c); has {
				return p.runOnRecords(c, []*event.Event{rec}, nil, nil)
			}
		}
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
	return p.runOnRecords(c, records, msgs, decodedIdx)
}

// runOnRecords executes the sub-chain stage-wise over records. With msgs
// non-nil (normal runtime) originals encode back into their message slot,
// clones append messages, and pooled records are released. With msgs nil
// (dry-run adoption) the caller owns the records: serial execution only
// (deterministic diffs), no encode pass, no release.
func (p *ForEachProcessor) runOnRecords(c *pipeline.Context, records []*event.Event, msgs []queue.Message, decodedIdx []int) error {
	segments := buildSegments(p.sub.List)

	// Stage-wise execution: segments run in WRITTEN order (record runs,
	// then a batch call, then more record runs...). A batch processor sees
	// the edits of record processors declared before it; drops propagate
	// to every later segment; clones join the active set for later
	// segments.
	active := make([]forEachActive, 0, len(records))
	for j, rec := range records {
		idx := -1
		if msgs != nil {
			idx = decodedIdx[j]
		}
		active = append(active, forEachActive{rec: rec, msgIdx: idx})
	}
	// offsetOf is for logging only; adopted records carry no message.
	offsetOf := func(ar *forEachActive) interface{} {
		if msgs == nil || ar.msgIdx < 0 {
			return -1
		}
		return msgs[ar.msgIdx].Offset
	}
	// release pooled records on every exit path — by now each record has
	// been encoded (or dropped) and clones have joined active; non-pooled
	// records (codec- or clone-allocated) make ReleaseEvent a no-op.
	// Adopted dry-run records belong to the harness — never released here.
	if msgs != nil {
		defer func() {
			for i := range active {
				event.ReleaseEvent(active[i].rec)
			}
		}()
	}

	for _, seg := range segments {
		if c.IsCanceled() || !c.ShouldContinue() {
			return nil
		}
		if seg.batch {
			live := make([]*event.Event, 0, len(active))
			for i := range active {
				if !pipeline.IsDropped(active[i].rec) {
					live = append(live, active[i].rec)
				}
			}
			if len(live) == 0 {
				continue
			}
			bp := seg.proc.(pipeline.BatchProcessor)
			if err := bp.ProcessBatch(c, live); err != nil {
				if p.cfg.OnFailure == "fail" {
					return p.handleFailure(c, err)
				}
				if p.cfg.OnFailure == "tag" {
					// a batch-segment failure tags every live record of
					// the batch (there is no per-record scope here)
					for _, rec := range live {
						mergeFailureTags(rec, []string{p.cfg.FailureTag})
					}
				}
				log.Warnf("for_each: batch segment [%s] error (%s): %v", seg.proc.Name(), p.cfg.OnFailure, err)
			}
			continue
		}

		// record segment: per active record, run the segment's sub-chain
		// (dry-run adoption stays serial for deterministic diffs)
		if p.cfg.Parallel > 1 && msgs != nil {
			runRecordSegmentParallel(c, seg.chain, active, p.cfg, func(cl *event.Event, idx int) {
				active = append(active, forEachActive{rec: cl, msgIdx: idx, isClone: true})
			})
			continue
		}
		for i := range active {
			if c.IsCanceled() || !c.ShouldContinue() {
				break
			}
			ar := &active[i]
			if pipeline.IsDropped(ar.rec) {
				continue
			}
			c.Set(pipeline.RecordContextKey, ar.rec)
			clones := &[]*event.Event{}
			c.Set(pipeline.CloneContextKey, clones)
			if p.cfg.OnFailure == "tag" {
				var tags []string
				c.Set(pipeline.FailureTagsKey, &tags)
				tagged := false
				for _, proc := range seg.chain.List {
					if err := proc.Process(c); err != nil {
						if !tagged {
							pipeline.AppendFailureTag(c, p.cfg.FailureTag)
							tagged = true
						}
						log.Warnf("for_each: sub-chain error tagged %q on record at offset %v: %v",
							p.cfg.FailureTag, offsetOf(ar), err)
					}
				}
				mergeFailureTags(ar.rec, tags)
			} else {
				if err := seg.chain.Process(c); err != nil {
					if err2 := p.handleFailure(c, err); err2 != nil {
						return err2
					}
				}
			}
			// clones join the active set and flow through later segments
			for _, cl := range pipeline.TakeClones(c) {
				active = append(active, forEachActive{rec: cl, msgIdx: ar.msgIdx, isClone: true})
			}
		}
	}

	// Adopted dry-run records have no message to encode back into — the
	// harness snapshots the record directly.
	if msgs == nil {
		return nil
	}

	// Encode pass: originals write their message slot, clones append new
	// messages sharing the parent's offset semantics.
	for i := range active {
		ar := active[i]
		if pipeline.IsDropped(ar.rec) {
			if !ar.isClone {
				msgs[ar.msgIdx].Data = nil
				msgs[ar.msgIdx].Size = 0
			}
			continue
		}
		encoded, err := p.codec.Encode(ar.rec)
		if err != nil {
			log.Warnf("for_each: failed to encode record at offset %v: %v", msgs[ar.msgIdx].Offset, err)
			continue
		}
		if ar.isClone {
			msgs = append(msgs, queue.Message{
				Offset: msgs[ar.msgIdx].Offset,
				Data:   encoded,
				Size:   len(encoded),
			})
		} else {
			msgs[ar.msgIdx].Data = encoded
			msgs[ar.msgIdx].Size = len(encoded)
		}
	}

	// publish the (possibly extended) batch back to the context
	c.Set(param.ParaKey(p.cfg.MessageField), msgs)
	return nil
}

// handleFailure applies the configured on_failure strategy to a sub-chain
// error. Returns a non-nil error only for the fail strategy (abort batch).
// Outside a per-record scope (the batch pass) there is no tag slice
// attached, so the tag strategy degrades to a warning there.
func (p *ForEachProcessor) handleFailure(c *pipeline.Context, err error) error {
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

// mergeFailureTags persists accumulated failure tags into the record's
// Fields["tags"], preserving tags the record already carried.
func mergeFailureTags(rec *event.Event, tags []string) {
	if len(tags) == 0 {
		return
	}
	if rec.Fields == nil {
		rec.Fields = util.MapStr{}
	}
	switch existing := rec.Fields["tags"].(type) {
	case nil:
		rec.Fields["tags"] = append([]string(nil), tags...)
	case []string:
		rec.Fields["tags"] = append(existing, tags...)
	case []interface{}:
		for _, t := range tags {
			existing = append(existing, t)
		}
		rec.Fields["tags"] = existing
	}
}

// forEachActive is one in-flight record of a batch (record + origin
// message slot + clone marker).
type forEachActive struct {
	rec     *event.Event
	msgIdx  int
	isClone bool
}

// runRecordSegmentParallel runs one record segment with N workers. Each
// worker derives its own context (per-record state like RecordContextKey
// is goroutine-local; cancellation still propagates from the parent), so
// processors only ever touch their own record — the concurrency contract
// for parallel record processing.
func runRecordSegmentParallel(c *pipeline.Context, chain *pipeline.Processors, active []forEachActive, cfg ForEachConfig, addClone func(cl *event.Event, idx int)) {
	if len(active) == 0 {
		return
	}
	workers := cfg.Parallel
	if workers > len(active) {
		workers = len(active)
	}
	var next int64
	var mu sync.Mutex
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// per-worker derived context: own record/clone/tag state
			wc := &pipeline.Context{ParentContext: c, Context: c.Context, Config: c.Config}
			for {
				if c.IsCanceled() || !c.ShouldContinue() {
					return
				}
				i := int(atomic.AddInt64(&next, 1) - 1)
				if i >= len(active) {
					return
				}
				ar := active[i]
				if pipeline.IsDropped(ar.rec) {
					continue
				}
				wc.Set(pipeline.RecordContextKey, ar.rec)
				clones := &[]*event.Event{}
				wc.Set(pipeline.CloneContextKey, clones)
				if cfg.OnFailure == "tag" {
					var tags []string
					wc.Set(pipeline.FailureTagsKey, &tags)
					for _, proc := range chain.List {
						if err := proc.Process(wc); err != nil {
							pipeline.AppendFailureTag(wc, cfg.FailureTag)
							log.Warnf("for_each: sub-chain error tagged %q (parallel): %v", cfg.FailureTag, err)
						}
					}
					mergeFailureTags(ar.rec, tags)
				} else {
					if err := chain.Process(wc); err != nil {
						log.Warnf("for_each: record error (parallel, on_failure=%s): %v", cfg.OnFailure, err)
					}
				}
				for _, cl := range pipeline.TakeClones(wc) {
					mu.Lock()
					addClone(cl, ar.msgIdx)
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()
}
