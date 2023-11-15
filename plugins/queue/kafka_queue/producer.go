/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package kafka_queue

import (
	"context"
	"github.com/twmb/franz-go/pkg/kgo"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
	"time"
)

type Producer struct {
	ID string
	client *kgo.Client
	cfg    *queue.QueueConfig
}

func (p *Producer) Produce(reqs *[]queue.ProduceRequest) (*[]queue.ProduceResponse, error) {

	if p.client == nil || reqs == nil {
		panic(errors.New("invalid request"))
	}

	messages := []*kgo.Record{}
	for _, req := range *reqs {
		msg := &kgo.Record{}
		if req.Topic!=""{
			msg.Topic=req.Topic
		}else{
			msg.Topic=p.cfg.ID
		}
		msg.Timestamp = time.Now()
		msg.Key = util.UnsafeStringToBytes(util.GetUUID())
		msg.Value = req.Data
		messages = append(messages, msg)
	}

	results := []queue.ProduceResponse{}


	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*10))
	defer cancel()

	response := p.client.ProduceSync(ctx, messages...)

	for _, r := range response {
		if r.Err == nil {
			if r.Record != nil {
				result := queue.ProduceResponse{}
				result.Offset = queue.Offset{Segment: int64(r.Record.Partition), Position: r.Record.Offset}
				result.Timestamp = r.Record.Timestamp.Unix()
				result.Topic = r.Record.Topic
				result.Partition = int64(r.Record.Partition)
				results = append(results, result)
			}
		}
	}
	return &results, response.FirstErr()
}

func (p *Producer) Close() error {
	return nil
}
