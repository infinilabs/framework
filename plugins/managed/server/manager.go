/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package server

import (
	"crypto/tls"
	"fmt"
	log "github.com/cihub/seelog"
	common2 "infini.sh/console/modules/agent/common"
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/errors"
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

	api.HandleAPIMethod(api.POST, common.SYNC_API, handler.syncConfigs)           //client sync configs from config servers
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

var mTLSClient *http.Client //TODO get mTLSClient
var initOnce = sync.Once{}

func ProxyAgentRequest(endpoint string, req *util.Request, responseObjectToUnMarshall interface{}) (*util.Result, error) {
	var err error
	var res *util.Result

	initOnce.Do(func() {

		//get ca files
		agCfg := common2.GetAgentConfig()
		clientTLSCertFile, clientTLSKeyFile, err := common2.GetAgentInstanceCerts(agCfg.Setup.CACertFile, agCfg.Setup.CAKeyFile)
		if err != nil {
			panic(err)
		}

		//init client
		hClient, err := util.NewMTLSClient(
			agCfg.Setup.CACertFile,
			clientTLSCertFile,
			clientTLSKeyFile)
		if err != nil {
			panic(err)
		}
		mTLSClient = hClient
	})

	req.Url=endpoint+req.Path


	res, err = util.ExecuteRequestWithCatchFlag(mTLSClient, req, true)
	if err != nil || res.StatusCode != 200 {
		body := ""
		if res != nil {
			body = string(res.Body)
		}
		return res, errors.New(fmt.Sprintf("request error: %v, %v", err, body))
	}

	if res != nil {
		if res.Body != nil {
			if responseObjectToUnMarshall != nil {
				return res, util.FromJSONBytes(res.Body, responseObjectToUnMarshall)
			}
		}
	}

	return res, err
}
