/*
Copyright Medcl (m AT medcl.net)

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

package rpc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	log "github.com/cihub/seelog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	_ "google.golang.org/grpc/encoding/gzip" // Install the gzip compressor
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
	"net"
	"runtime"
	"time"
)

func GetRPCServer() *grpc.Server {
	return s
}

//only select local connection
func ObtainLocalConnection() (conn *ClientConn, err error) {
	return ObtainConnection(listenAddress)
}

//func ObtainLeaderConnection() (conn *grpc.ClientConn, err error) {
////	return ObtainConnection(leaderAddr)
////}

//var addr = flag.String("rpc.bind", "localhost:20000", "the rpc address to bind to")

//auto select connection
func ObtainConnection(addr string) (client *ClientConn, err error) {
	log.Trace("obtain client connection: ", addr)

	if rpcConfig.TLSConfig.TLSEnabled {
		var creds credentials.TransportCredentials
		log.Trace("using tls connection")

		cert := rpcConfig.TLSConfig.TLSCertFile
		key := rpcConfig.TLSConfig.TLSKeyFile

		if cert != "" && key != "" {
			log.Trace("use pre-defined cert")

			// Create tls based credential.
			creds, err = credentials.NewClientTLSFromFile(cert, key)
			if err != nil {
				log.Errorf("failed to load credentials: %v", err)
			}

			// Set up a connection to the server.
			//conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds))
			//if err != nil {
			//	log.Errorf("did not connect: %v", err)
			//}

			// Load the client certificates from disk
			certificate, err := tls.LoadX509KeyPair(cert, key)
			if err != nil {
				panic(err)
			}

			creds = credentials.NewTLS(&tls.Config{
				Certificates:       []tls.Certificate{certificate},
				RootCAs:            certPool,
				InsecureSkipVerify: true,
			})

		} else {
			log.Debug("auto generate server cert")

			clientTLSCert, _, _ := util.GetClientCert(rootCert, rootKey)

			// Create the TLS credentials
			creds = credentials.NewTLS(&tls.Config{
				Certificates:       []tls.Certificate{clientTLSCert},
				RootCAs:            certPool,
				InsecureSkipVerify: true,
			})
		}

		dialOption := grpc.WithTransportCredentials(creds)

		p, err := New(func() (*grpc.ClientConn, error) {
			return grpc.Dial(addr, dialOption)
		}, 1, 2, 0)

		// Get a client
		client, err := p.Get(context.Background())
		if err != nil {
			log.Errorf("Get returned an error: %s", err.Error())
		}
		if client == nil {
			log.Error("client was nil")
		}
		return client, nil

		//conn, err = grpc.Dial(addr, dialOption)
		//if err != nil {
		//	log.Errorf("cannot connect to %s, %v", addr, err)
		//}

	} else {
		log.Trace("using insecure tcp connection")

		p, err := New(func() (*grpc.ClientConn, error) {
			return grpc.Dial(addr, grpc.WithInsecure())
		}, 1, 2, 0)

		// Get a client
		client, err := p.Get(context.Background())
		if err != nil {
			log.Errorf("Get returned an error: %s", err.Error())
		}
		if client == nil {
			log.Error("client was nil")
		}

		return client, err

		//conn, err = grpc.Dial(addr, grpc.WithInsecure())
		//if err != nil {
		//	log.Errorf("cannot connect to %s, %v", addr, err)
		//}

	}
}

var s *grpc.Server
var rpcConfig *config.RPCConfig
var connected bool
var lastContact *time.Time
var certPool *x509.CertPool
var rootCert *x509.Certificate
var rootKey *rsa.PrivateKey
var rootCertPEM []byte

func Setup(cfg *config.RPCConfig) {
	rpcConfig = cfg
	var err error
	if rpcConfig.TLSConfig.TLSEnabled {
		cert := rpcConfig.TLSConfig.TLSCertFile
		key := rpcConfig.TLSConfig.TLSKeyFile
		log.Trace("using tls connection")

		var creds credentials.TransportCredentials
		if cert != "" && key != "" {
			log.Debug("use pre-defined cert")
			creds, err = credentials.NewServerTLSFromFile(cert, key)
			if err != nil {
				log.Errorf("failed to create credentials: %v", err)
				panic(err)
			}
		} else {
			log.Debug("auto generate server cert")
			rootCert, rootKey, rootCertPEM = util.GetRootCert()

			certPool = x509.NewCertPool()
			certPool.AppendCertsFromPEM(rootCertPEM)

			//fmt.Printf("ca\n")
			//fmt.Printf("%s\n", rootCertPEM)

			// create a key-pair for the server
			servKey, err := rsa.GenerateKey(rand.Reader, 2048)
			if err != nil {
				panic(err)
			}

			// create a template for the server
			servCertTmpl, err := util.GetCertTemplateWithSingleDomain(rpcConfig.TLSConfig.DefaultDomain)
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

			//fmt.Printf("server_cert\n")
			//fmt.Printf("%s\n", servCertPEM)
			//fmt.Printf("server_key\n")
			//fmt.Printf("%s\n", servKeyPEM)

			servTLSCert, err := tls.X509KeyPair(servCertPEM, servKeyPEM)
			if err != nil {
				log.Errorf("invalid key pair: %v", err)
			}

			// Create the TLS credentials
			creds = credentials.NewTLS(&tls.Config{
				Certificates:       []tls.Certificate{servTLSCert},
				ClientAuth:         tls.RequireAndVerifyClientCert,
				ClientCAs:          certPool,
				InsecureSkipVerify: false,
			})

			if rpcConfig.TLSConfig.TLSInsecureSkipVerify {

				// Create the TLS credentials
				creds = credentials.NewTLS(&tls.Config{
					Certificates:       []tls.Certificate{servTLSCert},
					ClientAuth:         tls.RequireAnyClientCert,
					ClientCAs:          certPool,
					InsecureSkipVerify: true,
				})
			}
		}

		s = grpc.NewServer(grpc.Creds(creds), grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: 5 * time.Minute}))
	} else {
		log.Trace("using insecure tcp connection")
		s = grpc.NewServer(grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: 5 * time.Minute}))
	}

}

var listener net.Listener
var listenAddress string

func GetListener() net.Listener {
	return listener
}

func GetRPCAddress() string {
	return listenAddress
}

func StartRPCServer() {

	if rpcConfig.NetworkConfig.SkipOccupiedPort {
		listenAddress = util.AutoGetAddress(rpcConfig.NetworkConfig.GetBindingAddr())
	} else {
		listenAddress = rpcConfig.NetworkConfig.GetBindingAddr()
	}

	var err error
	listener, err = net.Listen("tcp", listenAddress)
	if err != nil {
		log.Errorf("failed to listen: %v", err)
	}

	// Register reflection service on gRPC server.
	reflection.Register(s)

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

		if err := s.Serve(listener); err != nil {
			log.Errorf("failed to serve: %v", err)
		}
	}()

	err = util.WaitServerUp(listenAddress, 30*time.Second)
	if err != nil {
		panic(err)
	}
	log.Info("rpc server listen at: ", listenAddress)
}
