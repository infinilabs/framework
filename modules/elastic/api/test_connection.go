/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	"fmt"
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/fasthttp"
	"infini.sh/framework/modules/elastic/common"
	"net/http"
	log "github.com/cihub/seelog"
	"github.com/segmentio/encoding/json"
	"strings"
)

type TestAPI struct {
	api.Handler
}

var testAPI= TestAPI{}

var testInited bool
func InitTestAPI()  {
	if !testInited{
		api.HandleAPIMethod(api.POST, "/elasticsearch/try_connect", testAPI.HandleTestConnectionAction)
		testInited=true
	}
}


func (h TestAPI) HandleTestConnectionAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var (
		freq = fasthttp.AcquireRequest()
		fres = fasthttp.AcquireResponse()
		resBody = map[string] interface{}{}
	)
	defer func() {
		fasthttp.ReleaseRequest(freq)
		fasthttp.ReleaseResponse(fres)
	}()
	var config = &elastic.ElasticsearchConfig{}
	err := h.DecodeJSON(req, &config)
	if err != nil {
		resBody["error"] = fmt.Sprintf("json decode error: %v", err)
		log.Errorf("json decode error: %v", err)
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	defer req.Body.Close()
	var url string
	if config.Endpoint!=""{
		url=config.Endpoint
	}else if config.Host!=""&&config.Schema!=""{
		url = fmt.Sprintf("%s://%s", config.Schema, config.Host)
		config.Endpoint=url
	}else{
		resBody["error"] = fmt.Sprintf("invalid config: %v", util.MustToJSON(config))
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	if url==""{
		panic(errors.Error("invalid url: "+util.MustToJSON(config)))
	}

	if !util.SuffixStr(url,"/"){
		url=fmt.Sprintf("%s/", url)
	}

	freq.SetRequestURI(url)
	freq.Header.SetMethod("GET")

	basicAuth, err := common.GetBasicAuth(config)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if strings.TrimSpace(basicAuth.Username) != ""{
		freq.SetBasicAuth(basicAuth.Username, basicAuth.Password)
	}

	err = client.Do(freq, fres)

	if err != nil {
		resBody["error"] = fmt.Sprintf("request error: %v", err)
		log.Error( "test_connection ", "request error: ", err)
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	b := fres.Body()
	clusterInfo := &elastic.ClusterInformation{}
	err = json.Unmarshal(b, clusterInfo)
	if err != nil {
		resBody["error"] = fmt.Sprintf("cluster info decode error: %v", err)
		log.Error(resBody["error"])
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	resBody["version"] = clusterInfo.Version.Number
	resBody["cluster_uuid"] = clusterInfo.ClusterUUID
	resBody["cluster_name"] = clusterInfo.ClusterName

	//fetch cluster health info
	freq.SetRequestURI(fmt.Sprintf("%s/_cluster/health", config.Endpoint))
	fres.Reset()
	err = client.Do(freq, fres)
	if err != nil {
		resBody["error"] = fmt.Sprintf("request cluster health info error: %v", err)
		log.Error(resBody["error"])
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	var statusCode = fres.StatusCode()
	if statusCode == http.StatusUnauthorized {
		resBody["error"] = fmt.Sprintf("required authentication credentials")
		log.Error(resBody["error"])
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	healthInfo := &elastic.ClusterHealth{}
	err = json.Unmarshal(fres.Body(), &healthInfo)
	if err != nil {
		resBody["error"] = fmt.Sprintf("cluster health info decode error: %v", err)
		log.Error(resBody["error"])
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	resBody["status"] = healthInfo.Status
	resBody["number_of_nodes"] = healthInfo.NumberOfNodes
	resBody["number_of_data_nodes"] = healthInfo.NumberOf_data_nodes
	resBody["active_shards"] = healthInfo.ActiveShards

	h.WriteJSON(w, resBody, http.StatusOK)

}
