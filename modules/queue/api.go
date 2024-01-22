/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package queue

import (
	"fmt"
	"infini.sh/framework/core/global"
	queue "infini.sh/framework/modules/queue/disk_queue"
	"net/http"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/errors"
	queue1 "infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/queue/common"
)

type API struct {
	api.Handler
}

func init() {
	module := API{}
	api.HandleAPIMethod(api.GET, "/queue/stats", module.QueueStatsAction)
	api.HandleAPIMethod(api.GET, "/queue/:id/stats", module.SingleQueueStatsAction)
	api.HandleAPIMethod(api.GET, "/queue/:id/_scroll", module.QueueExplore)

	api.HandleAPIMethod(api.DELETE, "/queue/:id", module.DeleteQueue)
	api.HandleAPIMethod(api.DELETE, "/queue/_search", module.DeleteQueuesByQuery)

	//create consumer
	//api.HandleAPIMethod(api.POST,"/queue/:id/consumer/:consumer_id", module.QueueResetConsumerOffset)

	//reset consumer offset
	api.HandleAPIMethod(api.PUT, "/queue/:id/consumer/:consumer_id/offset", module.QueueResetConsumerOffset)
	//get consumer offset
	api.HandleAPIMethod(api.GET, "/queue/:id/consumer/:consumer_id/offset", module.QueueGetConsumerOffset)

	// delete consumer and it's offset
	api.HandleAPIMethod(api.DELETE, "/queue/:id/consumer/:consumer_id", module.QueueDeleteConsumerByID)
	// delete all consumers of queues specified by query
	api.HandleAPIMethod(api.DELETE, "/queue/consumer/_search", module.DeleteConsumersByQuery)
}

func (module *API) SingleQueueStatsAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {

	metadata := module.Get(req, "metadata", "true")
	consumer := module.Get(req, "consumers", "true")
	useKey := module.Get(req, "use_key", "false")

	data := util.MapStr{}
	module.getQueueStats("", ps.ByName("id"), metadata, consumer, useKey, data)
	module.WriteJSON(w, data, 200)
}

type DeleteQueuesByQueryRequest struct {
	Selector *queue1.QueueSelector `json:"selector"`
}

func (module *API) DeleteQueuesByQuery(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var obj = DeleteQueuesByQueryRequest{}
	err := module.DecodeJSON(req, &obj)
	if err != nil {
		module.WriteError(w, err.Error(), http.StatusBadRequest)
		log.Error("failed to parse queue selector: ", err)
		return
	}

	if obj.Selector == nil {
		module.WriteError(w, "no selector specified", http.StatusBadRequest)
		return
	}

	queues := queue1.GetConfigBySelector(obj.Selector)
	for _, queue := range queues {
		module.deleteQueueByID(queue.ID)
	}
	module.WriteAckOKJSON(w)
}

func (module *API) DeleteQueue(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	module.deleteQueueByID(id)
	module.WriteAckOKJSON(w)
}

func (module *API) deleteQueueByID(id string) {
	queueConfig, ok := queue1.SmartGetConfig(id)
	if !ok || queueConfig == nil {
		log.Errorf("invalid queue id [%v]", id)
		return
	}
	ok = queue1.RemoveConfig(queueConfig)
	if ok {
		ok, err := queue1.RemoveAllConsumers(queueConfig)
		if ok && err != nil {
			log.Errorf("failed to remove consumers for queue [%v]", id)
			return
		}
		err = queue1.Destroy(queueConfig)
		if err != nil {
			log.Errorf("failed to destroy queue [%v]", id)
			return
		}
	}
	common.PersistQueueMetadata()
}

func (module *API) QueueStatsAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	metadata := module.Get(req, "metadata", "true")
	consumer := module.Get(req, "consumers", "true")
	useKey := module.Get(req, "use_key", "false")

	datas := map[string]util.MapStr{}
	queues := queue1.GetQueues()
	for t, qs := range queues {
		data := util.MapStr{}
		for _, q := range qs {
			err := module.getQueueStats(t, q, metadata, consumer, useKey, data)
			if err != nil {
				panic(err)
			}
		}
		log.Tracef("queue [%v] stats: %v", t, data)
		datas[t] = data
	}
	module.WriteJSON(w, util.MapStr{
		"queue": datas,
	}, 200)
}

