/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package pipeline

import (
	"fmt"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/rotate"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/bytebufferpool"
	elastic2 "infini.sh/gateway/proxy/filters/elastic"
	"runtime"
	log "github.com/cihub/seelog"

	"sync"
	"time"
)

type MetadataProcessor struct {
	bufferPool     *bytebufferpool.Pool
	config         *Config
	runningConfigs map[string]*queue.Config
	bulkSizeInByte int
	wg             sync.WaitGroup
	inFlightQueueConfigs sync.Map
	detectorRunning bool
	id string
}

type Config struct {
	NumOfWorkers         int    `config:"worker_size"`

	IdleTimeoutInSecond  int    `config:"idle_timeout_in_seconds"`
	MaxConnectionPerHost int    `config:"max_connection_per_node"`

	BulkSizeInKb         int    `config:"bulk_size_in_kb,omitempty"`
	BulkSizeInMb         int    `config:"bulk_size_in_mb,omitempty"`
	BulkMaxDocsCount     int    `config:"bulk_max_docs_count,omitempty"`

	Queues          map[string]interface{} `config:"queues,omitempty"`

	Consumer   queue.ConsumerConfig `config:"consumer"`

	MaxWorkers int      `config:"max_worker_size"`

	DetectActiveQueue bool     `config:"detect_active_queue"`
	DetectIntervalInMs   int         `config:"detect_interval"`

	ValidateRequest bool     `config:"valid_request"`
	SkipEmptyQueue bool     `config:"skip_empty_queue"`
	SkipOnMissingInfo bool  `config:"skip_info_missing"`

	RotateConfig rotate.RotateConfig          `config:"rotate"`
	BulkConfig   elastic2.BulkProcessorConfig `config:"bulk"`

	Elasticsearch     string    `config:"elasticsearch,omitempty"`

	WaitingAfter        []string `config:"waiting_after"`

}

func init()  {
	pipeline.RegisterProcessorPlugin("metadata", New)
}

func New(c *config.Config) (pipeline.Processor, error) {
	cfg := Config{
		NumOfWorkers:         1,
		MaxWorkers:           10,
		MaxConnectionPerHost: 1,
		IdleTimeoutInSecond:  5,
		BulkSizeInMb:         10,
		DetectIntervalInMs:   10000,
		Queues: map[string]interface{}{},

		Consumer: queue.ConsumerConfig{
			Group: "metadata-001",
			Name: "metadata-001",
			FetchMinBytes:   	1,
			FetchMaxMessages:   100,
			FetchMaxWaitMs:   10000,
		},

		DetectActiveQueue:    true,
		ValidateRequest:      false,
		SkipEmptyQueue:      true,
		SkipOnMissingInfo:   false,
		RotateConfig:         rotate.DefaultConfig,
		BulkConfig:           elastic2.DefaultBulkProcessorConfig,
	}

	if err := c.Unpack(&cfg); err != nil {
		log.Error(err)
		return nil, fmt.Errorf("failed to unpack the configuration of flow_runner processor: %s", err)
	}

	runner := MetadataProcessor{
		id:util.GetUUID(),
		config: &cfg,
		runningConfigs: map[string]*queue.Config{},
		inFlightQueueConfigs:sync.Map{},
	}

	runner.bulkSizeInByte= 1048576 * runner.config.BulkSizeInMb
	if runner.config.BulkSizeInKb > 0 {
		runner.bulkSizeInByte = 1024 * runner.config.BulkSizeInKb
	}

	estimatedBulkSizeInByte := runner.bulkSizeInByte + (runner.bulkSizeInByte / 3)
	runner.bufferPool = bytebufferpool.NewPool(uint64(estimatedBulkSizeInByte), uint64(runner.bulkSizeInByte*2))

	runner.wg = sync.WaitGroup{}

	return &runner, nil
}

func (processor *MetadataProcessor) Name() string {
	return "metadata"
}

