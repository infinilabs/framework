/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package server

import (
	"crypto/tls"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/model"
	"infini.sh/framework/core/proxy"
	"infini.sh/framework/core/util"
	"infini.sh/framework/plugins/managed/common"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type APIHandler struct {
	api.Handler
}

var serverInit = sync.Once{}
var configRepo common.ConfigRepo
var handler = APIHandler{}

func init() {

	api.HandleAPIMethod(api.POST, common.SYNC_API, handler.syncConfigs)          //client sync configs from config servers
	api.HandleAPIMethod(api.POST, "/configs/_reload", handler.refreshConfigsRepo) //client sync configs from config servers
	//delegate api to instances
	api.HandleAPIFunc("/ws_proxy", func(w http.ResponseWriter, req *http.Request) {
		log.Debug(req.RequestURI)
		endpoint := req.URL.Query().Get("endpoint")
		path := req.URL.Query().Get("path")
		var tlsConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
		target, err := url.Parse(endpoint)
		if err != nil {
			panic(err)
		}
		newURL, err := url.Parse(path)
		if err != nil {
			panic(err)
		}
		req.URL.Path = newURL.Path
		req.URL.RawPath = newURL.RawPath
		req.URL.RawQuery = ""
		req.RequestURI = req.URL.RequestURI()
		req.Header.Set("HOST", target.Host)
		req.Host = target.Host
		wsProxy := NewSingleHostReverseProxy(target)
		wsProxy.Dial = (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial
		wsProxy.TLSClientConfig = tlsConfig
		wsProxy.ServeHTTP(w, req)
	})
}

func ProxyRequestToRuntimeInstance(endpoint, method, path string, body interface{}, contentLength int64, auth *model.BasicAuth) (*proxy.Response, error) {

	req := &proxy.Request{
		Method:        method,
		Endpoint:      endpoint,
		Path:          path,
		Body:          body,
		BasicAuth:     auth,
		ContentLength: int(contentLength),
	}

	if global.Env().IsDebug {
		log.Debug(util.MustToJSON(req))
	}

	res, err := proxy.DoProxyRequest(req)

	if global.Env().IsDebug {
		if err != nil {
			log.Debug(err)
		}

		if res != nil {
			log.Debug(res.StatusCode, string(res.Body))
		}
	}

	return res, err
}
