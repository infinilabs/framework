/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package api

import (
	"fmt"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"net/http"
	log "src/github.com/cihub/seelog"
)

func (h *APIHandler) HandleRemoveLocalState(w http.ResponseWriter, req *http.Request, ps httprouter.Params){
	//todo add UI
	var (
		typ = ps.MustGetParameter("type")
		target = ps.ByName("target")
	)
	switch typ {
	case "index_state":
		indexName := orm.GetIndexName(elastic.ElasticsearchConfig{})
		searchRes, err := h.Client().SearchWithRawQueryDSL(indexName, util.MustToJSONBytes(util.MapStr{
			"size": 1000,
		}))
		if err != nil {
			if err != nil {
				log.Error(err)
				h.WriteJSON(w, util.MapStr{
					"error": err.Error(),
				}, http.StatusInternalServerError)
				return
			}
		}
		if target == "*" {
			for _, conf := range searchRes.Hits.Hits {
				err = kv.DeleteKey(elastic.KVElasticIndexMetadata, []byte(conf.ID))
				if err != nil {
					log.Error(err)
					h.WriteJSON(w, util.MapStr{
						"error": err.Error(),
					}, http.StatusInternalServerError)
					return
				}
			}
		}else{
			found := false
			for _, conf := range searchRes.Hits.Hits {
				if conf.ID == target {
					found = true
				}
			}
			if !found {
				h.WriteJSON(w, util.MapStr{
					"error": fmt.Sprintf("target %s not found", target),
				}, http.StatusNotFound)
			}
			err = kv.DeleteKey(elastic.KVElasticIndexMetadata, []byte(target))
			if err != nil {
				log.Error(err)
				h.WriteJSON(w, util.MapStr{
					"error": err.Error(),
				}, http.StatusInternalServerError)
				return
			}
		}
	}
	h.WriteJSON(w, util.MapStr{
		"result": "deleted",
	}, http.StatusOK)
}