func (processor *MetadataProcessor) Process(c *pipeline.Context) error {
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
				log.Error("error in metadata processor,", v)
			}
		}
		log.Trace("exit metadata processor")
	}()

	//handle updates
	if processor.config.DetectActiveQueue{
		log.Tracef("detector running [%v]",processor.detectorRunning)
		if !processor.detectorRunning{
			processor.detectorRunning=true
			processor.wg.Add(1)
			go func(c *pipeline.Context) {
				log.Tracef("init detector for active queue [%v] ",processor.id)
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
							log.Error("error in bulk indexing processor,", v)
						}
					}
					processor.detectorRunning=false
					log.Debug("exit detector for active queue")
					processor.wg.Done()
				}()

				for {
					if c.IsCanceled() {
						return
					}

					if global.Env().IsDebug{
						log.Tracef("inflight queues: %v",util.MapLength(&processor.inFlightQueueConfigs))
						processor.inFlightQueueConfigs.Range(func(key, value interface{}) bool {
							log.Tracef("inflight queue:%v",key)
							return true
						})
					}

					cfgs:=queue.GetQueuesFilterByLabel(processor.config.Queues)
					for _,v:=range cfgs{
						if c.IsCanceled() {
							return
						}
						//if have depth and not in in flight
						if queue.HasLag(v){
							_,ok:=processor.inFlightQueueConfigs.Load(v.Id)
							if !ok{
								log.Tracef("detecting new queue: %v",v.Name)
								processor.HandleQueueConfig(v,c)
							}
						}
					}
					if processor.config.DetectIntervalInMs>0{
						time.Sleep(time.Millisecond*time.Duration(processor.config.DetectIntervalInMs))
					}
				}
			}(c)
		}
	}else{
		cfgs:=queue.GetQueuesFilterByLabel(processor.config.Queues)
		log.Debugf("filter queue by:%v, num of queues:%v",processor.config.Queues,len(cfgs))
		for _,v:=range cfgs{
			log.Tracef("checking queue: %v",v)
			processor.HandleQueueConfig(v,c)
		}
	}

	processor.wg.Wait()

	return nil
}

func (processor *MetadataProcessor) HandleQueueConfig(v *queue.Config,c *pipeline.Context){

	if processor.config.SkipEmptyQueue{
		if !queue.HasLag(v){
			if global.Env().IsDebug{
				log.Tracef("skip empty queue:[%v]",v.Name)
			}
			return
		}
	}

	elasticsearch := processor.config.Elasticsearch
	if elasticsearch==""{
		log.Error("elasticsearch config was not found in metadata processor" )
		return
	}

	meta := elastic.GetMetadata(util.ToString(elasticsearch))
	if meta == nil {
		log.Debugf("metadata for [%v] is nil",elasticsearch)
		return
	}

	host := meta.GetActiveHost()
	log.Debugf("random choose node [%v] to consume queue [%v]",host,v.Id)
	processor.wg.Add(1)

	//go processor.NewBulkWorker("bulk_indexing_"+host,c, processor.bulkSizeInByte, v, host)
	go processor.HandleMessage(c, v)

}

