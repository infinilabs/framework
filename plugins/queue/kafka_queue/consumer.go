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
	"infini.sh/framework/core/stats"
	"infini.sh/framework/core/util"
	"time"
)

type Consumer struct {
	qCfg *queue.QueueConfig
	cCfg *queue.ConsumerConfig

	client *kgo.Client
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
	req[this.qCfg.ID][int32(part)] = kgo.EpochOffset{Offset: readPos, Epoch: 0}
	this.client.SetOffsets(req)
	//this.client.CommitOffsetsSync(context.Background(),req, func(client *kgo.Client, request *kmsg.OffsetCommitRequest, response *kmsg.OffsetCommitResponse, err error) {
	//	if err!=nil{
	//		panic(err)
	//	}
	//})
	return err
}

func (this *Consumer) CommitOffset(off queue.Offset) error {

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*10))
	defer cancel()

	var ret error
	offset := map[string]map[int32]kgo.EpochOffset{}
	offset[this.qCfg.ID] = map[int32]kgo.EpochOffset{}
	offset[this.qCfg.ID][0] = kgo.EpochOffset{Offset: off.Position, Epoch: 0}
	this.client.CommitOffsetsSync(ctx, offset, func(client *kgo.Client, request *kmsg.OffsetCommitRequest, response *kmsg.OffsetCommitResponse, err error) {
		if ret != nil {
			log.Error(ret)
		}
		ret = err
	})

	if global.Env().IsDebug {
		log.Infof("commit %v[%v] offset: %v", this.qCfg.Name, this.qCfg.ID, off.String())
	}
	return ret
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

	defer stats.IncrementBy(this.qCfg.ID, "fetched_message", int64(ctx.MessageCount))

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
				log.Tracef(string(r.Key), "from an iterator!,offset:", r.Offset, ",partition:", r.Partition,", get message from queue:%v,consumer:%v, ctx:%v, offset:%v",this.qCfg.Name,this.cCfg.Key(),ctx.String(),r.Offset)
			}

			offsetStr := queue.NewOffset(int64(r.Partition), r.Offset)
			nextOffsetStr := queue.NewOffset(int64(r.Partition), r.Offset+1)
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
