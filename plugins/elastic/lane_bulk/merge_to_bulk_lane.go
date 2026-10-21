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

package lane_bulk

import (
	"fmt"
	"hash/fnv"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/buger/jsonparser"
	log "github.com/cihub/seelog"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/lane"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/stats"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/bytebufferpool"
	"infini.sh/framework/modules/elastic/common"
)

// autoLaneCapacity bounds the lane auto-created when no `lane:` stanza
// defines the output lane: enough slack to smooth sender hiccups, small
// enough that block-on-full engages at a sane memory ceiling.
const autoLaneCapacity = 64

// MergeToBulkLaneProcessor is the memory-path twin of indexing_merge: it
// pops JSON documents off the input disk queue, merges them into ES bulk
// NDJSON payloads (bulk_size / idle_timeout), and emits each merged payload
// as ONE record into the output lane — no output disk queue in between.
//
// Durability note: the input queue pop is the point of no return (same
// semantics as indexing_merge); a crash loses whatever is buffered in
// memory. Records travel without ack receipts — a future commit-gated
// variant can switch to EmitAck and wait on settlement without changing
// this processor's configuration contract.
//
// Backpressure: lane capacity bounds in-flight bulks; a full lane BLOCKS
// (the default overflow policy) — the merge loop stalls, then input queue
// consumption stalls. That is the intended posture for the memory path:
// pressure propagates back to the durable entry point instead of paying
// intermediate disk IO. (`overflow: spill:<queue>` remains a lane-level
// opt-out; spilled payloads are raw bulk NDJSON the legacy bulk_indexing
// can consume.)
type MergeToBulkLaneProcessor struct {
	config     MergeConfig
	failureQ   *queue.QueueConfig
	outputLane lane.Lane
	// laneCfg is the effective output-lane config (stanza-bound or the
	// auto-create default) — the blueprint for GetOrCreate re-creation.
	laneCfg lane.LaneConfig
}

func (processor *MergeToBulkLaneProcessor) Name() string {
	return "merge_to_bulk_lane"
}

type MergeConfig struct {
	InputQueue string `config:"input_queue"`
	// InputLane is the in-memory input (mutually exclusive with InputQueue):
	// records arrive as event.Event, their Fields are serialized into
	// document bytes and take the same aggregation path — written directly
	// by an upstream output{lane}, in memory end to end (the staging queue
	// is the only disk landing point).
	InputLane   string `config:"input_lane"`
	OutputLane  string `config:"output_lane"`
	IndexName   string `config:"index_name"`
	TypeName    string `config:"type_name"`
	WriteOpType string `config:"write_op_type"`
	KeyField    string `config:"key_field"`
	KeyFields   []string `config:"key_fields"`

	BulkSizeInKB         int `config:"bulk_size_in_kb"`
	BulkSizeInMB         int `config:"bulk_size_in_mb"`
	IdleTimeoutInSeconds int `config:"idle_timeout_in_seconds"`
	NumOfWorkers         int `config:"worker_size"`

	// Elasticsearch is only used to default type_name from the cluster's
	// doc type; the merge itself never talks to Elasticsearch.
	Elasticsearch string `config:"elasticsearch"`

	FailureQueue string `config:"failure_queue"`
}

func init() {
	pipeline.RegisterProcessorPluginWithConfigMetadata("merge_to_bulk_lane", NewMergeToBulkLane, MergeConfig{})
}

func NewMergeToBulkLane(c *config.Config) (pipeline.Processor, error) {
	cfg := MergeConfig{
		BulkSizeInMB:         10,
		NumOfWorkers:         1,
		IdleTimeoutInSeconds: 5,
	}

	if err := c.Unpack(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unpack the configuration of merge_to_bulk_lane processor: %s", err)
	}

	if (cfg.InputQueue == "") == (cfg.InputLane == "") {
		return nil, errors.New("merge_to_bulk_lane processor requires exactly one of input_queue | input_lane")
	}
	if cfg.OutputLane == "" {
		return nil, errors.New("merge_to_bulk_lane processor requires output_lane")
	}
	if cfg.IndexName == "" {
		return nil, errors.New("merge_to_bulk_lane processor requires index_name")
	}

	if cfg.FailureQueue == "" {
		fq := cfg.InputQueue
		if fq == "" {
			fq = cfg.InputLane
		}
		cfg.FailureQueue = fq + "_failure"
	}

	// Bind to the static lane definition (top-level `lane:` stanza) when
	// present — its capacity and overflow policy govern this producer.
	// Without one, create a block-on-full lane with a MODEST capacity:
	// merged bulks are large, and the 4096-record lane default would bound
	// memory at capacity × bulk_size (40GB at 10MB bulks) — blocking must
	// arrive at a realistic ceiling. Define an explicit stanza to tune the
	// budget (capacity × bulk_size_in_mb is the worst case).
	outputLane := lane.Get(cfg.OutputLane)
	var laneCfg lane.LaneConfig
	if outputLane != nil {
		laneCfg = outputLane.Config()
	} else {
		log.Warnf("merge_to_bulk_lane: lane %q not defined under lane:, creating block-on-full lane with capacity %d; define an explicit stanza to tune the memory budget", cfg.OutputLane, autoLaneCapacity)
		laneCfg = lane.LaneConfig{Name: cfg.OutputLane, Capacity: autoLaneCapacity}
		outputLane = lane.GetOrCreate(laneCfg)
	}

	return &MergeToBulkLaneProcessor{
		config:     cfg,
		failureQ:   queue.SmartGetOrInitConfig(&queue.QueueConfig{Name: cfg.FailureQueue}),
		outputLane: outputLane,
		laneCfg:    laneCfg,
	}, nil
}

