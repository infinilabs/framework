/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/agent"
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/api/rbac"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/migration"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/proxy"
	task2 "infini.sh/framework/core/task"
	"infini.sh/framework/core/util"
	"net/http"
	"strconv"
	"time"
)

type APIHandler struct {
	api.Handler
}

func (h *APIHandler) createDataMigrationTask(w http.ResponseWriter, req *http.Request, ps httprouter.Params){
	clusterTaskConfig := &migration.ElasticDataConfig{}
	err := h.DecodeJSON(req, clusterTaskConfig)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	clusterTaskConfig.Creator = struct {
		Name string `json:"name"`
		Id   string `json:"id"`
	}{}
	claims, ok := req.Context().Value("user").(*rbac.UserClaims)
	if ok {
		clusterTaskConfig.Creator.Name = claims.Username
		clusterTaskConfig.Creator.Id = claims.ID
	}

	var totalDocs int64
	for _, index := range clusterTaskConfig.Indices {
		totalDocs += index.Source.Docs
	}

	t := task2.Task{
		ID: util.GetUUID(),
		Metadata: task2.Metadata{
			Type: "pipeline",
			Labels: util.MapStr{
				"pipeline_id": "cluster_migration",
				"source_cluster_id": clusterTaskConfig.Cluster.Source.Id,
				"target_cluster_id": clusterTaskConfig.Cluster.Target.Id,
				"source_total_docs": totalDocs,
			},
		},
		Cancellable: true,
		Runnable: false,
		Status: "init",
		Created: time.Now().UTC(),
		Updated: time.Now().UTC(),
		Parameters: map[string]interface{}{
			"pipeline": util.MapStr{
				"id":     "cluster_migration",
				"config": clusterTaskConfig,
			},
		},
	}
	err = orm.Create(&t)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}

	h.WriteJSON(w, util.MapStr{
		"_id":    t.ID,
		"result": "created",
	}, 200)

}

func (h *APIHandler) searchDataMigrationTask(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var (
		keyword = h.GetParameterOrDefault(req, "keyword", "")
		strSize      = h.GetParameterOrDefault(req, "size", "20")
		strFrom      = h.GetParameterOrDefault(req, "from", "0")
		mustQ       []interface{}
	)
	mustQ = append(mustQ, util.MapStr{
		"term": util.MapStr{
			"metadata.labels.pipeline_id": util.MapStr{
				"value": "cluster_migration",
			},
		},
	})

	if keyword != "" {
		mustQ = append(mustQ, util.MapStr{
			"query_string": util.MapStr{
				"default_field": "*",
				"query":         keyword,
			},
		})
	}
	size, _ := strconv.Atoi(strSize)
	if size <= 0 {
		size = 20
	}
	from, _ := strconv.Atoi(strFrom)
	if from < 0 {
		from = 0
	}

	queryDSL := util.MapStr{
		"sort": []util.MapStr{
			{
				"created": util.MapStr{
					"order": "desc",
				},
			},
		},
		"size": size,
		"from": from,
		"query": util.MapStr{
			"bool": util.MapStr{
				"must": mustQ,
			},
		},
	}

	q := orm.Query{}
	q.RawQuery = util.MustToJSONBytes(queryDSL)

	err, res := orm.Search(&task2.Task{}, &q)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	searchRes := &elastic.SearchResponse{}
	err = util.FromJSONBytes(res.Raw, searchRes)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, hit := range searchRes.Hits.Hits {
		config, err := util.MapStr(hit.Source).GetValue("parameters.pipeline.config")
		if err != nil {
			log.Error(err)
			continue
		}
		buf := util.MustToJSONBytes(config)
		dataConfig := migration.ElasticDataConfig{}
		err = util.FromJSONBytes(buf, &dataConfig)
		if err != nil {
			log.Error(err)
			continue
		}
		esClient := elastic.GetClient(dataConfig.Cluster.Target.Id)
		var targetTotalDocs int64
		for _, index := range dataConfig.Indices {
			count, _ := getIndexTaskCount(&index, esClient)
			targetTotalDocs += count
		}
		util.MapStr(hit.Source).Put("metadata.labels.target_total_docs", targetTotalDocs)
		sourceTotalDocs, _ := util.MapStr(hit.Source).GetValue("metadata.labels.source_total_docs")
		if sv, ok := sourceTotalDocs.(float64); ok{
			if int64(sv) == targetTotalDocs {
				hit.Source["status"] = "complete"
			}
		}

	}

	h.WriteJSON(w, searchRes, http.StatusOK)
}

