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

package queue

import (
	"fmt"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/security"
	queue "infini.sh/framework/modules/queue/disk_queue"
	"net/http"
	"time"

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
	// queue overview/browsing endpoints consumed via the reverse channel;
	// served on the web port behind login + RBAC
	api.HandleUIMethod(api.GET, "/queue/stats", module.QueueStatsAction, api.RequireLogin(), api.RequirePermission(security.PermissionSystemQueueRead))
	api.HandleUIMethod(api.GET, "/queue/:id/stats", module.SingleQueueStatsAction, api.RequireLogin(), api.RequirePermission(security.PermissionSystemQueueRead))
	api.HandleUIMethod(api.GET, "/queue/:id/_scroll", module.QueueExplore, api.RequireLogin(), api.RequirePermission(security.PermissionSystemQueueRead))

	//purge: drop all messages and consumed segments, keep the queue registration
	api.HandleUIMethod(api.POST, "/queue/:id/_empty", module.QueueEmptyAction, api.RequireLogin(), api.RequirePermission(security.PermissionSystemQueueUpdate))

	api.HandleUIMethod(api.DELETE, "/queue/:id", module.DeleteQueue, api.RequireLogin(), api.RequirePermission(security.PermissionSystemQueueDelete))
	api.HandleUIMethod(api.DELETE, "/queue/_search", module.DeleteQueuesByQuery, api.RequireLogin(), api.RequirePermission(security.PermissionSystemQueueDelete))

	//create consumer
	//api.HandleAPIMethod(api.POST,"/queue/:id/consumer/:consumer_id", module.QueueResetConsumerOffset)

	//reset consumer offset
	api.HandleUIMethod(api.PUT, "/queue/:id/consumer/:consumer_id/offset", module.QueueResetConsumerOffset, api.RequireLogin(), api.RequirePermission(security.PermissionSystemQueueUpdate))
	//get consumer offset
	api.HandleUIMethod(api.GET, "/queue/:id/consumer/:consumer_id/offset", module.QueueGetConsumerOffset, api.RequireLogin(), api.RequirePermission(security.PermissionSystemQueueRead))

	// delete consumer and it's offset
	api.HandleUIMethod(api.DELETE, "/queue/:id/consumer/:consumer_id", module.QueueDeleteConsumerByID, api.RequireLogin(), api.RequirePermission(security.PermissionSystemQueueDelete))
	// delete all consumers of queues specified by query
	api.HandleUIMethod(api.DELETE, "/queue/consumer/_search", module.DeleteConsumersByQuery, api.RequireLogin(), api.RequirePermission(security.PermissionSystemQueueDelete))
}

func (module *API) SingleQueueStatsAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {

	metadata := module.Get(req, "metadata", "true")
	consumer := module.Get(req, "consumers", "true")
	useKey := module.Get(req, "use_key", "false")

	data := util.MapStr{}
	err := module.getQueueStatsSafe("", ps.MustGetParameter("id"), metadata, consumer, useKey, data)
	if err != nil {
		data["error"] = err.Error()
		module.WriteJSON(w, data, http.StatusInternalServerError)
		return
	}
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
		_ = log.Error("failed to parse queue selector: ", err)
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
	id := ps.MustGetParameter("id")
	module.deleteQueueByID(id)
	module.WriteAckOKJSON(w)
}

// QueueEmptyAction handles POST /queue/:id/_empty: drop all buffered
// messages and consumed segment files (releases disk space) while keeping
// the queue registration and its consumers; consumers whose offsets fall
// out of range auto-reset to the fresh head.
func (module *API) QueueEmptyAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	queueConfig, ok := queue1.SmartGetConfig(id)
	if !ok || queueConfig == nil {
		module.WriteError(w, fmt.Sprintf("queue [%v] not found", id), http.StatusNotFound)
		return
	}
	if err := queue1.EmptyQueue(queueConfig); err != nil {
		_ = log.Errorf("failed to empty queue [%v]: %v", id, err)
		module.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	common.PersistQueueMetadata()
	module.WriteAckOKJSON(w)
}

