/*
Copyright 2016 Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package api

import (
	"crypto/tls"
	"errors"
	log "github.com/cihub/seelog"
	"github.com/gorilla/context"
	ctx "context"
	"infini.sh/framework/core/api/gzip"
	"infini.sh/framework/core/api/router"
	"infini.sh/framework/core/api/websocket"
	"infini.sh/framework/core/global"
	_ "infini.sh/framework/core/log"
	"infini.sh/framework/core/util"
	"net/http"
	_ "net/http/pprof"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var uiRouter *httprouter.Router
var uiServeMux *http.ServeMux
var uiMutex sync.Mutex
var uiConfig *UIConfig

func GetUIConfig() UIConfig {
	return *uiConfig
}

var bindAddress string

func GetBindAddress() string {
	return bindAddress
}

func StopUI(cfg *UIConfig) {
	if srv!=nil{
		ctx1, cancel := ctx.WithTimeout(ctx.Background(), 10*time.Second)
		defer cancel()
		err:=srv.Shutdown(ctx1)
		if err!=nil{
			panic(err)
		}

		log.Debug("stopping UI server")
	}
}

func StartUI(cfg *UIConfig) {
	uiConfig = cfg
	//start web ui
	uiServeMux = http.NewServeMux()

	uiRouter = httprouter.New(uiServeMux)
	//uiRouter.RedirectTrailingSlash=false
	//uiRouter.RedirectFixedPath=false

	uiRouter.NotFound= http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Write([]byte("my 404"))
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

	if cfg.EmbeddingAPI{
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



	//init websocket,TODO configurable
	websocket.InitWebSocket()
	uiServeMux.HandleFunc("/ws", websocket.ServeWs)

	if registeredWebSocketCommandHandler != nil {
		for k, v := range registeredWebSocketCommandHandler {
			log.Debug("register websocket handler: ", k, " ", v)
			websocket.HandleWebSocketCommand(k, webSocketCommandUsage[k], v)
		}
	}

	schema := "http://"

	if uiConfig.NetworkConfig.SkipOccupiedPort {
		bindAddress = util.AutoGetAddress(uiConfig.NetworkConfig.GetBindingAddr())
	} else {
		bindAddress = uiConfig.NetworkConfig.GetBindingAddr()
	}

	handler := context.ClearHandler(uiRouter)
	if uiConfig.Gzip.Enabled {
		log.Debug("gzip enabled")
		wrapper, _ := gzip.NewGzipLevelHandler(uiConfig.Gzip.Level)
		handler = wrapper(handler)
	}
	if uiConfig.TLSConfig.TLSEnabled {
		log.Debug("tls enabled")

		schema = "https://"

		certFile := path.Join(global.Env().SystemConfig.PathConfig.Cert, "*c*rt*")
		match, err := filepath.Glob(certFile)
		if err != nil {
			panic(err)
		}
		if len(match) <= 0 {
			panic(errors.New("no cert file found, the file name must end with .crt"))
		}
		certFile = match[0]

		keyFile := path.Join(global.Env().SystemConfig.PathConfig.Cert, "*key*")
		match, err = filepath.Glob(keyFile)
		if err != nil {
			panic(err)
		}
		if len(match) <= 0 {
			panic(errors.New("no key file found, the file name must end with .key"))
		}
		keyFile = match[0]

		cfg := &tls.Config{
			MinVersion:               tls.VersionTLS12,
			CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
			PreferServerCipherSuites: true,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			},
		}


		srv = &http.Server{
			Addr:         bindAddress,
			Handler:      RecoveryHandler()(handler),
			TLSConfig:    cfg,
			TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
		}

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
						log.Error("error", v)
					}
				}
			}()

			err = srv.ListenAndServeTLS(certFile, keyFile)
			if err != nil&& err != http.ErrServerClosed {
				log.Error(err)
				panic(err)
			}
		}(srv)

	} else {
		srv= &http.Server{Addr: bindAddress, Handler: RecoveryHandler()(handler)}
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
						log.Error("error", v)
					}
				}
			}()

			err :=  srv.ListenAndServe()
			if err != nil&& err != http.ErrServerClosed {
				log.Error(err)
				panic(err)
			}
		}(srv)

	}

	err := util.WaitServerUp(bindAddress, 30*time.Second)
	if err != nil {
		panic(err)
	}

	log.Info("ui listen at: ", schema, bindAddress)

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
