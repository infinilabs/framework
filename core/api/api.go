package api

import (
	"crypto/tls"
	"errors"
	log "github.com/cihub/seelog"
	"github.com/gorilla/context"
	"github.com/infinitbyte/framework/core/api/router"
	"github.com/infinitbyte/framework/core/global"
	"github.com/infinitbyte/framework/core/util"
	"net/http"
	"path"
	"path/filepath"
	"sync"
)

// RegisteredAPIHandler is a hub for registered api
var registeredAPIHandler map[string]http.Handler

// RegisteredAPIFuncHandler is a hub for registered api
var registeredAPIFuncHandler map[string]func(http.ResponseWriter, *http.Request)

// RegisteredAPIMethodHandler is a hub for registered api
var registeredAPIMethodHandler map[string]map[string]func(w http.ResponseWriter, req *http.Request, ps httprouter.Params)

var l sync.Mutex

// HandleAPIFunc register api handler to specify pattern
func HandleAPIFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	l.Lock()
	if registeredAPIFuncHandler == nil {
		registeredAPIFuncHandler = map[string]func(http.ResponseWriter, *http.Request){}
	}
	registeredAPIFuncHandler[pattern] = handler
	l.Unlock()
}

// HandleAPI register api handler
func HandleAPI(pattern string, handler http.Handler) {

	l.Lock()
	if registeredAPIHandler == nil {
		registeredAPIHandler = map[string]http.Handler{}
	}
	registeredAPIHandler[pattern] = handler
	l.Unlock()
}

// HandleAPIMethod register api handler
func HandleAPIMethod(method Method, pattern string, handler func(w http.ResponseWriter, req *http.Request, ps httprouter.Params)) {
	l.Lock()
	if registeredAPIMethodHandler == nil {
		registeredAPIMethodHandler = map[string]map[string]func(w http.ResponseWriter, req *http.Request, ps httprouter.Params){}
	}

	m := registeredAPIMethodHandler[string(method)]
	if m == nil {
		registeredAPIMethodHandler[string(method)] = map[string]func(w http.ResponseWriter, req *http.Request, ps httprouter.Params){}
	}
	registeredAPIMethodHandler[string(method)][pattern] = handler
	l.Unlock()
}

var router *httprouter.Router
var mux *http.ServeMux

// StartAPI will start listen and act as the API server
func StartAPI() {

	mux = http.NewServeMux()
	router = httprouter.New(mux)

	//registered handlers
	if registeredAPIHandler != nil {
		for k, v := range registeredAPIHandler {
			log.Debugf("register custom http handler: %v", k)
			mux.Handle(k, v)
		}
	}
	if registeredAPIFuncHandler != nil {
		for k, v := range registeredAPIFuncHandler {
			log.Debugf("register custom http handler: %v", k)
			mux.HandleFunc(k, v)
		}
	}
	if registeredAPIMethodHandler != nil {
		for k, v := range registeredAPIMethodHandler {
			for m, n := range v {
				log.Debugf("register custom http handler: %v %v", k, m)
				router.Handle(k, m, n)
			}
		}
	}

	address := util.AutoGetAddress(global.Env().SystemConfig.APIBinding)

	if global.Env().SystemConfig.TLSEnabled {
		log.Debug("start ssl endpoint")

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

		srv := &http.Server{
			Addr:         address,
			Handler:      context.ClearHandler(router),
			TLSConfig:    cfg,
			TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
		}

		log.Info("api server listen at: https://", address)
		err = srv.ListenAndServeTLS(certFile, keyFile)
		if err != nil {
			log.Error(err)
			panic(err)
		}

	} else {
		log.Info("api server listen at: http://", address)
		err := http.ListenAndServe(address, context.ClearHandler(router))
		if err != nil {
			log.Error(err)
			panic(err)
		}
	}

}