func (processor *MetadataProcessor) HandleMessage(ctx *pipeline.Context, qConfig *queue.Config){
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
				log.Errorf("error in %s processor: %v", processor.Name(), v)
			}
		}
		processor.wg.Done()
		log.Tracef("exit %s processor", processor.Name())
	}()

	key:=qConfig.Id

	if processor.config.MaxWorkers>0&&util.MapLength(&processor.inFlightQueueConfigs)>processor.config.MaxWorkers{
		log.Debugf("reached max num of workers, skip init [%v]",qConfig.Name)
		return
	}

	var workerID=util.GetUUID()
	_,exists:= processor.inFlightQueueConfigs.Load(key)
	if exists{
		log.Errorf("queue [%v] has more then one consumer",qConfig.Id)
		return
	}

	processor.inFlightQueueConfigs.Store(key,workerID)
	log.Debugf("starting worker:[%v], queue:[%v]",workerID, qConfig.Name)
	var consumer=queue.GetOrInitConsumerConfig(qConfig.Id,processor.config.Consumer.Group,processor.config.Consumer.Name)
	initOffset,_:=queue.GetOffset(qConfig,consumer)
	offset:= initOffset
	defer func() {
		log.Debugf("worker:[%v] start consume queue:[%v] offset:%v",workerID,qConfig.Id,offset)
	}()

	for {
		if ctx.IsCanceled() {
			return
		}

		ctx1,messages,timeout,err:=queue.Consume(qConfig,consumer.Name,offset,processor.config.Consumer.FetchMaxMessages,time.Millisecond*time.Duration(processor.config.Consumer.FetchMaxWaitMs))

		if timeout{
			log.Tracef("timeout on queue:[%v]",qConfig.Name)
			ctx.Failed()
			return
		}

		if err != nil {
			log.Tracef("error on queue:[%v]",qConfig.Name)
			if err.Error()=="EOF" {
				if len(messages)>0{
					goto HANDLE_MESSAGE
				}
				return
			}
			panic(err)
		}

	HANDLE_MESSAGE:

		//update temp offset, not committed, continued reading
		offset=ctx1.NextOffset

		if len(messages) > 0 {
			for _, pop := range messages {
				ev := event.Event{}
				util.MustFromJSONBytes(pop.Data, &ev)
				switch ev.Metadata.Name {
				case "index_health_change":
					err = processor.HandleIndexHealthChange(&ev)
				case "index_state_change":
					err = processor.HandleIndexStateChange(&ev)
				}

			}
		}
		if err == nil {
			if offset!=""&& initOffset !=offset{
				ok,err:=queue.CommitOffset(qConfig,consumer,offset)
				if !ok||err!=nil{
					panic(err)
				}
			}
		}else{
			log.Error(err)
		}
	}
}
func (processor *MetadataProcessor) HandleIndexStateChange(ev *event.Event) error{
	typ := ev.Metadata.Labels["type"].(string)
	// save activity
	activityInfo := &event.Activity{
		ID: util.GetUUID(),
		Timestamp: ev.Timestamp,
		Metadata: event.ActivityMetadata{
			Category: ev.Metadata.Category,
			Group: "metadata",
			Name: "index_state_change",
			Type: typ,
			Labels: ev.Metadata.Labels,
		},
		Fields: ev.Fields,
	}
	if typ =="update" {
		if changelog, ok := activityInfo.Fields["changelog"]; ok {
			activityInfo.Changelog = changelog
			delete(activityInfo.Fields, "changelog")
		}
	}
	esClient := elastic.GetClient(processor.config.Elasticsearch)
	_, err := esClient.Index(orm.GetIndexName(activityInfo), "", activityInfo.ID, activityInfo)
	if err != nil {
		return err
	}
	// save index metadata
	var (
		indexConfig *elastic.IndexConfig
		clusterID string
		clusterName string
		indexName string
		health string
		ok bool
	)
	if clusterName, ok = ev.Metadata.Labels["cluster_name"].(string); !ok {
		return fmt.Errorf("empty cluster name")
	}
	if clusterID, ok = ev.Metadata.Labels["cluster_id"].(string); !ok {
		return fmt.Errorf("process cluster %s: empty cluster id", clusterName)
	}
	if indexName, ok = ev.Metadata.Labels["index_name"].(string); !ok {
		return fmt.Errorf("process cluster %s: empty index name", clusterName)
	}
	if typ != "delete" {
		if health, ok = ev.Metadata.Labels["health"].(string); !ok {
			return fmt.Errorf("process cluster %s: empty health", clusterName)
		}
	}
	indexID := fmt.Sprintf("%s:%s", clusterID, indexName)
	queryDsl := `{
	"size": 1,
  "query": {
    "bool": {
      "must": [
        {
          "term": {
            "metadata.index_id": {
              "value": "%s"
            }
          }
        }
      ],
      "must_not": [
        {
          "term": {
            "metadata.labels.index_status": {
              "value": "deleted"
            }
          }
        }
      ]
    }
  }
}`
	queryDsl = fmt.Sprintf(queryDsl, indexID)
	storeIndexName := orm.GetIndexName(elastic.IndexConfig{})
	searchRes, err := esClient.SearchWithRawQueryDSL(storeIndexName, []byte(queryDsl))
	if err != nil {
		return err
	}
	aliases, _ := ev.Fields.GetValue("index_state.aliases")
	state, _ := ev.Fields.GetValue("index_state.state")
	version, _ := ev.Fields.GetValue("index_state.version")
	indexUUID, _ :=  ev.Fields.GetValue("index_state.settings.index.uuid")
	indexConfig = &elastic.IndexConfig{
		ID:        util.GetUUID(),
		Timestamp: time.Now(),
		Metadata:  elastic.IndexMetadata{
			IndexID: fmt.Sprintf("%s:%s", clusterID, indexName),
			IndexName: indexName,
			ClusterName: clusterName,
			Aliases:  aliases,
			ClusterID: clusterID,
			Labels: util.MapStr{
				"version": version,
				"state": state,
				"index_uuid": indexUUID,
				"health_status": health,
			},
			Category: "elasticsearch",
		},
		Fields: ev.Fields,
	}
	switch typ {
	case "update":
		//todo skip version lower than old state
		if searchRes.GetTotal() == 0 {
			return fmt.Errorf("index id %s can not be found", indexID)
		}
		indexConfig.ID = searchRes.Hits.Hits[0].ID
		oldConfig := util.MapStr(searchRes.Hits.Hits[0].Source)
		if labels, err := oldConfig.GetValue("metadata.labels"); err == nil {
			if labelsM, ok := labels.(map[string]interface{}); ok {
				for k, v := range labelsM {
					if _, ok := indexConfig.Metadata.Labels[k]; !ok {
						indexConfig.Metadata.Labels[k] = v
					}
				}
			}
		}
		if tags, err := oldConfig.GetValue("metadata.tags"); err == nil {
			if vtags, ok := tags.([]interface{}); ok {
				indexConfig.Metadata.Tags = vtags
			}
		}
		if healthStatus, err := oldConfig.GetValue("metadata.labels.health_status"); err == nil {
			if v, ok := healthStatus.(string); ok && v != health {
				// health status change
				activityInfo = &event.Activity{
					ID: util.GetUUID(),
					Timestamp: ev.Timestamp,
					Metadata: event.ActivityMetadata{
						Category: ev.Metadata.Category,
						Group: "metadata",
						Name: "index_health_change",
						Type: "update",
						Labels: util.MapStr{
							"cluster_id": clusterID,
							"index_id":   indexID,
							"index_uuid": indexUUID,
							"index_name": indexName,
							"from":       v,
							"to":         health,
						},
					},
				}
				_, err = esClient.Index(orm.GetIndexName(activityInfo), "", activityInfo.ID, activityInfo)
				if err != nil {
					return err
				}
			}
		}
	case "create":
		if searchRes.GetTotal() > 0 {
			return fmt.Errorf("index id %s already exists", indexID)
		}
	case "delete":
		if searchRes.GetTotal() == 0 {
			return fmt.Errorf("index id %s can not be found", indexID)
		}
		_, err = util.MapStr(searchRes.Hits.Hits[0].Source).Put("metadata.labels.index_status", "deleted")
		if err != nil {
			return err
		}
		_, err = esClient.Index(storeIndexName, "",  searchRes.Hits.Hits[0].ID, searchRes.Hits.Hits[0].Source)
		return err
	}
	_, err = esClient.Index(storeIndexName, "",  indexConfig.ID, indexConfig)
	return err
}

