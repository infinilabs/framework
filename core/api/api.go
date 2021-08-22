package api

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	log "github.com/cihub/seelog"
	"github.com/gorilla/context"
	"github.com/rs/cors"
	"golang.org/x/net/http2"
	"infini.sh/framework/core/api/filter"
	"infini.sh/framework/core/api/router"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
	"net"
	"net/http"
	"os"
	"path"
	"sync"
	"time"
)

// RegisteredAPIFuncHandler is a hub for registered api
var registeredAPIFuncHandler = make(map[string]func(http.ResponseWriter, *http.Request))

// RegisteredAPIMethodHandler is a hub for registered api
var registeredAPIMethodHandler = make(map[string]map[string]func(w http.ResponseWriter, req *http.Request, ps httprouter.Params))

var l sync.Mutex

var filters []filter.Filter

var APIs = map[string]string{}

// HandleAPIFunc register api handler to specify pattern
func HandleAPIFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	l.Lock()
	if registeredAPIFuncHandler == nil {
		registeredAPIFuncHandler = map[string]func(http.ResponseWriter, *http.Request){}
	}

	for _, f := range filters {
		handler = f.FilterHttpHandlerFunc(pattern, handler)
	}

	registeredAPIFuncHandler[pattern] = handler
	APIs[pattern]="*"

	log.Debugf("register custom http handler: %v", pattern)
	mux.HandleFunc(pattern, handler)

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

	//Apply handler filters
	for _, f := range filters {
		handler = f.FilterHttpRouter(pattern, handler)
	}

	registeredAPIMethodHandler[m][pattern] = handler
	APIs[pattern]=m

	log.Debugf("register custom http handler: %v %v", m, pattern)

	router.Handle(m, pattern, handler)
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

var notfoundHandler =http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
	rw.Write([]byte("{\"message\":\"not_found\"}"))
	rw.WriteHeader(404)
})

func SetNotFoundHandler(handler func(rw http.ResponseWriter, r *http.Request))  {
	notfoundHandler=handler
}

// StartAPI will start listen and act as the API server
func StartAPI() {

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"HEAD", "GET", "POST", "DELETE", "PUT", "OPTIONS"},
	})

	apiConfig = &global.Env().SystemConfig.APIConfig

	if !apiConfig.Enabled {
		return
	}

	if apiConfig.NetworkConfig.SkipOccupiedPort {
		listenAddress = util.AutoGetAddress(apiConfig.NetworkConfig.GetBindingAddr())
	} else {
		listenAddress = apiConfig.NetworkConfig.GetBindingAddr()
	}

	l, err := net.Listen("tcp", listenAddress)

	if err != nil {
		panic(err)
	}

	router.NotFound=notfoundHandler

	schema := "http://"
	if apiConfig.TLSConfig.TLSEnabled {
		schema = "https://"
		cfg := &tls.Config{
			MinVersion: tls.VersionTLS12,
			CurvePreferences: []tls.CurveID{
				tls.CurveP256,
				tls.X25519, // Go 1.8 only
			},
			PreferServerCipherSuites: true,
			InsecureSkipVerify:       true,
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

		var ca, cert, key string
		log.Trace("using tls connection")

		//var creds credentials.TransportCredentials
		if cert != "" && key != "" {
			log.Debug("using pre-defined cert files")

		} else {
			ca = path.Join(global.Env().GetDataDir(), "certs", "root.cert")
			cert = path.Join(global.Env().GetDataDir(), "certs", "auto.cert")
			key = path.Join(global.Env().GetDataDir(), "certs", "auto.key")

			if !(util.FileExists(ca) && util.FileExists(cert) && util.FileExists(key)) {

				os.MkdirAll(path.Join(global.Env().GetDataDir(), "certs"), 0775)

				log.Info("auto generating cert files")
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

				util.FilePutContentWithByte(ca, rootCertPEM)
				util.FilePutContentWithByte(cert, servCertPEM)
				util.FilePutContentWithByte(key, servKeyPEM)
			} else {
				log.Debug("loading auto generated certs")
			}
		}

		srv := &http.Server{
			ReadTimeout:       5 * time.Second,
			WriteTimeout:      10 * time.Second,
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       10 * time.Second,
			Addr:              listenAddress,
			Handler:           c.Handler(context.ClearHandler(router)),
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
	log.Info("api listen at: ", schema, listenAddress)
}
