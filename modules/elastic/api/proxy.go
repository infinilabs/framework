package api

import (
	"context"
	"crypto/tls"
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/segmentio/encoding/json"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/lib/fasthttp"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (h *APIHandler) HandleProxyAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	resBody := map[string]interface{}{
	}
	targetClusterID := ps.ByName("id")
	method := h.GetParameterOrDefault(req, "method", "")
	path := h.GetParameterOrDefault(req, "path", "")
	if method == "" || path == ""{
		resBody["error"] = fmt.Errorf("parameter method and path is required")
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	exists,_,err:=h.GetClusterClient(targetClusterID)

	if err != nil {
		log.Error(err)
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	if !exists{
		resBody["error"] = fmt.Sprintf("cluster [%s] not found",targetClusterID)
		log.Error(resBody["error"])
		h.WriteJSON(w, resBody, http.StatusNotFound)
		return
	}

	authPath, _ := url.PathUnescape(path)
	reqUrl, err := url.Parse(authPath)
	if err != nil {
		log.Error(err)
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	newReq := req.Clone(context.Background())
	newReq.URL = reqUrl
	newReq.Method = method
	_, err = h.ValidateProxyRequest(newReq, targetClusterID)
	if err != nil {
		log.Error(err)
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusForbidden)
		return
	}
	//if permission != "" {
	//	if permission == "cat.indices" || permission == "cat.shards" {
	//		reqUrl.Path
	//	}
	//}


	var (
		freq = fasthttp.AcquireRequest()
		fres = fasthttp.AcquireResponse()
	)
	defer func() {
		fasthttp.ReleaseRequest(freq)
		fasthttp.ReleaseResponse(fres)
	}()
	metadata := elastic.GetMetadata(targetClusterID)
	if metadata==nil{
		resBody["error"] = fmt.Sprintf("cluster [%s] metadata not found",targetClusterID)
		log.Error(resBody["error"])
		h.WriteJSON(w, resBody, http.StatusNotFound)
		return
	}

	if metadata.Config.BasicAuth != nil {
		freq.SetBasicAuth(metadata.Config.BasicAuth.Username, metadata.Config.BasicAuth.Password)
	}

	endpoint:=fmt.Sprintf("%s/%s", metadata.GetActivePreferredSeedEndpoint(), path)

	freq.SetRequestURI(endpoint)

	method = strings.ToUpper(method)
	if method == http.MethodGet && req.ContentLength > 0 {
		method = http.MethodPost
	}
	freq.Header.SetMethod(method)

	freq.SetBodyStream(req.Body, -1)
	defer req.Body.Close()
	client := &fasthttp.Client{
		MaxConnsPerHost: 1000,
		TLSConfig:       &tls.Config{InsecureSkipVerify: true},
		ReadTimeout: 5 *time.Second,
		WriteTimeout: 5 *time.Second,
	}
	err = client.Do(freq, fres)
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	okBody := struct {
		RequestHeader string `json:"request_header"`
		ResponseHeader string `json:"response_header"`
		ResponseBody string `json:"response_body"`
	}{
		RequestHeader: freq.Header.String(),
		ResponseHeader: fres.Header.String(),
		ResponseBody: string(fres.Body()),
	}

	w.Header().Set("Content-type", string(fres.Header.ContentType()))
	w.WriteHeader(fres.StatusCode())
	json.NewEncoder(w).Encode(okBody)

}
