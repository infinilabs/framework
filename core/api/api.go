// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"github.com/gorilla/context"
	"github.com/rs/cors"
	"golang.org/x/net/http2"
	common "infini.sh/framework/core/api/common"
	"infini.sh/framework/core/api/filter"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/api/websocket"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	_ "infini.sh/framework/core/logging"
	"infini.sh/framework/core/logging/logger"
	"infini.sh/framework/core/util"
)

// RegisteredAPIFuncHandler is a hub for registered api
var registeredAPIFuncHandler = make(map[string]func(http.ResponseWriter, *http.Request))

// RegisteredAPIMethodHandler is a hub for registered api
var registeredAPIMethodHandler = make(map[string]map[string]func(w http.ResponseWriter, req *http.Request, ps httprouter.Params))

var apiOptions = NewOptionRegistry()

var l sync.Mutex

var filters []filter.Filter

func RegisterAPIFilter(f filter.Filter) {
	l.Lock()
	defer l.Unlock()
	filters = append(filters, f)
}

var APIs = map[string]util.KV{}

// TODO to be removed
func IsAuthEnable() bool {
	return common.IsAuthEnable()
}

// HandleAPIFunc register api handler to specify pattern
func HandleAPIFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	l.Lock()
	if registeredAPIFuncHandler == nil {
		registeredAPIFuncHandler = map[string]func(http.ResponseWriter, *http.Request){}
	}
	registeredAPIFuncHandler[pattern] = handler
	l.Unlock()
}

func initializeAPI() {
	l.Lock()
	defer l.Unlock()

	for pattern, handler := range registeredAPIFuncHandler {
		for _, f := range filters {
			handler = f.FilterHttpHandlerFunc(pattern, handler)
		}

		APIs[pattern+"*"] = util.KV{Key: "*", Value: pattern}

		log.Debugf("register custom http handler: %v", pattern)
		mux.HandleFunc(pattern, handler)
	}

	for m, handlers := range registeredAPIMethodHandler {
		for pattern, handler := range handlers {
			//Apply handler filters
			for _, f := range filters {
				handler = f.FilterHttpRouter(pattern, handler)
			}

			APIs[pattern+m] = util.KV{Key: m, Value: pattern}

			log.Debugf("register http handler: %v %v, total apis: %v", m, pattern, len(APIs))

			router.Handle(m, pattern, handler)
		}
	}
}

// HandleAPIMethod register api handler
func HandleAPIMethod(method Method, pattern string, handler func(w http.ResponseWriter, req *http.Request, ps httprouter.Params)) {
	l.Lock()
	if registeredAPIMethodHandler == nil {
		registeredAPIMethodHandler = map[string]map[string]func(w http.ResponseWriter, req *http.Request, ps httprouter.Params){}
	}

	m := string(method)
	m1 := registeredAPIMethodHandler[m]
	if m1 == nil {
		registeredAPIMethodHandler[m] = map[string]func(w http.ResponseWriter, req *http.Request, ps httprouter.Params){}
	}
	registeredAPIMethodHandler[m][pattern] = handler

	l.Unlock()
}

var router = httprouter.New(mux)
var mux = http.NewServeMux()

var certPool *x509.CertPool
var rootCert *x509.Certificate
var rootKey *rsa.PrivateKey
var rootCertPEM []byte

var apiConfig *config.APIConfig

var listenAddress string

var notfoundHandler = http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
	rw.Write([]byte("{\"message\":\"not_found\"}"))
	rw.WriteHeader(404)
})

func SetNotFoundHandler(handler func(rw http.ResponseWriter, r *http.Request)) {
	notfoundHandler = handler
}

var DefaultAPI = Handler{}

