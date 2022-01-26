package api

import (
	"fmt"
	"infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"net/http"
	"strings"
)

func (h *APIHandler) SearchClusterMetadata(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var (
		name        = h.GetParameterOrDefault(req, "name", "")
		queryDSL    = `{"query":{"bool":{"must":[%s]}}, "size": %d, "from": %d}`
		size     = h.GetIntOrDefault(req, "size", 20)
		from     = h.GetIntOrDefault(req, "from", 0)
		mustBuilder = &strings.Builder{}
	)

	if name != "" {
		mustBuilder.WriteString(fmt.Sprintf(`{"prefix":{"name.text": "%s"}}`, name))
	}

	if size <= 0 {
		size = 20
	}

	if from < 0 {
		from = 0
	}

	q := orm.Query{}
	queryDSL = fmt.Sprintf(queryDSL, mustBuilder.String(), size, from)
	q.RawQuery = []byte(queryDSL)

	err, res := orm.Search(&elastic.ElasticsearchConfig{}, &q)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}


	response:=elastic.SearchResponse{}
	util.FromJSONBytes(res.Raw,&response)
	for i,hit:=range  response.Hits.Hits{
		result:=util.MapStr{}
		result["metadata"]=hit.Source
		result["summary"]=getClusterStatus(hit.ID)
		result["metrics"]=getClusterMetrics(hit.ID)
		response.Hits.Hits[i].Source=result
	}

	h.WriteJSON(w, response,200)
}

func getClusterStatus(id interface{}) interface{} {
	return nil
}

func getClusterMetrics(id interface{}) interface{} {
	return nil
}

