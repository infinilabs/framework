/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package api

import (
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"net/http"
)

func (h *APIHandler) HandleSearchActivityAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params){
	resBody:=util.MapStr{}
	reqBody := struct{
		Keyword string `json:"keyword"`
		Size int `json:"size"`
		From int `json:"from"`
		Aggregations []elastic.SearchAggParam `json:"aggs"`
		Highlight elastic.SearchHighlightParam `json:"highlight"`
		Filter elastic.SearchFilterParam `json:"filter"`
		Sort []string `json:"sort"`
		StartTime interface{} `json:"start_time"`
		EndTime interface{} `json:"end_time"`
	}{}
	err := h.DecodeJSON(req, &reqBody)
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w,resBody, http.StatusInternalServerError )
		return
	}
	aggs := elastic.BuildSearchTermAggregations(reqBody.Aggregations)
	aggs["term_cluster_id"] = util.MapStr{
		"terms": util.MapStr{
			"field": "metadata.labels.cluster_id",
			"size": 1000,
		},
		"aggs": util.MapStr{
			"term_cluster_name": util.MapStr{
				"terms": util.MapStr{
					"field": "metadata.labels.cluster_name",
					"size": 1,
				},
			},
		},
	}
	filter := elastic.BuildSearchTermFilter(reqBody.Filter)
	if reqBody.StartTime != "" {
		filter = append(filter, util.MapStr{
			"range": util.MapStr{
				"timestamp": util.MapStr{
					"gte": reqBody.StartTime,
					"lte": reqBody.EndTime,
				},
			},
		})
	}
	var should = []util.MapStr{}
	if reqBody.Keyword != "" {
		should = []util.MapStr{
			{
				"prefix": util.MapStr{
					"metadata.labels.node_name": util.MapStr{
						"value": reqBody.Keyword,
						"boost": 15,
					},
				},
			},
			{
				"prefix": util.MapStr{
					"metadata.labels.cluster_name": util.MapStr{
						"value": reqBody.Keyword,
						"boost": 10,
					},
				},
			},
			{
				"prefix": util.MapStr{
					"metadata.labels.index_name": util.MapStr{
						"value": reqBody.Keyword,
						"boost": 20,
					},
				},
			},
		}
	}
	query := util.MapStr{
		"aggs":      aggs,
		"size":      reqBody.Size,
		"from": reqBody.From,
		"_source": []string{"payload.diff", "id", "metadata", "timestamp"},
		"highlight": elastic.BuildSearchHighlight(&reqBody.Highlight),
		"query": util.MapStr{
			"bool": util.MapStr{
				"filter": filter,
				"should": should,
			},
		},
	}
	if len(reqBody.Sort) == 0 {
		reqBody.Sort = []string{"timestamp", "desc"}
	}

	query["sort"] =  []util.MapStr{
		{
			reqBody.Sort[0]: util.MapStr{
				"order": reqBody.Sort[1],
			},
		},
	}

	dsl := util.MustToJSONBytes(query)
	response, err := elastic.GetClient(h.Config.Elasticsearch).SearchWithRawQueryDSL(orm.GetIndexName(event.Activity{}), dsl)
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w,resBody, http.StatusInternalServerError )
		return
	}
	w.Write(response.RawResult.Body)
}