// StartAPI will start listen and act as the API server
func StartAPI() {

	apiConfig = &global.Env().SystemConfig.APIConfig

	if !apiConfig.Enabled {
		return
	}
	if apiConfig.WebsocketConfig.Enabled {
		websocket.InitWebSocket(apiConfig.WebsocketConfig)
		HandleAPIFunc(apiConfig.WebsocketConfig.BasePath, websocket.ServeWs)
		logger.RegisterWebsocketHandler(func(message string, level log.LogLevel, context log.LogContextInterface) {
			websocket.BroadcastMessage(message)
		})
		if registeredWebSocketCommandHandler != nil {
			for k, v := range registeredWebSocketCommandHandler {
				log.Debug("register websocket handler: ", k, " ", v)
				websocket.HandleWebSocketCommand(k, webSocketCommandUsage[k], v)
			}
		}
	}

	c := cors.New(cors.Options{
		AllowedOrigins:   apiConfig.CrossDomain.AllowedOrigins,
		AllowCredentials: true,
		AllowedMethods:   []string{"HEAD", "GET", "POST", "DELETE", "PUT", "OPTIONS"},
	})

	//init api handlers
	if apiConfig.Security.Enabled {
		apiBasicAuthFilter := BasicAuthFilter{
			Username: apiConfig.Security.Username,
			Password: apiConfig.Security.Password,
		}

		//register api filters
		RegisterAPIFilter(&apiBasicAuthFilter)
	}

	//TODO support filter out specify api
	initializeAPI()

	if apiConfig.NetworkConfig.SkipOccupiedPort {
		listenAddress = util.AutoGetAddress(apiConfig.NetworkConfig.GetBindingAddr())
	} else {
		listenAddress = apiConfig.NetworkConfig.GetBindingAddr()
	}

	if util.ContainStr(listenAddress, util.AnyAddress) {
		ips := util.GetLocalIPs()
		if len(ips) > 0 {
			log.Infof("local ips: %v", util.JoinArray(ips, ", "))
		}
	}

	l, err := net.Listen("tcp", listenAddress)

	if err != nil {
		panic(err)
	}

	router.NotFound = notfoundHandler

	tlsCfg := apiConfig.TLSConfig

	schema := "http://"
	if tlsCfg.TLSEnabled {
		schema = "https://"

		log.Trace("using tls connection")

		cfg, err := GetServerTLSConfig(&tlsCfg)
		if err != nil {
			panic(err)
		}

		srv := &http.Server{
			ReadTimeout:       5 * time.Second,
			WriteTimeout:      10 * time.Second,
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       10 * time.Second,
			Addr:              listenAddress,
			Handler:           RecoveryHandler()(c.Handler(context.ClearHandler(http.StripPrefix(apiConfig.BasePath,router)))),
			TLSConfig:         cfg,
		}

		http2.ConfigureServer(srv, &http2.Server{
			MaxHandlers:          1000,
			MaxConcurrentStreams: 1000,
		})

		go func() {
			defer func() {
				if !global.Env().IsDebug {
					if r := recover(); r != nil {
						var v string
						switch r.(type) {
						case error:
							v = r.(error).Error()
						case runtime.Error:
							v = r.(runtime.Error).Error()
						case string:
							v = r.(string)
						}
						log.Error(v)
					}
				}
			}()

			err = srv.ServeTLS(l, "", "")
			if err != nil {
				log.Error(err)
				panic(err)
			}
		}()

	} else {
		log.Trace("starting insecure API server")
		go func() {
			defer func() {
				if !global.Env().IsDebug {
					if r := recover(); r != nil {
						var v string
						switch r.(type) {
						case error:
							v = r.(error).Error()
						case runtime.Error:
							v = r.(runtime.Error).Error()
						case string:
							v = r.(string)
						}
						log.Error(v)
					}
				}
			}()

			err := http.Serve(l, RecoveryHandler()(c.Handler(context.ClearHandler(http.StripPrefix(apiConfig.BasePath, router)))))
			if err != nil {
				log.Error(err)
				panic(err)
			}
		}()
	}

	err = util.WaitServerUp(listenAddress, 30*time.Second)
	if err != nil {
		panic(errors.Wrap(err, fmt.Sprintf("failed to listen on: %v", listenAddress)))
	}

	log.Info("api server listen at: ", schema, listenAddress)

}