func (processor *MergeToBulkLaneProcessor) Process(ctx *pipeline.Context) error {
	defer func() {
		if !global.Env().IsDebug {
			if r := recover(); r != nil {
				var v string
				switch r := r.(type) {
				case error:
					v = r.Error()
				case runtime.Error:
					v = r.Error()
				case string:
					v = r
				}
				log.Error("error in merge_to_bulk_lane,", v)
			}
		}
	}()

	bulkSizeInByte := 1048576 * processor.config.BulkSizeInMB
	if processor.config.BulkSizeInKB > 0 {
		bulkSizeInByte = 1024 * processor.config.BulkSizeInKB
	}

	wg := sync.WaitGroup{}
	for i := 0; i < processor.config.NumOfWorkers; i++ {
		wg.Add(1)
		go processor.newBulkWorker(ctx, bulkSizeInByte, &wg)
	}

	wg.Wait()

	return nil
}

func (processor *MergeToBulkLaneProcessor) newBulkWorker(ctx *pipeline.Context, bulkSizeInByte int, wg *sync.WaitGroup) {
	defer func() {
		if !global.Env().IsDebug {
			if r := recover(); r != nil {
				var v string
				switch r := r.(type) {
				case error:
					v = r.Error()
				case runtime.Error:
					v = r.Error()
				case string:
					v = r
				}
				log.Error("error in merge_to_bulk_lane worker,", v)
				ctx.RecordError(fmt.Errorf("merge_to_bulk_lane panic: %v", r))
			}
		}
		wg.Done()
	}()

	log.Trace("start merge_to_bulk_lane bulk worker")

	mainBuf := bytebufferpool.Get("lane_bulk_merge")
	defer bytebufferpool.Put("lane_bulk_merge", mainBuf)

	idleDuration := time.Duration(processor.config.IdleTimeoutInSeconds) * time.Second

	typeName := processor.config.TypeName
	if typeName == "" && processor.config.Elasticsearch != "" {
		// tolerant defaulting: an unavailable cluster leaves _type unset
		// (legal in the action line) instead of failing the merge task
		if meta := elastic.GetMetadata(processor.config.Elasticsearch); meta != nil {
			typeName = common.GetClusterDocType(processor.config.Elasticsearch)
		}
	}

	writeOpType := processor.config.WriteOpType
	if writeOpType == "" {
		writeOpType = "index"
	}

	var lastCommit time.Time = time.Now()

	// Input source: with input_lane, subscribe to the lane and serialize
	// Fields into document bytes; with input_queue, PopTimeout the disk
	// queue directly (the original path).
	docs := make(chan []byte, 256)
	var laneCancel func()
	if processor.config.InputLane != "" {
		l := lane.Get(processor.config.InputLane)
		if l == nil {
			l = lane.GetOrCreate(lane.LaneConfig{Name: processor.config.InputLane, Capacity: autoLaneCapacity})
		}
		laneCancel = l.SubscribeBatch(64, 200*time.Millisecond, func(recs []*event.Event) {
			for _, rec := range recs {
				if rec == nil {
					continue
				}
				b, err := util.ToJSONBytes(rec.Fields)
				if err != nil {
					log.Warnf("merge_to_bulk_lane: skip record (serialize fields): %v", err)
					continue
				}
				select {
				case <-ctx.Done():
					return
				case docs <- b:
				}
			}
		})
		defer laneCancel()
	}

READ_DOCS:
	for {

		if ctx.IsCanceled() {
			goto CLEAN_BUFFER
		}

		var pop []byte
		if processor.config.InputLane != "" {
			select {
			case <-ctx.Done():
				goto CLEAN_BUFFER
			case pop = <-docs:
			}
		} else {
			var err error
			pop, _, err = queue.PopTimeout(queue.GetOrInitConfig(processor.config.InputQueue), idleDuration)
			if err != nil {
				log.Error(err)
				panic(err)
			}
		}

		if len(pop) > 0 {

			var id_part = ""
			if len(processor.config.KeyFields) > 0 {
				id_part = fmt.Sprintf(", \"_id\":\"%v\"", processor.fingerprintID(pop))
			} else if processor.config.KeyField != "" {
				// zero-alloc path query: extracting ONE field
				// must not float-parse the whole document (embeddings!)
				v, _, _, err := jsonparser.Get(pop, strings.Split(processor.config.KeyField, ".")...)
				if err != nil {
					log.Warnf("merge_to_bulk_lane: key_field %q not found: %v", processor.config.KeyField, err)
				} else if s, ok := trimJSONString(v); ok {
					id_part = fmt.Sprintf(", \"_id\":\"%v\"", s)
				} else {
					id_part = fmt.Sprintf(", \"_id\":\"%v\"", string(v))
				}
			}

			if typeName != "" {
				mainBuf.WriteString(fmt.Sprintf("{ \"%s\" : { \"_index\" : \"%s\", \"_type\" : \"%s\" %v } }\n", writeOpType, processor.config.IndexName, typeName, id_part))
			} else {
				mainBuf.WriteString(fmt.Sprintf("{ \"%s\" : { \"_index\" : \"%s\"  %v } }\n", writeOpType, processor.config.IndexName, id_part))
			}

			util.WalkBytesAndReplace(pop, util.NEWLINE, util.SPACE)

			// Data stream (create) documents must carry a top-level date
			// @timestamp; the otel envelope keeps the timestamp at the
			// envelope level and the record body under "payload", which
			// neither reaches the doc top level. Normalize envelope docs
			// for data stream semantics; bare docs pass through as-is.
			if writeOpType == "create" {
				pop = normalizeDataStreamDoc(pop)
			}

			mainBuf.Write(pop)
			mainBuf.WriteString("\n")
		}

		//submit no matter the size of bulk after idle timeout
		if time.Since(lastCommit) > idleDuration && mainBuf.Len() > 0 {
			if global.Env().IsDebug {
				log.Trace("hit idle timeout, ", idleDuration.String())
			}
			goto CLEAN_BUFFER
		}

		if mainBuf.Len() > (bulkSizeInByte) {
			if global.Env().IsDebug {
				log.Trace("hit buffer size, ", mainBuf.Len())
			}
			goto CLEAN_BUFFER
		}

	}

CLEAN_BUFFER:

	lastCommit = time.Now()

	if mainBuf.Len() > 0 {
		mainBuf.WriteByte('\n')

		// the record outlives this reusable buffer — hand over an
		// exact-size copy, then reset
		bulk := make([]byte, mainBuf.Len())
		copy(bulk, mainBuf.Bytes())
		mainBuf.Reset()

		rec := newBulkRecord(bulk, "bulk-"+util.GetUUID())
		// Re-acquire per flush: the consumer task (bulk_indexing_lane)
		// DELETES the lane on exit; GetOrCreate re-creates it when absent
		// or closed. A stale cached handle would emit-fail "closed"
		// forever across the consumer's stop/restart cycle — re-acquiring
		// here makes that cycle self-healing (bulks buffer in the fresh
		// lane while the consumer is down, then drain on its restart).
		processor.outputLane = lane.GetOrCreate(processor.laneCfg)
		// Emit (not EmitAck): there is no commit gate on this host to
		// attach receipts to, and Emit is the variant that reports the
		// closed-lane failure the salvage path below needs
		if err := processor.outputLane.Emit(rec); err != nil {
			// closed lane (sender task stopped first): salvage the merged
			// payload on the failure queue so a replay path exists
			log.Errorf("merge_to_bulk_lane: emit to lane %q failed: %v", processor.config.OutputLane, err)
			stats.Increment("merge_to_bulk_lane", "salvaged")
			if err := queue.Push(processor.failureQ, bulk); err != nil {
				log.Errorf("merge_to_bulk_lane: push to failure queue %q failed: %v", processor.failureQ.Name, err)
			}
		} else {
			stats.Increment("merge_to_bulk_lane", "emitted")
			stats.IncrementBy("merge_to_bulk_lane", "emitted_bytes", int64(len(bulk)))
		}

		log.Trace("clean buffer")
	}

	if ctx.IsCanceled() {
		return
	}
	goto READ_DOCS
}

