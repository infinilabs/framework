/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/gorilla/context"
	"github.com/rs/cors"
	"golang.org/x/net/http2"
	"infini.sh/framework/core/api/filter"
	"infini.sh/framework/core/api/router"
	"infini.sh/framework/core/api/websocket"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	_ "infini.sh/framework/core/logging"
	"infini.sh/framework/core/logging/logger"
	"infini.sh/framework/core/util"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"runtime"
	"sync"
	"time"
)

// RegisteredAPIFuncHandler is a hub for registered api
var registeredAPIFuncHandler = make(map[string]func(http.ResponseWriter, *http.Request))

// RegisteredAPIMethodHandler is a hub for registered api
var registeredAPIMethodHandler = make(map[string]map[string]func(w http.ResponseWriter, req *http.Request, ps httprouter.Params))

var l sync.Mutex

var filters []filter.Filter

func RegisterAPIFilter(f filter.Filter) {
	l.Lock()
	defer l.Unlock()
	filters = append(filters, f)
}

var APIs = map[string]util.KV{}

var authEnabled = false

func EnableAuth(enable bool) {
	authEnabled = enable
}

func IsAuthEnable() bool {
	return authEnabled //TODO moved to global registered variable
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
		HandleAPIFunc("/ws", websocket.ServeWs)
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

	if util.ContainStr(listenAddress, "0.0.0.0") {
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

	schema := "http://"
	if apiConfig.TLSConfig.TLSEnabled {
		schema = "https://"

		log.Trace("using tls connection")

		cfg := &tls.Config{
			MinVersion: tls.VersionTLS12,
			CurvePreferences: []tls.CurveID{
				tls.CurveP256,
				tls.X25519, // Go 1.8 only
			},
			PreferServerCipherSuites: true,
			InsecureSkipVerify:       apiConfig.TLSConfig.TLSInsecureSkipVerify,
			SessionTicketsDisabled:   false,
			ClientSessionCache:       tls.NewLRUClientSessionCache(128),
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305, // Go 1.8 only
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,   // Go 1.8 only
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
			NextProtos: []string{"spdy/3"},
		}
		if apiConfig.TLSConfig.TLSCertFile == "" && apiConfig.TLSConfig.TLSKeyFile == "" {
			dataDir := global.Env().GetDataDir()
			apiConfig.TLSConfig.TLSCertFile = path.Join(dataDir, "certs/instance.crt")
			apiConfig.TLSConfig.TLSKeyFile = path.Join(dataDir, "certs/instance.key")
			apiConfig.TLSConfig.TLSCACertFile = path.Join(dataDir, "certs/ca.crt")
			caKey := path.Join(dataDir, "certs/ca.key")
			if !(util.FileExists(apiConfig.TLSConfig.TLSCACertFile) && util.FileExists(apiConfig.TLSConfig.TLSCertFile) && util.FileExists(apiConfig.TLSConfig.TLSKeyFile)) {
				err = os.MkdirAll(path.Join(dataDir, "certs"), 0775)
				if err != nil {
					panic(err)
				}
				log.Info("auto generating cert files")
				rootCert, rootKey, rootCertPEM = util.GetRootCert()
				if apiConfig.TLSConfig.DefaultDomain == "" {
					apiConfig.TLSConfig.DefaultDomain = "localhost"
				}
				instanceCertPEM, instanceKeyPEM, err := util.GenerateServerCert(rootCert, rootKey, rootCertPEM, []string{apiConfig.TLSConfig.DefaultDomain})
				if err != nil {
					panic(err)
				}
				caKeyPEM := pem.EncodeToMemory(&pem.Block{
					Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rootKey),
				})
				util.FilePutContentWithByte(caKey, caKeyPEM)
				util.FilePutContentWithByte(apiConfig.TLSConfig.TLSCACertFile, rootCertPEM)
				util.FilePutContentWithByte(apiConfig.TLSConfig.TLSCertFile, instanceCertPEM)
				util.FilePutContentWithByte(apiConfig.TLSConfig.TLSKeyFile, instanceKeyPEM)
			}
		}

		if !apiConfig.TLSConfig.TLSInsecureSkipVerify {
			if certPool == nil {
				certPool = x509.NewCertPool()
			}
			if len(rootCertPEM) == 0 {
				rootCertPEM, err = ioutil.ReadFile(apiConfig.TLSConfig.TLSCACertFile)
				if err != nil {
					panic(err)
				}
			}
			certPool.AppendCertsFromPEM(rootCertPEM)
			cfg.ClientCAs = certPool
			cfg.ClientAuth = tls.RequireAndVerifyClientCert
		}

		srv := &http.Server{
			ReadTimeout:       5 * time.Second,
			WriteTimeout:      10 * time.Second,
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       10 * time.Second,
			Addr:              listenAddress,
			Handler:           RecoveryHandler()(c.Handler(context.ClearHandler(router))),
			TLSConfig:         cfg,
			TLSNextProto: map[string]func(*http.Server, *tls.Conn, http.Handler){
				"spdy/3": func(s *http.Server, conn *tls.Conn, h http.Handler) {
					buf := make([]byte, 1)
					if n, err := conn.Read(buf); err != nil {
						log.Error("%v|%v\n", n, err)
					}
				},
			},
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

			err = srv.ServeTLS(l, apiConfig.TLSConfig.TLSCertFile, apiConfig.TLSConfig.TLSKeyFile)
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

			err := http.Serve(l, RecoveryHandler()(c.Handler(context.ClearHandler(router))))
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

	log.Info("api listen at: ", schema, listenAddress)

}
