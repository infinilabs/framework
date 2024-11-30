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

/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package merge_to_bulk

import (
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/param"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/bytebufferpool"
	"infini.sh/framework/modules/elastic/common"
	"sync"
)

type IndexingMergeToBulkProcessor struct {
	initLocker        sync.RWMutex
	config            Config
	outputQueueConfig *queue.QueueConfig
	metadata          *elastic.ElasticsearchMetadata
	bulkSizeInByte    int
	producer          queue.ProducerAPI
}

// 处理纯 json 格式的消息索引
func (processor *IndexingMergeToBulkProcessor) Name() string {
	return "merge_to_bulk"
}

type Config struct {
	MessageField param.ParaKey `config:"message_field"`

	BulkSizeInKB  int    `config:"bulk_size_in_kb"`
	BulkSizeInMB  int    `config:"bulk_size_in_mb"`
	Elasticsearch string `config:"elasticsearch"`
	IndexName     string `config:"index_name"`
	TypeName      string `config:"type_name"`

	OutputQueue struct {
		Name   string                 `config:"name"`
		Labels map[string]interface{} `config:"label" json:"label,omitempty"`
	} `config:"output_queue"`
}

func init() {
	pipeline.RegisterProcessorPlugin("merge_to_bulk", New)
}

func New(c *config.Config) (pipeline.Processor, error) {
	cfg := Config{
		MessageField: "messages",
		BulkSizeInMB: 10,
	}

	if err := c.Unpack(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unpack the configuration of index_diff processor: %s", err)
	}

	if cfg.OutputQueue.Name == "" {
		panic(errors.New("name of output_queue can't be nil"))
	}

	if cfg.IndexName == "" {
		panic(errors.New("index name can't be nil"))
	}

	diff := &IndexingMergeToBulkProcessor{
		config: cfg,
	}

	labels:=util.MapStr{}
	labels["type"] = "merge_to_bulk"

	if cfg.IndexName != "" {
		labels["_index"] = cfg.IndexName
	} else {
		panic("index name is empty")
	}

	if cfg.TypeName == "" {
		cfg.TypeName = common.GetClusterDocType(cfg.Elasticsearch)
	}

	if cfg.TypeName != "" {
		labels["_type"] = cfg.TypeName
	}

	for k, v := range cfg.OutputQueue.Labels {
		labels[k] = v
	}

	if cfg.Elasticsearch != "" {
		labels["elasticsearch"] = cfg.Elasticsearch
	}

	diff.metadata = elastic.GetMetadata(cfg.Elasticsearch)
	if diff.metadata == nil {
		panic(errors.Errorf("cluster metadata [%v] not ready", cfg.Elasticsearch))
	}

	diff.bulkSizeInByte = 1048576 * cfg.BulkSizeInMB
	if cfg.BulkSizeInKB > 0 {
		diff.bulkSizeInByte = 1024 * cfg.BulkSizeInKB
	}

	queueConfig := queue.AdvancedGetOrInitConfig("",cfg.OutputQueue.Name,labels)
	queueConfig.ReplaceLabels(labels)
	diff.outputQueueConfig = queueConfig

	producer,err:=queue.AcquireProducer(queueConfig)
	if err!=nil{
		panic(err)
	}

	diff.producer=producer

	return diff, nil

}

// 合并批量处理的操作，这里只用来合并请求和构造 bulk 请求。
func (processor *IndexingMergeToBulkProcessor) Process(ctx *pipeline.Context) error {

	//get message from queue
	obj := ctx.Get(processor.config.MessageField)
	if obj != nil {
		messages := obj.([]queue.Message)
		if global.Env().IsDebug{
			log.Tracef("get %v messages from context", len(messages))
		}

		if len(messages) == 0 {
			return nil
		}
		//parse template
		mainBuf := bytebufferpool.Get("index_merge_main")
		defer bytebufferpool.Put("index_merge_main", mainBuf)

		lastOffset := len(messages) - 1
		for i, message := range messages {
			if processor.config.TypeName != "" {
				mainBuf.WriteString(fmt.Sprintf("{ \"index\" : { \"_index\" : \"%s\", \"_type\" : \"%s\" } }\n", processor.config.IndexName, processor.config.TypeName))
			} else {
				mainBuf.WriteString(fmt.Sprintf("{ \"index\" : { \"_index\" : \"%s\" } }\n", processor.config.IndexName))
			}

			pop := message.Data
			util.WalkBytesAndReplace(pop, util.NEWLINE, util.SPACE)

			mainBuf.Write(pop)
			mainBuf.WriteString("\n")

			//clean buffer
			if mainBuf.Len() > (processor.bulkSizeInByte) || i == lastOffset {

				data:=mainBuf.Bytes()
				//push to output queue
				r:=queue.ProduceRequest{Topic: processor.outputQueueConfig.ID, Data: data}
				res:=[]queue.ProduceRequest{r}
				_,err := processor.producer.Produce(&res)
				if err != nil {
					panic(errors.Errorf("failed to push message to output queue: %v, %s, offset:%v, size:%v, err:%v", processor.outputQueueConfig.Name,processor.outputQueueConfig.ID,message.Offset.String(),len(data),err))
				}

				//cleanup
				mainBuf.Reset()
				if global.Env().IsDebug {
					log.Trace("clean buffer")
				}
			}
		}
	}
	return nil
}
