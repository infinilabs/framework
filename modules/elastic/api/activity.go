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

	clusterFilter, hasAllPrivilege := h.GetClusterFilter(req, "metadata.labels.cluster_id")
	if !hasAllPrivilege && clusterFilter == nil {
		h.WriteJSON(w, elastic.SearchResponse{

		}, http.StatusOK)
		return
	}
	if !hasAllPrivilege && clusterFilter != nil {
		filter = append(filter, clusterFilter)
	}

	var should = []util.MapStr{}
	if reqBody.Keyword != "" {
		should = []util.MapStr{
			{
				"prefix": util.MapStr{
					"metadata.labels": util.MapStr{
						"value": reqBody.Keyword,
						"boost": 15,
					},
				},
			},
			{
				"prefix": util.MapStr{
					"changelog": util.MapStr{
						"value": reqBody.Keyword,
						"boost": 10,
					},
				},
			},
			{
				"match": util.MapStr{
					"metadata.labels": util.MapStr{
						"query":                reqBody.Keyword,
						"fuzziness":            "AUTO",
						"max_expansions":       10,
						"prefix_length":        2,
						"fuzzy_transpositions": true,
						"boost":                5,
					},
				},
			},
			{
				"match": util.MapStr{
					"changelog": util.MapStr{
						"query":                reqBody.Keyword,
						"fuzziness":            "AUTO",
						"max_expansions":       10,
						"prefix_length":        2,
						"fuzzy_transpositions": true,
						"boost":                5,
					},
				},
			},
		}
	}
	var boolQuery = util.MapStr{
		"filter": filter,
	}
	if len(should) >0 {
		boolQuery["should"] = should
		boolQuery["minimum_should_match"] = 1
	}
	query := util.MapStr{
		"aggs":      aggs,
		"size":      reqBody.Size,
		"from": reqBody.From,
		"_source": []string{"changelog", "id", "metadata", "timestamp"},
		"highlight": elastic.BuildSearchHighlight(&reqBody.Highlight),
		"query": util.MapStr{
			"bool": boolQuery,
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