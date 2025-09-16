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

/*
Copyright 2016 Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package indexing_merge

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"github.com/savsgio/gotils/bytes"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/stats"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/bytebufferpool"
	"infini.sh/framework/modules/elastic/common"
)

type IndexingMergeProcessor struct {
	initLocker        sync.RWMutex
	config            Config
	outputQueueConfig *queue.QueueConfig
}

// 处理纯 json 格式的消息索引
func (processor *IndexingMergeProcessor) Name() string {
	return "indexing_merge"
}

type Config struct {
	NumOfWorkers         int `config:"worker_size"`
	IdleTimeoutInSeconds int `config:"idle_timeout_in_seconds"`
	BulkSizeInKB         int `config:"bulk_size_in_kb"`
	BulkSizeInMB         int `config:"bulk_size_in_mb"`

	IndexName string `config:"index_name"`
	TypeName  string `config:"type_name"`
	WriteOpType string `config:"write_op_type"` //create, index, update

	KeyField string `config:"key_field"` //the field name used as document's primary key aka `_id

	Elasticsearch string `config:"elasticsearch"`

	InputQueue string `config:"input_queue"`

	OutputQueue struct {
		Name   string                 `config:"name"`
		Labels map[string]interface{} `config:"label" json:"label,omitempty"`
	} `config:"output_queue"`

	FailureQueue string `config:"failure_queue"`
	InvalidQueue string `config:"invalid_queue"`
}

func init() {
	pipeline.RegisterProcessorPlugin("indexing_merge", New)
}

func New(c *config.Config) (pipeline.Processor, error) {
	cfg := Config{
		BulkSizeInMB:         10,
		NumOfWorkers:         1,
		IdleTimeoutInSeconds: 5,
	}

	if err := c.Unpack(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unpack the configuration of index_diff processor: %s", err)
	}

	if cfg.InputQueue == "" {
		panic(errors.New("input_queue can't be nil"))
	}
	if cfg.OutputQueue.Name == "" {
		panic(errors.New("name of output_queue can't be nil"))
	}

	if cfg.IndexName == "" {
		panic(errors.New("index name can't be nil"))
	}

	if cfg.FailureQueue == "" {
		cfg.FailureQueue = cfg.InputQueue + "_failure"
	}

	if cfg.InvalidQueue == "" {
		cfg.InvalidQueue = cfg.InputQueue + "_invalid"
	}

	diff := &IndexingMergeProcessor{
		config: cfg,
	}

	queueConfig := queue.GetOrInitConfig(cfg.OutputQueue.Name)
	labels := util.MapStr{}
	labels["type"] = "indexing_merge"

	if cfg.IndexName != "" {
		labels["_index"] = cfg.IndexName
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

	queueConfig.ReplaceLabels(labels)
	//update queue config
	queue.RegisterConfig(queueConfig)

	diff.outputQueueConfig = queueConfig

	return diff, nil

}

// 合并批量处理的操作，这里只用来合并请求和构造 bulk 请求。
// TODO 重启子进程，当子进程挂了之后
func (processor *IndexingMergeProcessor) Process(ctx *pipeline.Context) error {
	defer func() {
		if !global.Env().IsDebug {
			if r := recover(); r != nil {
				var v string
				switch r.(type) {
				case error:
					v = r.(error).Error()
				case runtime.Error:
					v = r.(runtime.Error).Error()
				case string:
					v = r.(string)
				}
				log.Error("error in json indexing,", v)
			}
		}
	}()

	bulkSizeInByte := 1048576 * processor.config.BulkSizeInMB
	if processor.config.BulkSizeInKB > 0 {
		bulkSizeInByte = 1024 * processor.config.BulkSizeInKB
	}

	wg := sync.WaitGroup{}
	totalSize := 0
	for i := 0; i < processor.config.NumOfWorkers; i++ {
		wg.Add(1)
		go processor.NewBulkWorker(ctx, &totalSize, bulkSizeInByte, &wg)
	}

	wg.Wait()

	return nil
}

func (processor *IndexingMergeProcessor) NewBulkWorker(ctx *pipeline.Context, count *int, bulkSizeInByte int, wg *sync.WaitGroup) {

	defer func() {
		if !global.Env().IsDebug {
			if r := recover(); r != nil {
				var v string
				switch r.(type) {
				case error:
					v = r.(error).Error()
				case runtime.Error:
					v = r.(runtime.Error).Error()
				case string:
					v = r.(string)
				}
				log.Error("error in json indexing worker,", v)
				ctx.RecordError(fmt.Errorf("indexing merge panic: %v", r))
			}
		}
		wg.Done()
	}()

	log.Trace("start bulk worker")

	mainBuf := bytebufferpool.Get("index_merge_main")
	defer bytebufferpool.Put("index_merge_main", mainBuf)
	docBuf := bytebufferpool.Get("index_merge_docs")
	defer bytebufferpool.Put("index_merge_docs", docBuf)

	idleDuration := time.Duration(processor.config.IdleTimeoutInSeconds) * time.Second

	metadata := elastic.GetMetadata(processor.config.Elasticsearch)
	if metadata == nil {
		panic(errors.Errorf("cluster metadata [%v] not ready", processor.config.Elasticsearch))
	}

	if processor.config.TypeName == "" {
		processor.config.TypeName = common.GetClusterDocType(processor.config.Elasticsearch)
	}
	var lastCommit time.Time = time.Now()

READ_DOCS:
	for {

		if ctx.IsCanceled() {
			goto CLEAN_BUFFER
		}

		pop, _, err := queue.PopTimeout(queue.GetOrInitConfig(processor.config.InputQueue), idleDuration)
		if err != nil {
			log.Error(err)
			panic(err)
		}

		if len(pop) > 0 {

			if processor.config.IndexName == "" {
				panic("index name is empty")
			}

			var id_part = ""
			if processor.config.KeyField != "" {
				source := util.MapStr{}
				err := util.FromJSONBytes(pop, &source)
				if err != nil {
					panic(err)
				}

				v, err := source.GetValue(processor.config.KeyField)
				if err != nil {
					panic(err)
				}
				id_part = fmt.Sprintf(", \"_id\":\"%v\"", v)
			}
			// default to index
			writeOpType := "index"
			if processor.config.WriteOpType != "" {
				writeOpType = processor.config.WriteOpType
			}

			if processor.config.TypeName != "" {
				docBuf.WriteString(fmt.Sprintf("{ \"%s\" : { \"_index\" : \"%s\", \"_type\" : \"%s\" %v } }\n", writeOpType, processor.config.IndexName, processor.config.TypeName, id_part))
			} else {
				docBuf.WriteString(fmt.Sprintf("{ \"%s\" : { \"_index\" : \"%s\"  %v } }\n", writeOpType, processor.config.IndexName, id_part))
			}

			util.WalkBytesAndReplace(pop, util.NEWLINE, util.SPACE)

			docBuf.Write(pop)
			docBuf.WriteString("\n")

			mainBuf.Write(bytes.Copy(docBuf.Bytes()))

			docBuf.Reset()
			(*count)++
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

	if docBuf.Len() > 0 {
		mainBuf.Write(docBuf.Bytes())
	}

	//merge into bulk services
	if mainBuf.Len() > 0 {
		mainBuf.WriteByte('\n')
		//push to output queue
		err := queue.Push(processor.outputQueueConfig, mainBuf.Bytes())

		//result, err := client.Bulk(mainBuf.Bytes())
		if err != nil {
			stats.Increment("json_indexing", "error")
			queue.Push(queue.GetOrInitConfig(processor.config.FailureQueue), mainBuf.Bytes())
		}

		mainBuf.Reset()
		log.Trace("clean buffer")
	}

	if ctx.IsCanceled() {
		return
	}
	goto READ_DOCS
}