func (processor *MetadataProcessor) HandleIndexHealthChange(ev *event.Event) error{
	// save activity
	activityInfo := &event.Activity{
		ID: util.GetUUID(),
		Timestamp: ev.Timestamp,
		Metadata: event.ActivityMetadata{
			Category: ev.Metadata.Category,
			Group: "metadata",
			Name: "index_health_change",
			Type: "update",
			Labels: ev.Metadata.Labels,
		},
	}
	esClient := elastic.GetClient(processor.config.Elasticsearch)
	_, err := esClient.Index(orm.GetIndexName(activityInfo), "", activityInfo.ID, activityInfo)
	if err != nil {
		return err
	}
	// update index health status
	queryDslTpl := `{
  "size": 1, 
  "query": {
    "bool": {
      "must": [
        {"term": {
          "metadata.index_id": {
            "value": "%s"
          }
        }},
		 {"term": {
          "metadata.category": {
            "value": "elasticsearch"
          }
        }}
      ],
		"must_not": [
        {"term": {
          "metadata.labels.index_status": {
            "value": "deleted"
          }
        }}
      ]
    }
  }
}`
	queryDsl := fmt.Sprintf(queryDslTpl, ev.Metadata.Labels["index_id"])
	indexName := orm.GetIndexName(elastic.IndexConfig{})
	searchRes, err := esClient.SearchWithRawQueryDSL( indexName, []byte(queryDsl))
	if err != nil {
		return err
	}
	if searchRes.GetTotal()  == 0 {
		return nil
	}
	source := util.MapStr(searchRes.Hits.Hits[0].Source)
	source.Put("metadata.labels.health_status", ev.Metadata.Labels["to"])
	_, err = esClient.Index(indexName, "", searchRes.Hits.Hits[0].ID, source)
	return err
}