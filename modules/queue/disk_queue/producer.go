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
		result.Partition = 0
		result.Offset = queue.Offset{Segment: int64(res.Segment), Position: res.Position}
		results = append(results, result)
	}
	return &results, nil
}

func (p *Producer) Close() error {
	return nil
}
