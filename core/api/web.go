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
	ctx "context"
	"crypto/tls"
	log "github.com/cihub/seelog"
	"github.com/gorilla/context"
	"golang.org/x/net/http2"
	"infini.sh/framework/core/api/gzip"
	"infini.sh/framework/core/api/router"
	"infini.sh/framework/core/api/websocket"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/global"
	_ "infini.sh/framework/core/logging"
	"infini.sh/framework/core/util"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"strings"
	"sync"
	"time"
)

var uiRouter *httprouter.Router
var uiServeMux *http.ServeMux
var uiMutex sync.Mutex

var bindAddress string

func StopWeb(cfg config.WebAppConfig) {
	if srv != nil {
		ctx1, cancel := ctx.WithTimeout(ctx.Background(), 10*time.Second)
		defer cancel()
		err := srv.Shutdown(ctx1)
		if err != nil {
			panic(err)
		}

		log.Debug("stopping web server")
	}
}

func StartWeb(cfg config.WebAppConfig) {
	//start web ui
	uiServeMux = http.NewServeMux()

	uiRouter = httprouter.New(uiServeMux)
	//uiRouter.RedirectTrailingSlash=false
	//uiRouter.RedirectFixedPath=false

	uiRouter.NotFound = http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Write([]byte("Page Not Found"))
		rw.WriteHeader(404)
	})

	//registered handlers
	if registeredUIHandler != nil {
		for k, v := range registeredUIHandler {
			log.Debug("register http handler: ", k)
			uiServeMux.Handle(k, v)
		}
	}
	if registeredUIFuncHandler != nil {
		for k, v := range registeredUIFuncHandler {
			log.Debug("register http handler: ", k)
			uiServeMux.HandleFunc(k, v)
		}
	}
	if registeredUIMethodHandler != nil {
		for k, v := range registeredUIMethodHandler {
			for m, n := range v {
				log.Debug("register http handler: ", k, " ", m)
				uiRouter.Handle(k, m, n)
			}
		}
	}

	if cfg.EmbeddingAPI {
		if registeredAPIMethodHandler != nil {
			for k, v := range registeredAPIMethodHandler {
				for m, n := range v {
					log.Debug("register http handler: ", k, " ", m)
					uiRouter.Handle(k, m, n)
				}
			}
		}
		if registeredAPIFuncHandler != nil {
			for k, v := range registeredAPIFuncHandler {
				log.Debug("register http handler: ", k)
				uiServeMux.HandleFunc(k, v)
			}
		}
	}

	if cfg.WebsocketConfig.Enabled {
		websocket.InitWebSocket(cfg.WebsocketConfig)
		uiServeMux.HandleFunc("/ws", websocket.ServeWs)
		if registeredWebSocketCommandHandler != nil {
			for k, v := range registeredWebSocketCommandHandler {
				log.Debug("register websocket handler: ", k, " ", v)
				websocket.HandleWebSocketCommand(k, webSocketCommandUsage[k], v)
			}
		}
	}

	schema := "http://"

	if cfg.NetworkConfig.SkipOccupiedPort {
		bindAddress = util.AutoGetAddress(cfg.NetworkConfig.GetBindingAddr())
	} else {
		bindAddress = cfg.NetworkConfig.GetBindingAddr()
	}

	handler := context.ClearHandler(uiRouter)
	if cfg.Gzip.Enabled {
		log.Debug("gzip enabled")
		wrapper, _ := gzip.NewGzipLevelHandler(cfg.Gzip.Level)
		handler = wrapper(handler)
	}
	if cfg.TLSConfig.TLSEnabled {
		log.Debug("tls enabled")

		schema = "https://"

		cfg, err := GetServerTLSConfig(&cfg.TLSConfig)
		if err != nil {
			panic(err)
		}

		srv = &http.Server{
			Addr:         bindAddress,
			Handler:      globalInterceptorHandler.Handler(RecoveryHandler()(handler)),
			TLSConfig:    cfg,
			TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
		}

		http2.ConfigureServer(srv, &http2.Server{
			MaxHandlers:          1000,
			MaxConcurrentStreams: 1000,
		})

		go func(srv *http.Server) {
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
			err := srv.ListenAndServeTLS("", "")
			if err != nil && err != http.ErrServerClosed {
				log.Error(err)
				panic(err)
			}
		}(srv)

	} else {
		srv = &http.Server{Addr: bindAddress, Handler: globalInterceptorHandler.Handler(RecoveryHandler()(handler))}
		go func(srv *http.Server) {
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

			err := srv.ListenAndServe()
			if err != nil && err != http.ErrServerClosed {
				log.Error(err)
				panic(err)
			}
		}(srv)
	}

	err := util.WaitServerUp(bindAddress, 30*time.Second)
	if err != nil {
		panic(err)
	}

	log.Info("web server listen at: ", schema, bindAddress)

}

