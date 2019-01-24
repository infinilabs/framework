package api

import (
	"crypto/tls"
	"errors"
	log "github.com/cihub/seelog"
	"github.com/gorilla/context"
	"github.com/infinitbyte/framework/core/api/router"
	"github.com/infinitbyte/framework/core/global"
	"github.com/infinitbyte/framework/core/util"
	"github.com/rs/cors"
	"golang.org/x/net/http2"
	"net"
	"net/http"
	"path"
	"path/filepath"
	"sync"
	"time"
)

// RegisteredAPIHandler is a hub for registered api
var registeredAPIHandler = make(map[string]http.Handler)

// RegisteredAPIFuncHandler is a hub for registered api
var registeredAPIFuncHandler = make(map[string]func(http.ResponseWriter, *http.Request))

// RegisteredAPIMethodHandler is a hub for registered api
var registeredAPIMethodHandler = make(map[string]map[string]func(w http.ResponseWriter, req *http.Request, ps httprouter.Params))

var l sync.Mutex
var started bool

// HandleAPIFunc register api handler to specify pattern
func HandleAPIFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	l.Lock()
	if registeredAPIFuncHandler == nil {
		registeredAPIFuncHandler = map[string]func(http.ResponseWriter, *http.Request){}
	}
	registeredAPIFuncHandler[pattern] = handler

	log.Debugf("register custom http handler: %v", pattern)
	mux.HandleFunc(pattern, handler)

	l.Unlock()
}

// HandleAPI register api handler
func HandleAPI(pattern string, handler http.Handler) {

	l.Lock()
	if registeredAPIHandler == nil {
		registeredAPIHandler = map[string]http.Handler{}
	}
	registeredAPIHandler[pattern] = handler
	log.Debugf("register custom http handler: %v", pattern)

	mux.Handle(pattern, handler)

	l.Unlock()
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
	log.Debugf("register custom http handler: %v %v", m, pattern)

	router.Handle(m, pattern, handler)
	l.Unlock()
}

var router *httprouter.Router = httprouter.New(mux)
var mux *http.ServeMux = http.NewServeMux()

// StartAPI will start listen and act as the API server
func StartAPI() {

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "DELETE", "PUT", "OPTIONS"},
	})

	address := util.AutoGetAddress(global.Env().SystemConfig.NetworkConfig.APIBinding)
	global.Env().SystemConfig.NetworkConfig.APIBinding = address

	l, err := net.Listen("tcp", address)

	if err != nil {
		panic(err)
	}

	if global.Env().SystemConfig.TLSEnabled {
		log.Debug("tls enabled")

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
			Handler:      c.Handler(context.ClearHandler(router)),
			TLSConfig:    cfg,
			TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
		}

		http2.ConfigureServer(srv, &http2.Server{})

		go func() {
			err = srv.ServeTLS(l, certFile, keyFile)
			if err != nil {
				log.Error(err)
				panic(err)
			}
		}()

	} else {

		go func() {
			err := http.Serve(l, c.Handler(context.ClearHandler(router)))
			if err != nil {
				log.Error(err)
				panic(err)
			}
		}()
	}

	err = util.WaitServerUp(address, 30*time.Second)
	if err != nil {
		panic(err)
	}
	log.Info("api server listen at: ", address)
}