func (h *APIHandler) getIndexPartitionInfo(w http.ResponseWriter, req *http.Request, ps httprouter.Params){
	var (
		index = ps.MustGetParameter("index")
		clusterID = ps.MustGetParameter("id")
	)
	client := elastic.GetClient(clusterID)
	pq := &elastic.PartitionQuery{
		IndexName: index,
	}
	err := h.DecodeJSON(req, pq)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	partitions, err := elastic.GetPartitions(pq, client)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.WriteJSON(w, partitions, http.StatusOK)
}

func (h *APIHandler) startDataMigration(w http.ResponseWriter, req *http.Request, ps httprouter.Params){
	taskID := ps.MustGetParameter("task_id")
	obj := task2.Task{}

	obj.ID = taskID
	exists, err := orm.Get(&obj)
	if !exists || err != nil {
		h.WriteError(w,  fmt.Sprintf("task [%s] not found", taskID), http.StatusInternalServerError)
		return
	}
	if obj.Status == "init" {
		//root task
		obj.Status = "ready"
	}else if obj.Status == "stopped" {
		if obj.Metadata.Labels["level"] == "partition" {
			obj.Status = "ready"
		}else{
			obj.Status = "running"
			//update sub task status
			query := util.MapStr{
				"bool": util.MapStr{
					"must": []util.MapStr{
						{
							"term": util.MapStr{
								"parent_id": util.MapStr{
									"value": taskID,
								},
							},
						},
						{
							"term": util.MapStr{
								"metadata.labels.pipeline_id": util.MapStr{
									"value": "index_migration",
								},
							},
						},
						{
							"terms": util.MapStr{
								"status": []string{"error", "stopped"},
							},
						},
					},
				},
			}
			queryDsl := util.MapStr{
				"query": query,
				"script": util.MapStr{
					"source": "ctx._source['status'] = 'ready'",
				},
			}

			err = orm.UpdateBy(task2.Task{}, util.MustToJSONBytes(queryDsl))
			if err != nil {
				log.Error(err)
				h.WriteError(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

	}else if obj.Status == "error" {
		obj.Status = "ready"
	}

	err = orm.Update(&obj)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.WriteJSON(w, util.MapStr{
		"success": true,
	}, 200)
}

func getNodeEndpoint(nodeID string) (string, error){
	indexName := ".infini_agent,.infini_gateway-instance"
	query := util.MapStr{
		"size": 1,
		"query": util.MapStr{
			"term": util.MapStr{
				"id": util.MapStr{
					"value": nodeID,
				},
			},
		},
	}
	q := orm.Query{
		IndexName: indexName,
		RawQuery: util.MustToJSONBytes(query),
	}
	err, result := orm.Search(nil, &q)
	if err != nil {
		return "", err
	}
	if len(result.Result) == 0 {
		return "", fmt.Errorf("node [%s] not found", nodeID)
	}
	if info, ok := result.Result[0].(map[string]interface{}); ok {
		if v, ok := info["endpoint"]; ok {
			if endpoint, ok := v.(string); ok {
				return endpoint, nil
			}
			return "", fmt.Errorf("got invalid endpoint value: %v", v)
		}
		ag := agent.Instance{}
		buf := util.MustToJSONBytes(info)
		err = util.FromJSONBytes(buf, &ag)
		if err != nil {
			return "", err
		}
		return ag.GetEndpoint(), nil
	}
	return "", fmt.Errorf("got unexpect node info: %s", util.MustToJSON(result.Result[0]))
}

func stopTask(nodeID, taskID string) error {
	endpoint, err := getNodeEndpoint(nodeID)
	if err != nil {
		return err
	}
	res, err := proxy.DoProxyRequest(&proxy.Request{
		Method: http.MethodPost,
		Endpoint: endpoint,
		Path: fmt.Sprintf("/pipeline/task/%s/_stop", taskID),
	})

	if err != nil {
		return fmt.Errorf("call stop task api error: %w", err)
	}
	resBody := struct {
		Acknowledged bool `json:"acknowledged"`
		Error string `json:"error"`
	}{}
	err = util.FromJSONBytes(res.Body, &resBody)
	if err != nil {
		return err
	}
	if resBody.Acknowledged {
		return nil
	}
	return fmt.Errorf(resBody.Error)
}

func (h *APIHandler) stopDataMigrationTask(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("task_id")
	obj := task2.Task{}
	obj.ID = id

	exists, err := orm.Get(&obj)
	if !exists || err != nil {
		h.WriteJSON(w, util.MapStr{
			"_id":   id,
			"found": false,
		}, http.StatusNotFound)
		return
	}
	execution, _ := util.MapStr(obj.Parameters).GetValue("pipeline.config.settings.execution")
	if execution == nil {
		execution, err = util.MapStr(obj.Parameters).GetValue("pipeline.config.execution")
		if err != nil {
			errStr := fmt.Sprintf("get execution config in task %s error: %s", id, err.Error())
			h.WriteError(w, errStr, http.StatusInternalServerError)
			log.Error(errStr)
			return
		}
	}
	buf := util.MustToJSONBytes(execution)
	executionConfig := migration.ExecutionConfig{}
	err = util.FromJSONBytes(buf, &executionConfig)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}

	if len(executionConfig.Nodes.Permit) == 0 {
		h.WriteError(w, "node of running task can not found", http.StatusInternalServerError)
		return
	}

	err = stopTask(executionConfig.Nodes.Permit[0].ID, id)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}

	query := util.MapStr{
			"bool": util.MapStr{
				"must": []util.MapStr{
					{
						"term": util.MapStr{
							"parent_id": util.MapStr{
								"value": id,
							},
						},
					},
					{
						"term": util.MapStr{
							"metadata.labels.pipeline_id": util.MapStr{
								"value": "index_migration",
							},
						},
					},
					{
						"terms": util.MapStr{
							"status": []string{"running", "init"},
						},
					},
				},
			},
	}
	//todo reset stat_time?
	queryDsl := util.MapStr{
		"query": util.MapStr{
			"bool": util.MapStr{
				"minimum_should_match": 1,
				"should": []util.MapStr{
					{
						"term": util.MapStr{
							"id": util.MapStr{
								"value": id,
							},
						},
					},
					query,
				},
			},
		},
		"script": util.MapStr{
			"source": "ctx._source['status'] = 'stopped'",
		},
	}

	err = orm.UpdateBy(task2.Task{}, util.MustToJSONBytes(queryDsl))
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.WriteJSON(w, util.MapStr{
		"success": true,
	}, 200)
}

func getTaskConfig(task *task2.Task, config interface{}) error{
	configSec, err := util.MapStr(task.Parameters).GetValue("pipeline.config")
	if err != nil {
		return err
	}
	configBytes, err := util.ToJSONBytes(configSec)
	if err != nil {
		return err
	}

	return util.FromJSONBytes(configBytes, config)
}

func (h *APIHandler) getDataMigrationTaskInfo(w http.ResponseWriter, req *http.Request, ps httprouter.Params){
	id := ps.MustGetParameter("task_id")

	obj := task2.Task{}
	obj.ID = id

	exists, err := orm.Get(&obj)
	if !exists || err != nil {
		h.WriteJSON(w, util.MapStr{
			"_id":   id,
			"found": false,
		}, http.StatusNotFound)
		return
	}
	taskConfig := &migration.ElasticDataConfig{}
	err = getTaskConfig(&obj, taskConfig)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	taskErrors, err := getErrorPartitionTasks(id)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	//get status of sub task
	query :=  util.MapStr{
		"size": 1000,
		"query": util.MapStr{
			"bool": util.MapStr{
				"must": []util.MapStr{
					{
						"term": util.MapStr{
							"parent_id": util.MapStr{
								"value": id,
							},
						},
					},{
						"term": util.MapStr{
							"metadata.labels.pipeline_id": util.MapStr{
								"value": "index_migration",
							},
						},
					},
					{
						"term": util.MapStr{
							"metadata.labels.level": util.MapStr{
								"value": "index",
							},
						},
					},
				},
			},
		},
	}
	q := orm.Query{
		RawQuery: util.MustToJSONBytes(query),
	}
	err, result := orm.Search(task2.Task{}, &q)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	statusM := util.MapStr{}
	for _, row := range result.Result {
		if rowM, ok := row.(map[string]interface{}); ok {
			if v, ok := rowM["id"].(string); ok {
				statusM[v] = rowM["status"]
			}
		}
	}

	targetESClient := elastic.GetClient(taskConfig.Cluster.Target.Id)
	var completedIndices int
	for i, index := range taskConfig.Indices {
		count, err := getIndexTaskCount(&index, targetESClient)
		if err != nil {
			log.Error(err)
			continue
		}
		if st, ok := statusM[index.ID]; ok {
			taskConfig.Indices[i].Status = st.(string)
		}
		percent := float64(count * 100) / float64(index.Source.Docs)
		taskConfig.Indices[i].Target.Docs = count
		taskConfig.Indices[i].Percent = util.ToFixed(percent, 2)
		taskConfig.Indices[i].ErrorPartitions = taskErrors[index.ID]
		if count == index.Source.Docs {
			completedIndices ++
			taskConfig.Indices[i].Status = "complete"
		}
	}
	util.MapStr(obj.Parameters).Put("pipeline.config", taskConfig)
	obj.Metadata.Labels["completed_indices"] = completedIndices
	h.WriteJSON(w, obj, http.StatusOK)
}
func getErrorPartitionTasks(taskID string) (map[string]int, error){
	query := util.MapStr{
		"size": 0,
		"aggs": util.MapStr{
			"group_by_task": util.MapStr{
				"terms": util.MapStr{
					"field": "parent_id",
					"size": 100,
				},
			},
		},
		"query": util.MapStr{
			"bool": util.MapStr{
				"must": []util.MapStr{
					{
						"term": util.MapStr{
							"metadata.labels.pipeline_id": util.MapStr{
								"value": "index_migration",
							},
						},
					},
					{
						"term": util.MapStr{
							"runnable": util.MapStr{
								"value": true,
							},
						},
					},
					{
						"term": util.MapStr{
							"status": util.MapStr{
								"value": "error",
							},
						},
					},
					{
						"term": util.MapStr{
							"parent_id": util.MapStr{
								"value": taskID,
							},
						},
					},
				},
			},
		},
	}
	q := &orm.Query{
		RawQuery: util.MustToJSONBytes(query),
	}
	err, result := orm.Search(task2.Task{}, q)
	if err != nil {
		return nil, err
	}

	searchRes := &elastic.SearchResponse{}
	err = util.FromJSONBytes(result.Raw, searchRes)
	if err != nil {
		return nil, err
	}
	resBody := map[string]int{}

	if taskAgg, ok := searchRes.Aggregations["group_by_task"]; ok {
		for _, bk := range taskAgg.Buckets {
			if key, ok := bk["key"].(string); ok {
				if _, ok = resBody[key]; !ok {
					continue
				}
				resBody[key] = int(bk["doc_count"].(float64))
			}
		}
	}
	return resBody, nil
}

func getIndexTaskCount(index *migration.IndexConfig, targetESClient elastic.API) (int64, error) {
	targetIndexName := index.Target.Name
	if targetIndexName == "" {
		if v, ok := index.IndexRename[index.Source.Name].(string); ok {
			targetIndexName = v
		}
	}

	var body []byte
	if index.RawFilter != nil {
		query := util.MapStr{
			"query": index.RawFilter,
		}
		body = util.MustToJSONBytes(query)
	}
	countRes, err := targetESClient.Count(targetIndexName, body)
	return countRes.Count, err
}

func getExecutionConfig(parameters map[string]interface{}, key string)(*migration.ExecutionConfig, error){
	execution, err := util.MapStr(parameters).GetValue(key)
	if err != nil {
		return nil, err
	}
	buf := util.MustToJSONBytes(execution)
	executionConfig := migration.ExecutionConfig{}
	err = util.FromJSONBytes(buf, &executionConfig)
	return &executionConfig, err
}

func getTaskStats(nodeID string) (map[string]interface{}, error){
	endpoint, err := getNodeEndpoint(nodeID)
	if err != nil {
		return nil, err
	}
	res, err := proxy.DoProxyRequest(&proxy.Request{
		Method: http.MethodGet,
		Endpoint: endpoint,
		Path: "/stats",
	})

	if err != nil {
		return nil, fmt.Errorf("call stats api error: %w", err)
	}
	resBody := struct {
		Stats map[string]interface{} `json:"stats"`
	}{}
	err = util.FromJSONBytes(res.Body, &resBody)
	return resBody.Stats, err
}

func (h *APIHandler) getDataMigrationTaskOfIndex(w http.ResponseWriter, req *http.Request, ps httprouter.Params){
	id := ps.MustGetParameter("task_id")
	indexTask := task2.Task{
		ID: id,
	}
	exists, err := orm.Get(&indexTask)
	if !exists || err != nil {
		h.WriteError(w, fmt.Sprintf("task [%s] not found", id), http.StatusInternalServerError)
		return
	}

	var durationInMS int64
	if indexTask.StartTimeInMillis > 0 {
		durationInMS = time.Now().UnixMilli() - indexTask.StartTimeInMillis
		if indexTask.CompleteTime != nil {
			durationInMS = indexTask.CompleteTime.UnixMilli() - indexTask.StartTimeInMillis
		}
	}

	taskInfo := util.MapStr{
		"task_id": id,
		"start_time": indexTask.StartTimeInMillis,
		"status": indexTask.Status,
		"data_partition": indexTask.Metadata.Labels["partition_count"],
		"complete_time": indexTask.CompleteTime,
		"duration": durationInMS,
	}
	partitionTaskQuery := util.MapStr{
		"size": 500,
		"sort": []util.MapStr{
			{
				"created": util.MapStr{
					"order": "asc",
				},
			},
		},
		"query": util.MapStr{
			"bool": util.MapStr{
				"must": []util.MapStr{
					{
						"term": util.MapStr{
							"parent_id": util.MapStr{
								"value": id,
							},
						},
					},{
						"term": util.MapStr{
							"metadata.labels.pipeline_id": util.MapStr{
								"value": "index_migration",
							},
						},
					},
				},
			},
		},
	}
	q := &orm.Query{
		RawQuery: util.MustToJSONBytes(partitionTaskQuery),
	}
	err, result := orm.Search(task2.Task{}, q)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	executionConfig, err := getExecutionConfig(indexTask.Parameters, "pipeline.config.execution")
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(executionConfig.Nodes.Permit) == 0 {
		h.WriteError(w, "node of running task can not found", http.StatusInternalServerError)
		return
	}
	stats, err := getTaskStats(executionConfig.Nodes.Permit[0].ID)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var (
		partitionTaskInfos []util.MapStr
		completedPartitions int
	)
	for i, row := range result.Result {
		buf := util.MustToJSONBytes(row)
		ptask := task2.Task{}
		err = util.FromJSONBytes(buf, &ptask)
		if err != nil {
			log.Error(err)
			continue
		}
		start, _ := util.MapStr(ptask.Parameters).GetValue("pipeline.config.source.start")
		end, _ := util.MapStr(ptask.Parameters).GetValue("pipeline.config.source.end")
		if i == 0 {
			step, _ := util.MapStr(ptask.Parameters).GetValue("pipeline.config.source.step")
			taskInfo["step"] = step
		}
		durationInMS = 0
		if ptask.StartTimeInMillis > 0 {
			durationInMS = time.Now().UnixMilli() - ptask.StartTimeInMillis
			if ptask.CompleteTime != nil {
				durationInMS = ptask.CompleteTime.UnixMilli() - ptask.StartTimeInMillis
			}
		}
		var (
			scrollDocs float64
			indexDocs float64
		)
		if pt, ok := stats[ptask.ID]; ok {
			if ptv, ok := pt.(map[string]interface{}); ok {
				if v, ok := ptv["scroll_docs"].(float64); ok {
					scrollDocs = v
				}
				if v, ok := ptv["bulk_docs"].(float64); ok {
					indexDocs = v
				}
			}
		}

		partitionTotalDocs, _ := util.MapStr(ptask.Parameters).GetValue("pipeline.config.source.doc_count")
		partitionTaskInfos = append(partitionTaskInfos, util.MapStr{
			"task_id": ptask.ID,
			"status": ptask.Status,
			"start_time": ptask.StartTimeInMillis,
			"complete_time": ptask.CompleteTime,
			"start": start,
			"end": end,
			"duration": durationInMS,
			"scroll_docs": scrollDocs,
			"index_docs": indexDocs,
			"total_docs": partitionTotalDocs,
		})
		if ptask.Status == "complete" {
			completedPartitions++
		}
	}
	taskInfo["partitions"] = partitionTaskInfos
	taskInfo["completed_partitions"] = completedPartitions
	h.WriteJSON(w, taskInfo, http.StatusOK)
}

func (h *APIHandler) getMigrationTask(w http.ResponseWriter, req *http.Request, ps httprouter.Params){
	id := ps.MustGetParameter("task_id")

	obj := task2.Task{}
	obj.ID = id

	exists, err := orm.Get(&obj)
	if !exists || err != nil {
		h.WriteJSON(w, util.MapStr{
			"_id":   id,
			"found": false,
		}, http.StatusNotFound)
		return
	}

	h.WriteJSON(w, util.MapStr{
		"found":   true,
		"_id":     id,
		"_source": obj,
	}, 200)
}

func (h *APIHandler) countDocuments(w http.ResponseWriter, req *http.Request, ps httprouter.Params){
	var (
		index = ps.MustGetParameter("index")
		clusterID = ps.MustGetParameter("id")
	)
	client := elastic.GetClient(clusterID)
	reqBody := struct {
		Filter interface{} `json:"filter"`
	}{}
	err := h.DecodeJSON(req, &reqBody)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var query []byte
	if reqBody.Filter != nil {
		query = util.MustToJSONBytes(util.MapStr{
			"query": reqBody.Filter,
		})
	}

	countRes, err := client.Count(index, query)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.WriteJSON(w, countRes, http.StatusOK)
}

func (h *APIHandler) getMigrationTaskLog(w http.ResponseWriter, req *http.Request, ps httprouter.Params){
	id := ps.MustGetParameter("task_id")
	query := util.MapStr{
		"sort": []util.MapStr{
			{
				"timestamp": util.MapStr{
					"order": "asc",
				},
			},
		},
		"query": util.MapStr{
			"bool": util.MapStr{
				"minimum_should_match": 1,
				"should": []util.MapStr{
					{
						"term": util.MapStr{
							"parent_id": util.MapStr{
								"value": id,
							},
						},
					},{
						"term": util.MapStr{
							"id": util.MapStr{
								"value": id,
							},
						},
					},
				},
			},
		},
	}

	q := &orm.Query{
		RawQuery: util.MustToJSONBytes(query),
	}
	err, _ := orm.Search(task2.Log{}, q)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
	}
}

func (h *APIHandler) updateDataMigrationTaskStatus(w http.ResponseWriter, req *http.Request, ps httprouter.Params){
	id := ps.MustGetParameter("task_id")

	obj := task2.Task{}
	obj.ID = id

	exists, err := orm.Get(&obj)
	if !exists || err != nil {
		h.WriteJSON(w, util.MapStr{
			"_id":   id,
			"found": false,
		}, http.StatusNotFound)
		return
	}
	reqBody := struct {
		Status string `json:"status"`
	}{}
	err = h.DecodeJSON(req, &reqBody)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	obj.Status = reqBody.Status
	err = orm.Update(obj)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.WriteJSON(w, util.MapStr{
		"success": true,
	}, 200)
}