func (module *API) getQueueStats(t, q string, metadata string, consumer string, useKey string, data util.MapStr) error {

	var cfg *queue1.QueueConfig
	if t == "kafka" {
		var ok bool
		cfg, ok = queue1.GetConfigByUUID(q)
		//new config loaded from kafka
		//TODO load queue metadata from database
		if !ok {
			cfg = &queue1.QueueConfig{}
			cfg.ID = q
			cfg.Name = q
			cfg.Type = "kafka"
			queue1.RegisterConfig(cfg)
		}
	} else {
		var ok bool
		cfg, ok = queue1.SmartGetConfig(q)
		if !ok {
			return errors.Errorf("queue [%v] was not found", q)
		}
	}

	qd := util.MapStr{}
	if cfg.Type == "disk" || cfg.Type == "" {
		storeSize := queue1.GetStorageSize(q)
		qd["storage"] = util.MapStr{
			"local_usage":          util.ByteSize(storeSize),
			"local_usage_in_bytes": storeSize,
		}
	}

	if metadata != "false" {
		qd["metadata"] = cfg
	}

	var hasConsumers = false
	if consumer != "false" {
		cfg1, ok := queue1.GetConsumerConfigsByQueueID(q)
		if ok {
			maps := []util.MapStr{}
			for _, v := range cfg1 {
				m := util.MapStr{}
				m["source"] = v.Source
				m["id"] = v.ID
				m["group"] = v.Group
				m["name"] = v.Name
				offset, err := queue1.GetOffset(cfg, v)
				if err == nil {
					m["offset"] = offset.EncodeToString()
				}
				maps = append(maps, m)
			}
			if len(maps) > 0 {
				qd["consumers"] = maps
				hasConsumers = true
			}
		}
	}

	if !hasConsumers {
		qd["depth"] = queue1.Depth(cfg)
	} else {
		qd["messages"] = queue1.Depth(cfg)
		eoffset := queue1.GetEarlierOffsetStrByQueueID(q)
		qd["earliest_consumer_offset"] = eoffset.String()
		offset := queue1.LatestOffset(cfg)
		qd["offset"] = offset.String()
		qd["synchronization"] = util.MapStr{
			"latest_segment": queue.GetLastS3UploadFileNum(q),
		}
	}

	if useKey == "false" {
		data[q] = qd
	} else {
		data[cfg.Name] = qd
	}
	return nil
}

