// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
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

/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package kafka_queue

import (
	"context"
	log "github.com/cihub/seelog"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/locker"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
	"sync"
	"time"
)

type Consumer struct {
	qCfg *queue.QueueConfig
	cCfg *queue.ConsumerConfig

	client *kgo.Client

	// mu guards partNext. The fetch loop runs on the worker goroutine, but
	// commits also arrive from admin API goroutines (the queue offset
	// reset endpoints dispatch onto this cached consumer) and rebalance
	// callbacks run on the client's own goroutines — unsynchronized map
	// access here is a fatal concurrent-read panic.
	mu sync.Mutex

	// partNext tracks the per-partition progress (next offset to consume)
	// of fetched-but-uncommitted records. One fetch batch can mix
	// partitions while the framework's commit carries a single
	// (partition, offset) pair — the batch's last record — so committing
	// that pair alone drops every other partition's progress and the
	// group refetches it (duplicate delivery). CommitOffset flushes the
	// whole map once the acknowledged position catches up with the fetch
	// buffer; see buildCommitRequest for why it must not flush when the
	// acknowledged position trails.
	partNext map[int32]int64
}

func (this *Consumer) Close() error {
	this.client.Close()
	return nil
}

func (this *Consumer) ResetOffset(part, readPos int64) (err error) {
	if global.Env().IsDebug {
		log.Debugf("reset %v offset to %v for partition %v", this.qCfg.ID, readPos, part)
	}
	req := map[string]map[int32]kgo.EpochOffset{}
	req[this.qCfg.ID] = map[int32]kgo.EpochOffset{}
	req[this.qCfg.ID][int32(part)] = kgo.EpochOffset{Offset: readPos, Epoch: -1}
	this.client.SetOffsets(req)
	// keep the tracked progress in sync with the rewind, otherwise a later
	// commit would jump the partition forward past the reset position
	this.mu.Lock()
	if this.partNext != nil && readPos < this.partNext[int32(part)] {
		this.partNext[int32(part)] = readPos
	}
	this.mu.Unlock()
	return err
}

func (this *Consumer) CommitOffset(off queue.Offset) error {

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*10))
	defer cancel()

	// snapshot the commit request under the lock; the network commit runs
	// outside it so a slow broker cannot stall the fetch loop
	this.mu.Lock()
	req, drained := this.buildCommitRequest(off)
	this.mu.Unlock()

	if len(req) == 0 {
		return nil
	}

	var ret error
	this.client.CommitOffsetsSync(ctx, req, func(client *kgo.Client, request *kmsg.OffsetCommitRequest, response *kmsg.OffsetCommitResponse, err error) {
		ret = err
	})

	if ret == nil && drained {
		// fold the committed positions back into the tracked progress so a
		// later explicit commit cannot regress below them. A non-drained
		// commit already rewound the partition inside buildCommitRequest.
		this.mu.Lock()
		for p, eo := range req[this.qCfg.ID] {
			if this.partNext != nil && eo.Offset > this.partNext[p] {
				this.partNext[p] = eo.Offset
			}
		}
		this.mu.Unlock()
	}

	if global.Env().IsDebug {
		log.Infof("commit %v[%v] offset: %v", this.qCfg.Name, this.qCfg.ID, off.String())
	}
	return ret
}

// buildCommitRequest builds the kgo commit request for one queue offset
// from the tracked per-partition progress (partNext). The caller must hold
// mu. Returns the request and whether the acknowledged position drained
// the fetch buffer.
//
// The partition comes from the offset itself (queue offsets carry
// segment=partition since FetchMessages): committing hard-wired to
// partition 0 silently corrupted the committed positions of every
// multi-partition topic — each partition's progress overwrote partition
// 0's commit and its own was never persisted.
//
// The acknowledged position also decides how much progress is safe to
// flush. Framework commits carry the next offset of the last record of
// the last fetched batch (ctx.NextOffset):
//
//   - caught up with partNext for its partition: every fetched batch was
//     processed, flushing the whole map commits exactly the processed
//     records of all partitions in the buffer.
//   - trailing partNext: the batch was interrupted mid-processing (the
//     framework keeps the previous offset for at-least-once redelivery)
//     or this is an explicit rewind (admin reset). Only the requested
//     position is committed and the partition's tracking is rewound to
//     it — flushing the map here would permanently skip records that
//     were fetched but never processed, or silently ignore the reset.
func (this *Consumer) buildCommitRequest(off queue.Offset) (map[string]map[int32]kgo.EpochOffset, bool) {
	offset := map[string]map[int32]kgo.EpochOffset{}
	parts := map[int32]kgo.EpochOffset{}
	drained := true
	if off.Segment >= 0 && this.partNext[int32(off.Segment)] > off.Position {
		drained = false
	}
	if drained {
		for p, o := range this.partNext {
			if o > 0 {
				parts[p] = kgo.EpochOffset{Offset: o, Epoch: -1}
			}
		}
		if off.Segment >= 0 {
			p := int32(off.Segment)
			if cur, ok := parts[p]; !ok || off.Position > cur.Offset {
				parts[p] = kgo.EpochOffset{Offset: off.Position, Epoch: -1}
			}
		}
	} else {
		// commit exactly the requested position and rewind tracking to it;
		// the other partitions' progress stays uncommitted and their
		// records are redelivered (at-least-once)
		p := int32(off.Segment)
		parts[p] = kgo.EpochOffset{Offset: off.Position, Epoch: -1}
		this.partNext[p] = off.Position
	}
	if len(parts) > 0 {
		offset[this.qCfg.ID] = parts
	}
	return offset, drained
}

