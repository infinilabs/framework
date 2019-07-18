package api

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	log "github.com/cihub/seelog"
	"github.com/gorilla/context"
	"github.com/infinitbyte/framework/core/api/router"
	"github.com/infinitbyte/framework/core/config"
	"github.com/infinitbyte/framework/core/env"
	"github.com/infinitbyte/framework/core/global"
	"github.com/infinitbyte/framework/core/util"
	"github.com/rs/cors"
	"golang.org/x/net/http2"
	"net"
	"net/http"
	"os"
	"path"
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

var connected bool
var lastContact *time.Time
var certPool *x509.CertPool
var rootCert *x509.Certificate
var rootKey *rsa.PrivateKey
var rootCertPEM []byte

var apiConfig *APIConfig

var listenAddress string

// StartAPI will start listen and act as the API server
func StartAPI(cfg *config.Config) {

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"HEAD", "GET", "POST", "DELETE", "PUT", "OPTIONS"},
	})

	apiConfig = &APIConfig{}

	env.ParseConfig("api", apiConfig)

	if !apiConfig.Enabled {
		return
	}

	listenAddress = util.AutoGetAddress(apiConfig.NetworkConfig.GetBindingAddr())
	l, err := net.Listen("tcp", listenAddress)

	if err != nil {
		panic(err)
	}

	if apiConfig.TLSConfig.TLSEnabled {

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

		var cert, key string
		log.Trace("using tls connection")

		//var creds credentials.TransportCredentials
		if cert != "" && key != "" {
			log.Debug("using pre-defined cert files")

		} else {
			log.Info("auto generate server cert")
			rootCert, rootKey, rootCertPEM = util.GetRootCert()

			certPool = x509.NewCertPool()
			certPool.AppendCertsFromPEM(rootCertPEM)

			// create a key-pair for the server
			servKey, err := rsa.GenerateKey(rand.Reader, 2048)
			if err != nil {
				panic(err)
			}

			// create a template for the server
			servCertTmpl, err := util.GetCertTemplate()
			if err != nil {
				panic(err)
			}

			servCertTmpl.KeyUsage = x509.KeyUsageDigitalSignature
			servCertTmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}

			// create a certificate which wraps the server's public key, sign it with the root private key
			_, servCertPEM, err := util.CreateCert(servCertTmpl, rootCert, &servKey.PublicKey, rootKey)
			if err != nil {
				panic(err)
			}

			// provide the private key and the cert
			servKeyPEM := pem.EncodeToMemory(&pem.Block{
				Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(servKey),
			})

			os.MkdirAll(path.Join(global.Env().GetWorkingDir(), "certs"), 0775)
			rootCert := path.Join(global.Env().GetWorkingDir(), "certs", "root.cert")
			cert = path.Join(global.Env().GetWorkingDir(), "certs", "auto.cert")
			key = path.Join(global.Env().GetWorkingDir(), "certs", "auto.key")

			util.FileAppendContentWithByte(rootCert, rootCertPEM)
			util.FileAppendContentWithByte(cert, servCertPEM)
			util.FileAppendContentWithByte(key, servKeyPEM)
		}

		srv := &http.Server{
			Addr:         listenAddress,
			Handler:      c.Handler(context.ClearHandler(router)),
			TLSConfig:    cfg,
			TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
		}

		http2.ConfigureServer(srv, &http2.Server{})

		go func() {
			err = srv.ServeTLS(l, cert, key)
			if err != nil {
				log.Error(err)
				panic(err)
			}
		}()

	} else {
		log.Trace("starting insecure API server")
		go func() {
			err := http.Serve(l, c.Handler(context.ClearHandler(router)))
			if err != nil {
				log.Error(err)
				panic(err)
			}
		}()
	}

	err = util.WaitServerUp(listenAddress, 30*time.Second)
	if err != nil {
		panic(err)
	}
	log.Info("api server listen at: ", listenAddress)
}