func (module *API) QueueExplore(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {

	queueID := ps.ByName("id")
	offsetStr := module.GetParameterOrDefault(req, "offset", "0,0")
	size := module.GetIntOrDefault(req, "size", 5)

	group := module.GetParameterOrDefault(req, "group", "api")
	name := module.GetParameterOrDefault(req, "name", "api")

	dataIsString := true

	var ctx *queue1.Context = &queue1.Context{
		InitOffset: queue1.DecodeFromString(offsetStr),
	}

	log.Debugf("queue explore [%v] offset [%v] size [%v]", queueID, offsetStr, size)

	var err error
	var timeout bool
	messages := []queue1.Message{}
	defer func() {
		result := util.MapStr{}
		status := 200
		if err != nil {
			result["error"] = err.Error()
			status = 500
		}
		if len(messages) > 0 {
			if dataIsString {
				msgs := []util.MapStr{}
				for _, v := range messages {
					msg := util.MapStr{}
					msg["message"] = string(v.Data)
					msg["offset"] = v.Offset.String()
					msg["size"] = v.Size
					msgs = append(msgs, msg)
				}
				result["messages"] = msgs
			} else {
				result["messages"] = messages
			}

			if ctx != nil {
				result["context"] = ctx
			}
			result["timeout"] = timeout
			if err != nil {
				result["error"] = err.Error()
			}
		}
		module.WriteJSON(w, result, status)
	}()

	_, ok := queue1.SmartGetConfig(queueID)
	if ok {
		consumer := queue1.NewConsumerConfig(queueID, group, name)
		consumer.FetchMaxMessages = size
		consumer.FetchMaxWaitMs = 500
		consumer.EOFMaxRetryTimes = 10
		consumer.FetchMaxBytes = 1024 * 500
		qConfig, ok := queue1.SmartGetConfig(queueID)
		if ok {
			consumerAPI, err := queue1.AcquireConsumer(qConfig, consumer)
			defer queue1.ReleaseConsumer(qConfig, consumer, consumerAPI)
			err = consumerAPI.ResetOffset(queue1.ConvertOffset(offsetStr)) //TODO fix offset reset
			if err != nil {
				return
			}
			messages, timeout, err = consumerAPI.FetchMessages(ctx, size)
			if global.Env().IsDebug {
				log.Trace(len(messages), ",", timeout, ",", err)
			}
			if err != nil {
				return
			}
		} else {
			err = errors.New(fmt.Sprintf("queue [%v] not exists", queueID))
		}
	} else {
		err = errors.New(fmt.Sprintf("queue [%v] not exists", queueID))
		return
	}

}

func (module *API) QueueGetConsumerOffset(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	queueID := ps.ByName("id")
	consumerID := ps.ByName("consumer_id")
	cfg, ok := queue1.SmartGetConfig(queueID)
	cfg1, ok1 := queue1.GetConsumerConfigID(queueID, consumerID)
	obj := util.MapStr{}
	var status = 404
	if ok && ok1 {
		offset, err := queue1.GetOffset(cfg, cfg1)
		if err != nil {
			obj["error"] = err.Error()
		} else {
			obj["found"] = true
			obj["result"] = offset
			status = 200
		}
	} else {
		obj["found"] = false
	}
	module.WriteJSON(w, obj, status)
}

func (module *API) QueueDeleteConsumerByID(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	queueID := ps.ByName("id")
	consumerID := ps.ByName("consumer_id")

	queueConfig, ok := queue1.SmartGetConfig(queueID)
	consumerConfig, ok1 := queue1.GetConsumerConfigID(queueID, consumerID)

	if !ok || !ok1 {
		module.WriteJSON(w, util.MapStr{
			"result": "not_found",
		}, 404)
		return
	}

	err := module.deleteQueueConsumer(queueConfig, consumerConfig)
	if err != nil {
		module.WriteJSON(w, util.MapStr{
			"result": "error",
			"error":  err.Error(),
		}, 500)
		return
	}

	module.WriteJSON(w, util.MapStr{
		"result": "ok",
	}, 200)
}

func (module *API) deleteQueueConsumer(queueConfig *queue1.QueueConfig, consumerConfig *queue1.ConsumerConfig) error {
	_, err := queue1.RemoveConsumer(queueConfig.ID, consumerConfig.Key())
	if err != nil {
		return fmt.Errorf("failed to delete consumer config, err: %v", err)
	}

	err = queue1.DeleteOffset(queueConfig, consumerConfig)
	if err != nil {
		return fmt.Errorf("failed to delete offset, err: %v", err)
	}
	return nil
}

type DeleteConsumersByQueryRequest struct {
	Selector *queue1.QueueSelector `json:"selector"`
}

func (module *API) DeleteConsumersByQuery(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var obj = DeleteConsumersByQueryRequest{}
	err := module.DecodeJSON(req, &obj)
	if err != nil {
		module.WriteError(w, err.Error(), http.StatusBadRequest)
		log.Error("failed to parse queue selector: ", err)
		return
	}
	if obj.Selector == nil {
		module.WriteError(w, "no selector specified", http.StatusBadRequest)
		return
	}

	queues := queue1.GetConfigBySelector(obj.Selector)
	for _, queue := range queues {
		consumers, ok := queue1.GetConsumerConfigsByQueueID(queue.ID)
		if !ok {
			continue
		}
		for _, consumer := range consumers {
			err := module.deleteQueueConsumer(queue, consumer)
			if err != nil {
				log.Warnf("failed to delete consumers of queue [%s], err: %v", queue.Name, err)
			}
		}
	}
	module.WriteAckOKJSON(w)
}

func (module *API) QueueResetConsumerOffset(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	queueID := ps.ByName("id")
	consumerID := ps.ByName("consumer_id")

	if queueID==""||consumerID==""{
		panic(errors.New("invalid queue or consumer"))
	}

	offsetStr := module.GetParameterOrDefault(req, "offset", "0,0")
	cfg, ok := queue1.SmartGetConfig(queueID)
	cfg1, ok1 := queue1.GetConsumerConfigID(queueID, consumerID)
	var ack = false
	var status = 404
	if ok && ok1 {
		oldOffset, err := queue1.GetOffset(cfg, cfg1)
		if err != nil {
			panic(err)
		}

		newOffset := queue1.DecodeFromString(offsetStr)
		newOffset.Version = oldOffset.Version + 1
		ok, err := queue1.CommitOffset(cfg, cfg1, newOffset)
		ack = ok
		status = 200
		if err != nil {
			module.WriteError(w, err.Error(), http.StatusBadRequest)
		}
	}

	module.WriteAckJSON(w, ack, status, nil)
}
