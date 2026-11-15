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
	"time"
)

type Consumer struct {
	qCfg *queue.QueueConfig
	cCfg *queue.ConsumerConfig

	client *kgo.Client

	// partNext tracks the per-partition progress (next offset to consume)
	// of the batches fetched since the last successful commit. One fetch
	// batch can mix partitions while the framework's commit carries a
	// single (partition, offset) pair — the batch's last record — so
	// committing that pair alone drops every other partition's progress
	// and the group refetches it (duplicate delivery). CommitOffset
	// flushes the whole map: every record of a committed batch was
	// processed by the same successful HandleMessages call.
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
	if this.partNext != nil && readPos < this.partNext[int32(part)] {
		this.partNext[int32(part)] = readPos
	}
	return err
}

func (this *Consumer) CommitOffset(off queue.Offset) error {

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*10))
	defer cancel()

	req := this.offsetsToCommit(off)
	if len(req) == 0 {
		return nil
	}

	var ret error
	this.client.CommitOffsetsSync(ctx, req, func(client *kgo.Client, request *kmsg.OffsetCommitRequest, response *kmsg.OffsetCommitResponse, err error) {
		ret = err
	})

	if ret == nil {
		// fold the committed positions back into the tracked progress so a
		// later explicit commit cannot regress below them
		for p, eo := range req[this.qCfg.ID] {
			if eo.Offset > this.partNext[p] {
				this.partNext[p] = eo.Offset
			}
		}
	}

	if global.Env().IsDebug {
		log.Infof("commit %v[%v] offset: %v", this.qCfg.Name, this.qCfg.ID, off.String())
	}
	return ret
}

// offsetsToCommit builds the kgo commit request for one queue offset: the
// offset itself plus the per-partition progress of every fetched-but-
// uncommitted batch (see partNext). The partition comes from the offset
// itself (queue offsets carry segment=partition since FetchMessages):
// committing hard-wired to partition 0 silently corrupted the committed
// positions of every multi-partition topic — each partition's progress
// overwrote partition 0's commit and its own was never persisted.
func (this *Consumer) offsetsToCommit(off queue.Offset) map[string]map[int32]kgo.EpochOffset {
	offset := map[string]map[int32]kgo.EpochOffset{}
	parts := map[int32]kgo.EpochOffset{}
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
	if len(parts) > 0 {
		offset[this.qCfg.ID] = parts
	}
	return offset
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
			if next := r.Offset + 1; next > this.partNext[r.Partition] {
				this.partNext[r.Partition] = next
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