// fingerprintID derives a deterministic document id: the configured dot-path
// fields are looked up in the queue message, joined and hashed (FNV-1a 64).
// When NONE of the fields resolve, the raw message bytes are hashed instead —
// the fallback keeps distinct documents apart for sources lacking e.g. file
// metadata. Deterministic ids make at-least-once replays idempotent when
// combined with write_op_type=create: duplicated deliveries turn into
// ES version conflicts instead of duplicate documents.
//
// NOTE the fields must be stable across redeliveries: per-attempt stamps like
// ingest timestamps change between retries and defeat dedup; content-derived
// or source-location fields (file path/offset) do not.
func (processor *MergeToBulkLaneProcessor) fingerprintID(pop []byte) string {
	h := fnv.New64a()

	// zero-alloc path queries: fingerprinting must not
	// float-parse the whole document (embeddings ride these messages)
	matched := false
	for _, f := range processor.config.KeyFields {
		h.Write([]byte{'|'})
		v, _, _, err := jsonparser.Get(pop, strings.Split(f, ".")...)
		if err != nil {
			// keep positional meaning for missing fields
			h.Write([]byte("\x00"))
			continue
		}
		matched = true
		h.Write(v) // raw JSON token: any scalar type hashes deterministically
	}
	if !matched {
		// none of the key fields exist on this message: fall back to a
		// content hash so unrelated messages don't collapse into one id
		h.Reset()
		h.Write(pop)
	}
	return fmt.Sprintf("%x", h.Sum64())
}