// dropPartitionProgress forgets tracked progress for partitions this
// consumer no longer owns (rebalance revoked, fencing lost): a later
// commit flushing the stale entries would overwrite the new owner's
// progress with this consumer's old positions.
func (this *Consumer) dropPartitionProgress(revoked map[string][]int32) {
	this.mu.Lock()
	defer this.mu.Unlock()
	if this.partNext == nil {
		return
	}
	for topic, parts := range revoked {
		if topic != this.qCfg.ID {
			continue
		}
		for _, p := range parts {
			delete(this.partNext, p)
		}
	}
}

func (this *Consumer) FetchMessages(ctx *queue.Context, numOfMessages int) (messages []queue.Message, isTimeout bool, err error) {

	timeoutDuration := time.Duration(this.cCfg.FetchMaxWaitMs) * time.Millisecond
	//place lock
	k := []byte(getGroupForKafka(this.cCfg.Group, this.qCfg.ID))
	ok, err := locker.Hold(queue.BucketWhoOwnsThisTopic, string(k), global.Env().SystemConfig.NodeConfig.ID, time.Duration(this.cCfg.ClientExpiredInSeconds)*time.Second, true)
	if !ok || err != nil {
		panic("failed to hold lock for topic: " + this.qCfg.ID)
	}

	if global.Env().IsDebug {
		log.Debugf("start to fetch message from queue: %v[%v], %v", this.qCfg.Name, this.qCfg.ID, this.cCfg.ID)
	}

	ctx1, cancel := context.WithTimeout(context.Background(), timeoutDuration)
	defer cancel()

	msgs := []queue.Message{}
	ctx.MessageCount = 0
	batchNext := map[int32]int64{}

	byteSize := 0
	start := time.Now()
	var nextOffset queue.Offset
	var firstOffset queue.Offset
	for {
		var fetches kgo.Fetches
		if numOfMessages > 0 {
			fetches = this.client.PollRecords(ctx1, numOfMessages)
		} else {
			fetches = this.client.PollFetches(ctx1)
		}
		if fetches.IsClientClosed() {
			return
		}

		if global.ShuttingDown() {
			log.Debugf("system is shutting down, consume [%v] will be stopped", this.qCfg.Name)
			break
		}

		fetches.EachRecord(func(r *kgo.Record) {
			if global.Env().IsDebug {
				log.Tracef(string(r.Key), "from an iterator!,offset:", r.Offset, ",partition:", r.Partition, ", get message from queue:%v,consumer:%v, ctx:%v, offset:%v", this.qCfg.Name, this.cCfg.Key(), ctx.String(), r.Offset)
			}

			offsetStr := queue.NewOffset(int64(r.Partition), r.Offset)
			nextOffsetStr := queue.NewOffset(int64(r.Partition), r.Offset+1)
			if next := r.Offset + 1; next > batchNext[r.Partition] {
				batchNext[r.Partition] = next
			}
			if ctx.MessageCount == 0 {
				firstOffset = offsetStr
			}
			nextOffset = nextOffsetStr
			size := len(r.Value)
			m := queue.Message{Offset: offsetStr, NextOffset: nextOffsetStr, Data: r.Value, Size: size, Timestamp: r.Timestamp.Unix()}
			msgs = append(msgs, m)
			ctx.MessageCount++
			byteSize += size

			if ctx.MessageCount >= numOfMessages {
				cancel()
			}

			if (numOfMessages > 0 && ctx.MessageCount > numOfMessages) ||
				(this.cCfg.FetchMaxMessages > 0 && ctx.MessageCount >= this.cCfg.FetchMaxMessages) ||
				(this.cCfg.FetchMaxBytes > 0 && byteSize >= this.cCfg.FetchMaxBytes) ||
				(this.cCfg.FetchMaxWaitMs > 0 && time.Since(start).Milliseconds() > this.cCfg.FetchMaxWaitMs) {
				log.Trace(this.cCfg.ID, " hit enough message, ", ctx.MessageCount, ",offset:", offsetStr, ",byte:", byteSize, ",elapsed:", time.Since(start).Milliseconds(), ",", util.MustToJSON(this.cCfg))
				return
			}
		})

		// merge this poll's per-partition progress into the tracked map
		// once, under the lock: commits and rebalance callbacks run on
		// other goroutines
		this.mu.Lock()
		for p, next := range batchNext {
			if next > this.partNext[p] {
				this.partNext[p] = next
			}
		}
		this.mu.Unlock()

		if errs := fetches.Errors(); len(errs) > 0 {
			// All errors are retried internally when fetching, but non-retriable errors are
			// returned from polls so that users can notice and take action.
			break
		}
	}

	if ctx.MessageCount == 0 {
		if global.Env().IsDebug {
			offset, err := queue.GetOffset(this.qCfg, this.cCfg)
			log.Infof("no message in queue: %v[%v], %v, %v->%v, %v,%v", this.qCfg.Name, this.qCfg.ID, this.cCfg.ID, firstOffset, nextOffset, offset, err)
		}
		return nil, true, nil
	}

	ctx.InitOffset = firstOffset
	ctx.NextOffset = nextOffset

	if global.Env().IsDebug {
		log.Debug(this.qCfg.Name, "[", this.qCfg.ID, "],", this.cCfg.ID, ",msg:", len(msgs), ",first:", firstOffset, ",last:", nextOffset)
	}

	return msgs, false, nil
}
