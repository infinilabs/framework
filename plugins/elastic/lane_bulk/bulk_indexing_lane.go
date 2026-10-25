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
	"sync"
	"time"

	log "github.com/cihub/seelog"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/lane"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/stats"
	"infini.sh/framework/core/util"
)

// BulkIndexingLaneProcessor is the memory-path twin of bulk_indexing: it
// subscribes to a lane of merged bulk records (merge_to_bulk_lane's output)
// and sends each payload to Elasticsearch. Receipts of a batch settle when
// its sends complete (the SubscribeBatch settlement contract), which is
// also the settlement point for the merge side's EmitAck receipts.
//
// Failure policy: a failed send retries after retry_delay_in_ms. Without a
// failure_queue retries continue indefinitely — a down cluster stalls the
// lane, which backpressures the merge, which stalls input queue consumption
// (no silent loss, matching legacy bulk_indexing's stall semantics). With a
// failure_queue set, a payload diverts there after max_retries attempts and
// the lane keeps flowing.
type BulkIndexingLaneProcessor struct {
	config        SenderConfig
	inputLane     lane.Lane
	failureQ      *queue.QueueConfig
	bulkProcessor elastic.BulkProcessor
	bufferPool    *elastic.BulkBufferPool
}

func (processor *BulkIndexingLaneProcessor) Name() string {
	return "bulk_indexing_lane"
}

type SenderConfig struct {
	Lane          string `config:"lane"`
	Elasticsearch string `config:"elasticsearch"`

	Bulk elastic.BulkProcessorConfig `config:"bulk"`

	FailureQueue   string `config:"failure_queue"`
	MaxRetries     int    `config:"max_retries"`
	RetryDelayInMs int    `config:"retry_delay_in_ms"`

	// WorkerSize sends one batch's payloads concurrently across workers;
	// the batch function waits for every send before returning, keeping
	// receipt settlement exact.
	WorkerSize int `config:"worker_size"`

	// BatchFlushMs flushes a partial batch after this idle window. Must
	// be > 0: without a timer, a batch below batch_max never reaches its
	// size trigger and its records would sit undelivered.
	BatchFlushMs int `config:"batch_flush_ms"`
}

func init() {
	pipeline.RegisterProcessorPluginWithConfigMetadata("bulk_indexing_lane", NewBulkIndexingLane, SenderConfig{})
}

func NewBulkIndexingLane(c *config.Config) (pipeline.Processor, error) {
	cfg := SenderConfig{
		RetryDelayInMs: 3000,
		WorkerSize:     1,
		BatchFlushMs:   100,
	}

	if err := c.Unpack(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unpack the configuration of bulk_indexing_lane processor: %s", err)
	}

	if cfg.Lane == "" {
		return nil, errors.New("bulk_indexing_lane processor requires lane")
	}
	if cfg.Elasticsearch == "" {
		return nil, errors.New("bulk_indexing_lane processor requires elasticsearch")
	}
	if cfg.RetryDelayInMs <= 0 {
		cfg.RetryDelayInMs = 3000
	}
	if cfg.WorkerSize <= 0 {
		cfg.WorkerSize = 1
	}
	if cfg.BatchFlushMs <= 0 {
		cfg.BatchFlushMs = 100
	}
	if cfg.FailureQueue != "" && cfg.MaxRetries <= 0 {
		// a failure queue without a retry budget diverts on first failure;
		// make the budget explicit instead of guessing
		return nil, errors.New("bulk_indexing_lane processor requires max_retries > 0 when failure_queue is set")
	}

	// GetOrCreate (not Get+error): tolerate the merge task not having run
	// yet (startup ordering), and re-create the lane when a previous task
	// instance closed it on exit — a bare Get would keep returning the
	// closed instance and the task would silently idle forever. Capacity
	// matches merge_to_bulk_lane's auto-create budget (merged bulks are
	// large; an explicit `lane:` stanza overrides when present).
	inputLane := lane.GetOrCreate(lane.LaneConfig{Name: cfg.Lane, Capacity: autoLaneCapacity})

	var failureQ *queue.QueueConfig
	if cfg.FailureQueue != "" {
		failureQ = queue.SmartGetOrInitConfig(&queue.QueueConfig{Name: cfg.FailureQueue})
	}

	return &BulkIndexingLaneProcessor{
		config:        cfg,
		inputLane:     inputLane,
		failureQ:      failureQ,
		bulkProcessor: elastic.NewBulkProcessor("bulk_indexing_lane", cfg.Elasticsearch, cfg.Bulk),
		bufferPool:    elastic.NewBulkBufferPool("bulk_indexing_lane", 1024*1024*1024, 100000),
	}, nil
}

// Process subscribes (once) and blocks until the pipeline task cancels —
// the host drives the lane for its whole lifetime. On exit the lane is
// DELETED (closed + unregistered), not merely closed: Close alone leaves a
// permanently-closed instance in the registry that wedges the producer
// (merge_to_bulk_lane's emits fail "closed" forever) and breaks this task's
// own restart (a re-Get would return the dead instance and the subscriber
// would exit immediately). Delete + the producers'/consumers' GetOrCreate
// re-creation makes a task stop/restart a recoverable cycle: the drain
// semantics of Close are preserved (buffered records flush through the
// batch function, in-flight receipts settle).
func (processor *BulkIndexingLaneProcessor) Process(ctx *pipeline.Context) error {
	meta := processor.waitMetadata(ctx)
	if meta == nil {
		return nil
	}

	// batch width tracks the worker fan-out: batch_max payloads are picked
	// up per flush (size trigger, or batch_flush_ms idle trigger — lone
	// records below batch_max rely on the timer) and sent
	// worker_size-at-a-time
	batchMax := processor.config.WorkerSize * 2
	_ = processor.inputLane.SubscribeBatch(batchMax, time.Duration(processor.config.BatchFlushMs)*time.Millisecond, func(recs []*event.Event) {
		processor.sendBatch(ctx, meta, recs)
	})

	<-ctx.Done()
	_ = lane.Delete(processor.config.Lane, true)
	return nil
}