func (module *API) deleteQueueByID(id string) {
	queueConfig, ok := queue1.SmartGetConfig(id)
	if !ok || queueConfig == nil {
		_ = log.Errorf("invalid queue id [%v]", id)
		return
	}
	ok = queue1.RemoveConfig(queueConfig)
	if ok {
		ok, err := queue1.RemoveAllConsumers(queueConfig)
		if ok && err != nil {
			_ = log.Errorf("failed to remove consumers for queue [%v]", id)
			return
		}
		err = queue1.Destroy(queueConfig)
		if err != nil {
			_ = log.Errorf("failed to destroy queue [%v]", id)
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
			err := module.getQueueStatsSafe(t, q, metadata, consumer, useKey, data)
			if err != nil {
				// A single queue failing its stats (e.g. offset/depth panics
				// when a kafka broker is unreachable) must not take down the
				// whole stats endpoint — mark that queue with an error entry,
				// return the rest.
				_ = log.Errorf("queue [%v] stats failed, skipped: %v", q, err)
				data[q] = util.MapStr{"error": err.Error()}
				continue
			}
		}
		log.Tracef("queue [%v] stats: %v", t, data)
		datas[t] = data
	}
	module.WriteJSON(w, util.MapStr{
		"queue": datas,
	}, 200)
}

// getQueueStatsSafe wraps getQueueStats with a recover: queue handlers may
// panic on remote stats failures (e.g. kafka Depth/LatestOffset/GetOffset on
// an unreachable broker), and one broken queue must not take down the whole
// stats endpoint. The panic is converted into a per-queue error instead.
func (module *API) getQueueStatsSafe(t, q string, metadata string, consumer string, useKey string, data util.MapStr) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.Errorf("queue [%v] stats failed: %v", q, r)
		}
	}()
	return module.getQueueStats(t, q, metadata, consumer, useKey, data)
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

				t1 := v.GetLastActiveTime()
				if t1 != nil {
					m["last_active"] = t1.Format(time.RFC3339)
				}

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

	queueID := ps.MustGetParameter("id")
	offsetStr := module.GetParameterOrDefault(req, "offset", "0,0")
	size := module.GetIntOrDefault(req, "size", 5)
	// per-message response cap: dead-letter style queues can hold multi-MB
	// bulk payloads, and unbounded sampling blows past reverse-channel and
	// browser limits. 0 disables truncation.
	maxMsgBytes := module.GetIntOrDefault(req, "max_msg_bytes", 256*1024)

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
					content := string(v.Data)
					if maxMsgBytes > 0 && len(content) > maxMsgBytes {
						content = content[:maxMsgBytes] + fmt.Sprintf("\n...[truncated %v of %v bytes]", len(v.Data)-maxMsgBytes, len(v.Data))
						msg["truncated"] = true
					}
					msg["message"] = content
					msg["offset"] = v.Offset.String()
					msg["size"] = v.Size
					msgs = append(msgs, msg)
				}
				result["messages"] = msgs
			} else {
				if maxMsgBytes > 0 {
					for i := range messages {
						if len(messages[i].Data) > maxMsgBytes {
							messages[i].Data = messages[i].Data[:maxMsgBytes]
						}
					}
				}
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
			consumerAPI, err := queue1.AcquireConsumer(qConfig, consumer, "api")
			if consumerAPI != nil {
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
				log.Errorf("can't acquire consumer [%v] for [%v]", consumer.Key(), qConfig.Name)
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
	queueID := ps.MustGetParameter("id")
	consumerID := ps.MustGetParameter("consumer_id")

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
	queueID := ps.MustGetParameter("id")
	consumerID := ps.MustGetParameter("consumer_id")

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
		_ = log.Error("failed to parse queue selector: ", err)
		return
	}
	if obj.Selector == nil {
		module.WriteError(w, "no selector specified", http.StatusBadRequest)
		return
	}

	queues := queue1.GetConfigBySelector(obj.Selector)
	for _, q := range queues {
		consumers, ok := queue1.GetConsumerConfigsByQueueID(q.ID)
		if !ok {
			continue
		}
		for _, consumer := range consumers {
			err := module.deleteQueueConsumer(q, consumer)
			if err != nil {
				log.Warnf("failed to delete consumers of queue [%s], err: %v", q.Name, err)
			}
		}
	}
	module.WriteAckOKJSON(w)
}

func (module *API) QueueResetConsumerOffset(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	queueID := ps.MustGetParameter("id")
	consumerID := ps.MustGetParameter("consumer_id")

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
