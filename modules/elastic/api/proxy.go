package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"github.com/buger/jsonparser"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	log "github.com/cihub/seelog"
	"github.com/segmentio/encoding/json"
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/fasthttp"
)

func (h *APIHandler) HandleProxyAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	resBody := map[string]interface{}{}
	targetClusterID := ps.ByName("id")
	method := h.GetParameterOrDefault(req, "method", "")
	path := h.GetParameterOrDefault(req, "path", "")
	if method == "" || path == "" {
		resBody["error"] = fmt.Errorf("parameter method and path is required")
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	exists, _, err := h.GetClusterClient(targetClusterID)

	if err != nil {
		log.Error(err)
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	if !exists {
		resBody["error"] = fmt.Sprintf("cluster [%s] not found", targetClusterID)
		log.Error(resBody["error"])
		h.WriteJSON(w, resBody, http.StatusNotFound)
		return
	}

	authPath, _ := url.PathUnescape(path)
	var realPath = authPath
	//ccs search
	if parts := strings.SplitN(authPath, "/", 2); strings.Contains(parts[0], ":") {
		ccsParts := strings.SplitN(parts[0], ":", 2)
		realPath = fmt.Sprintf("%s/%s", ccsParts[1], parts[1])
	}
	newReq := req.Clone(context.Background())
	newURL, err := url.Parse(realPath)
	if err != nil {
		log.Error(err)
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	newReq.URL = newURL
	newReq.Method = method
	isSuperAdmin, permission, err := h.ValidateProxyRequest(newReq, targetClusterID)
	if err != nil {
		log.Error(err)
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusForbidden)
		return
	}
	if permission == "" && api.IsAuthEnable() && !isSuperAdmin {
		resBody["error"] = "unknown request path"
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
	if metadata == nil {
		resBody["error"] = fmt.Sprintf("cluster [%s] metadata not found", targetClusterID)
		log.Error(resBody["error"])
		h.WriteJSON(w, resBody, http.StatusNotFound)
		return
	}

	if metadata.Config.BasicAuth != nil {
		freq.SetBasicAuth(metadata.Config.BasicAuth.Username, metadata.Config.BasicAuth.Password)
	}

	endpoint := util.JoinPath(metadata.GetActivePreferredSeedEndpoint(), path)

	freq.SetRequestURI(endpoint)
	method = strings.ToUpper(method)
	freq.Header.SetMethod(method)
	freq.Header.SetUserAgent(req.Header.Get("user-agent"))
	freq.Header.SetReferer(endpoint)
	rurl, _ := url.Parse(endpoint)

	if rurl != nil {
		freq.Header.SetHost(rurl.Host)
		freq.Header.SetRequestURI(rurl.RequestURI())
	}

	clonedURI := freq.CloneURI()
	defer fasthttp.ReleaseURI(clonedURI)
	clonedURI.SetScheme(metadata.GetSchema())
	freq.SetURI(clonedURI)

	if permission == "cluster.search" {
		indices, hasAll := h.GetAllowedIndices(req, targetClusterID)
		if !hasAll && len(indices) == 0 {
			h.WriteJSON(w, elastic.SearchResponse{}, http.StatusOK)
			return
		}
		if hasAll {
			freq.SetBodyStream(req.Body, int(req.ContentLength))
		} else {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				log.Error(err)
				h.WriteError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if len(body) == 0 {
				body = []byte("{}")
			}
			v, _, _, _ := jsonparser.Get(body, "query")
			newQ := bytes.NewBuffer([]byte(`{"bool": {"must": [{"terms": {"_index":`))
			indicesBytes := util.MustToJSONBytes(indices)
			newQ.Write(indicesBytes)
			newQ.Write([]byte("}}"))
			if len(v) > 0 {
				newQ.Write([]byte(","))
				newQ.Write(v)
			}
			newQ.Write([]byte(`]}}`))
			body, _ = jsonparser.Set(body, newQ.Bytes(), "query")
			freq.SetBody(body)
		}
	} else {
		freq.SetBodyStream(req.Body, int(req.ContentLength))
	}
	defer req.Body.Close()

	err = client.Do(freq, fres)
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	okBody := struct {
		RequestHeader  string `json:"request_header"`
		ResponseHeader string `json:"response_header"`
		ResponseBody   string `json:"response_body"`
	}{
		RequestHeader:  freq.Header.String(),
		ResponseHeader: fres.Header.String(),
		ResponseBody:   string(fres.GetRawBody()),
	}

	w.Header().Set("Content-type", string(fres.Header.ContentType()))
	w.WriteHeader(fres.StatusCode())
	json.NewEncoder(w).Encode(okBody)

}

var client = fasthttp.Client{
	MaxConnsPerHost: 1000,
	TLSConfig:       &tls.Config{InsecureSkipVerify: true},
	ReadTimeout:     60 * time.Second,
	WriteTimeout:    60 * time.Second,
	DialDualStack:   true,
	//Dial:            fasthttpproxy.FasthttpProxyHTTPDialerTimeout(time.Second * 2),
}