type Interceptor interface {
	Match(request *http.Request) bool
	PreHandle(c ctx.Context, writer http.ResponseWriter, request *http.Request) (ctx.Context, error)
	PostHandle(c ctx.Context, writer http.ResponseWriter, request *http.Request)
	Name() string
}

type InterceptorHandler struct {
	interceptors []Interceptor
}

func (i *InterceptorHandler) AddInterceptors(interceptors ...Interceptor) {
	for _, interceptor := range interceptors {
		if interceptor != nil {
			i.interceptors = append(i.interceptors, interceptor)
		}
	}
}

func (i *InterceptorHandler) Handler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var appliedInterceptors []Interceptor
		var appliedContexts []ctx.Context
		for _, interceptor := range i.interceptors {
			if !interceptor.Match(request) {
				continue
			}
			appliedInterceptors = append(appliedInterceptors, interceptor)
			if c, err := interceptor.PreHandle(ctx.Background(), writer, request); err != nil {
				log.Infof("encountered an error while calling the PreHandle method of %s, err: %s",
					interceptor.Name(), err.Error())
				return
			} else {
				appliedContexts = append(appliedContexts, c)
			}
		}
		handler.ServeHTTP(writer, request)
		for i := len(appliedInterceptors) - 1; i >= 0; i-- {
			interceptor := appliedInterceptors[i]
			c := appliedContexts[i]
			interceptor.PostHandle(c, writer, request)
		}
	})
}

func NewInterceptorHandler() *InterceptorHandler {
	return &InterceptorHandler{}
}

var globalInterceptorHandler = NewInterceptorHandler()
var globalInterceptorHandlerMtx sync.Mutex

func AddGlobalInterceptors(interceptors ...Interceptor) {
	globalInterceptorHandlerMtx.Lock()
	defer globalInterceptorHandlerMtx.Unlock()
	globalInterceptorHandler.AddInterceptors(interceptors...)
}

var srv *http.Server

// RegisteredUIHandler is a hub for registered ui handler
var registeredUIHandler map[string]http.Handler

// RegisteredUIFuncHandler is a hub for registered ui handler
var registeredUIFuncHandler map[string]func(http.ResponseWriter, *http.Request)

// RegisteredUIMethodHandler is a hub for registered ui handler
var registeredUIMethodHandler map[string]map[string]func(w http.ResponseWriter, req *http.Request, ps httprouter.Params)

var registeredWebSocketCommandHandler map[string]func(c *websocket.WebsocketConnection, array []string)
var webSocketCommandUsage map[string]string

// HandleUIFunc register ui request handler
func HandleUIFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	uiMutex.Lock()
	if registeredUIFuncHandler == nil {
		registeredUIFuncHandler = map[string]func(http.ResponseWriter, *http.Request){}
	}
	registeredUIFuncHandler[pattern] = handler
	uiMutex.Unlock()
}

// HandleUI register ui request handler
func HandleUI(pattern string, handler http.Handler) {

	uiMutex.Lock()
	if registeredUIHandler == nil {
		registeredUIHandler = map[string]http.Handler{}
	}
	registeredUIHandler[pattern] = handler
	uiMutex.Unlock()
}

// HandleUIMethod register ui request handler
func HandleUIMethod(method Method, pattern string, handler func(w http.ResponseWriter, req *http.Request, ps httprouter.Params)) {
	uiMutex.Lock()
	if registeredUIMethodHandler == nil {
		registeredUIMethodHandler = map[string]map[string]func(w http.ResponseWriter, req *http.Request, ps httprouter.Params){}
	}

	m := registeredUIMethodHandler[string(method)]
	if m == nil {
		registeredUIMethodHandler[string(method)] = map[string]func(w http.ResponseWriter, req *http.Request, ps httprouter.Params){}
	}
	registeredUIMethodHandler[string(method)][pattern] = handler
	uiMutex.Unlock()
}

// HandleWebSocketCommand register websocket command handler
func HandleWebSocketCommand(command string, usage string, handler func(c *websocket.WebsocketConnection, array []string)) {

	uiMutex.Lock()
	if registeredWebSocketCommandHandler == nil {
		registeredWebSocketCommandHandler = map[string]func(c *websocket.WebsocketConnection, array []string){}
		webSocketCommandUsage = map[string]string{}
	}

	command = strings.ToLower(strings.TrimSpace(command))
	registeredWebSocketCommandHandler[command] = handler
	webSocketCommandUsage[command] = usage
	uiMutex.Unlock()
}
