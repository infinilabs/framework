/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package queue

import (
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/queue"
	"time"
)

type Producer struct {
	q               *DiskBasedQueue
	cfg             *queue.QueueConfig
	diskQueueConfig *DiskQueueConfig
}

func (p *Producer) Produce(reqs *[]queue.ProduceRequest) (*[]queue.ProduceResponse, error) {
	results := []queue.ProduceResponse{}
	for _, req := range *reqs {
		msgSize := len(req.Data)
		if int32(msgSize) < p.diskQueueConfig.MinMsgSize || int32(msgSize) > p.diskQueueConfig.MaxMsgSize {
			return &results, errors.Errorf("queue:%v, invalid message size: %v, should between: %v TO %v", p.cfg.ID, msgSize, p.diskQueueConfig.MinMsgSize, p.diskQueueConfig.MaxMsgSize)
		}

		if req.Topic == "" {
			panic(errors.New("topic is required"))
		}

		if req.Topic != p.cfg.ID {
			panic(errors.Errorf("invalid topic: %v vs %v", req.Topic, p.cfg.ID))
		}

		res := p.q.Put(req.Data)
		if res.Error != nil {
			return &results, res.Error
		}

		result := queue.ProduceResponse{}
		result.Timestamp = time.Now().Unix()
		result.Topic = p.cfg.ID
		result.Partition =0
		result.Offset =queue.Offset{Segment: int64(res.Segment), Position: res.Position}
		results = append(results, result)
	}
	return &results, nil
}

func (p *Producer) Close() error {
	return nil
}