// sendBatch delivers one micro-batch: records are dealt across workers,
// every send completes (or diverts) before returning — receipts settle only
// after that (SubscribeBatch contract), so settlement means "delivered or
// salvaged", never "still pending".
func (processor *BulkIndexingLaneProcessor) sendBatch(ctx *pipeline.Context, meta *elastic.ElasticsearchMetadata, recs []*event.Event) {
	workers := processor.config.WorkerSize
	if workers > len(recs) {
		workers = len(recs)
	}
	if workers <= 1 {
		for _, rec := range recs {
			processor.sendRecord(ctx, meta, rec)
		}
		return
	}

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(chunk []*event.Event) {
			defer wg.Done()
			for _, rec := range chunk {
				processor.sendRecord(ctx, meta, rec)
			}
		}(recs[w:])
	}
	wg.Wait()
}

// waitMetadata blocks until the cluster metadata is available (startup
// ordering: the elastic module may finish after pipeline tasks) or the task
// cancels. Stalling here is safe — nothing has been consumed yet.
func (processor *BulkIndexingLaneProcessor) waitMetadata(ctx *pipeline.Context) *elastic.ElasticsearchMetadata {
	for {
		if ctx.IsCanceled() {
			return nil
		}
		if meta := elastic.GetMetadata(processor.config.Elasticsearch); meta != nil {
			return meta
		}
		log.Warnf("bulk_indexing_lane: cluster metadata [%v] not ready, waiting", processor.config.Elasticsearch)
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return nil
		}
	}
}

// sendRecord delivers one merged bulk payload with the configured retry
// policy. On task cancellation each in-flight record gets ONE final attempt
// (the legacy final-submit behavior) and unresolved payloads divert to the
// failure queue when one is configured.
func (processor *BulkIndexingLaneProcessor) sendRecord(ctx *pipeline.Context, meta *elastic.ElasticsearchMetadata, rec *event.Event) {
	bulk, ok := bulkPayloadOf(rec)
	if !ok {
		log.Warnf("bulk_indexing_lane: record without bulk payload (trace %q), skip", pipeline.TraceID(rec))
		stats.Increment("bulk_indexing_lane", "skipped")
		return
	}

	id := pipeline.TraceID(rec)
	if id == "" {
		id = "bulk-" + util.GetUUID()
	}

	// shutdown drain: one best-effort attempt, then salvage — retrying
	// forever here would hang the task teardown
	final := ctx.IsCanceled() || global.ShuttingDown()

	for attempt := 0; ; attempt++ {
		if !final && (ctx.IsCanceled() || global.ShuttingDown()) {
			final = true
		}

		host := meta.GetActivePreferredSeedHost()
		if elastic.IsHostDead(host) || !elastic.IsHostAvailable(host) {
			host = meta.GetActiveHost()
		}

		buf := processor.bufferPool.AcquireBulkBuffer()
		buf.Queue = processor.config.Elasticsearch
		buf.WriteMessageID(id)
		buf.Write(bulk)
		buf.SafetyEndWithNewline()

		continueNext, statsRet, _, err := processor.bulkProcessor.Bulk(ctx.Context, "bulk_indexing_lane", meta, host, buf)
		buf.ResetData()
		processor.bufferPool.ReturnBulkBuffer(buf)

		for code, n := range statsRet {
			stats.IncrementBy("bulk_indexing_lane", "response_code_"+util.ToString(code), int64(n))
		}

		if err == nil {
			stats.Increment("bulk_indexing_lane", "sent")
			stats.IncrementBy("bulk_indexing_lane", "sent_bytes", int64(len(bulk)))
			if !continueNext {
				log.Warnf("bulk_indexing_lane: cluster suggested backoff (continue=false)")
			}
			return
		}

		stats.Increment("bulk_indexing_lane", "failed")
		log.Errorf("bulk_indexing_lane: bulk send failed (host: %v, attempt: %v, err: %v)", host, attempt+1, err)

		// only a configured failure queue ends the retry loop early;
		// without one a hard-down cluster stalls the lane (intended)
		if final || (processor.failureQ != nil && attempt+1 >= processor.config.MaxRetries) {
			break
		}

		select {
		case <-time.After(time.Duration(processor.config.RetryDelayInMs) * time.Millisecond):
		case <-ctx.Done():
			continue // loop top re-checks cancellation and flags final
		}
	}

	if processor.failureQ != nil {
		if err := queue.Push(processor.failureQ, bulk); err != nil {
			log.Errorf("bulk_indexing_lane: push to failure queue %q failed: %v", processor.failureQ.Name, err)
		} else {
			stats.Increment("bulk_indexing_lane", "diverted")
		}
		return
	}
	if ctx.IsCanceled() || global.ShuttingDown() {
		// no salvage path configured on the shutdown drain — visible in
		// stats rather than silent
		stats.Increment("bulk_indexing_lane", "lost_on_shutdown")
	}
}