// trimJSONString strips JSON string quotes from a raw token ("" and false
// when the token is not a quoted string).
func trimJSONString(v []byte) (string, bool) {
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		return string(v[1 : len(v)-1]), true
	}
	return "", false
}

// normalizeDataStreamDoc turns one otel envelope (the queue's LogEvent JSON:
// {"metadata":{...},"payload":{...},"timestamp":"..."}) into a data stream
// compatible document:
//
//   - payload fields are promoted to the top level (message etc. become
//     first-class doc fields)
//   - a top-level @timestamp is derived from payload.timestamp /
//     payload.observed_timestamp / the envelope timestamp / now — data
//     streams reject documents without a date @timestamp. @timestamp is the
//     document's one true timestamp (house standard): the mirrored top-level
//     "timestamp" and any payload "timestamp" are folded into it and removed.
//   - a bare document (no "payload" key) keeps its own @timestamp when
//     present; only a missing one is stamped with now.
//   - metadata.file (agent collection origin) lives in @agent.file_* — the
//     former top-level "file" promotion is gone (zero readers).
//
// Documents that are not envelopes (no "payload" key) pass through
// unchanged except for the @timestamp stamp.
func normalizeDataStreamDoc(doc []byte) []byte {
	var m util.MapStr
	if err := util.FromJSONBytes(doc, &m); err != nil {
		return doc
	}

	ts := ""
	if payload, ok := m["payload"].(map[string]interface{}); ok {
		for _, k := range []string{"timestamp", "@timestamp", "observed_timestamp"} {
			if v, ok := payload[k].(string); ok && v != "" {
				ts = v
				break
			}
		}
	}
	if ts == "" {
		if v, ok := m["timestamp"].(string); ok {
			ts = v
		}
	}
	if ts == "" {
		if v, ok := m["@timestamp"].(string); ok { // bare doc already carrying one
			ts = v
		}
	}
	if ts == "" {
		ts = time.Now().UTC().Format(time.RFC3339Nano)
	}

	out := util.MapStr{}
	if payload, ok := m["payload"].(map[string]interface{}); ok {
		for k, v := range payload {
			out[k] = v
		}
	} else {
		for k, v := range m {
			out[k] = v
		}
	}
	out["@timestamp"] = ts
	delete(out, "timestamp") // @timestamp is the single source of truth

	if meta, ok := m["metadata"].(map[string]interface{}); ok {
		if rt, ok := meta["log_type"].(string); ok && rt != "" {
			out["log_type"] = rt
		}
		// per-pattern label (e.g. server|deprecation|slowlog|gc) — virtual
		// streams split on this field.
		if lk, ok := meta["log_kind"].(string); ok && lk != "" {
			out["log_kind"] = lk
		}
	}
	return util.MustToJSONBytes(out)
}
