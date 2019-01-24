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
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/infinitbyte/framework/core/env"
	"github.com/infinitbyte/framework/core/global"
	"github.com/infinitbyte/framework/core/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	_ "google.golang.org/grpc/encoding/gzip" // Install the gzip compressor
	"google.golang.org/grpc/reflection"
	"net"
	"time"
)

func GetRPCServer() *grpc.Server {
	return s
}

//only select local connection
func ObtainLocalConnection() (conn *grpc.ClientConn, err error) {
	return ObtainConnection(global.Env().SystemConfig.NetworkConfig.RPCBinding)
}

//func ObtainLeaderConnection() (conn *grpc.ClientConn, err error) {
////	return ObtainConnection(leaderAddr)
////}

//var addr = flag.String("rpc.bind", "localhost:20000", "the rpc address to bind to")

//auto select connection
func ObtainConnection(addr string) (conn *grpc.ClientConn, err error) {
	log.Trace("obtain client connection: ", addr)

	if c.TLSEnabled {
		var creds credentials.TransportCredentials
		log.Trace("using tls connection")

		cert := c.TLSCertFile
		key := c.TLSKeyFile
		if cert != "" && key != "" {
			log.Trace("use pre-defined cert")

			// Create tls based credential.
			creds, err = credentials.NewClientTLSFromFile(cert, key)
			if err != nil {
				log.Errorf("failed to load credentials: %v", err)
			}

			// Set up a connection to the server.
			conn, err = grpc.Dial(addr, grpc.WithTransportCredentials(creds))
			if err != nil {
				log.Errorf("did not connect: %v", err)
			}

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
		conn, err = grpc.Dial(addr, dialOption)
		if err != nil {
			log.Errorf("cannot connect to %s, %v", addr, err)
		}

	} else {
		log.Trace("using insecure tcp connection")
		conn, err = grpc.Dial(addr, grpc.WithInsecure())
		if err != nil {
			log.Errorf("cannot connect to %s, %v", addr, err)
		}
	}
	return conn, err
}

var s *grpc.Server
var c *RPCConfig
var connected bool
var lastContact *time.Time
var certPool *x509.CertPool
var rootCert *x509.Certificate
var rootKey *rsa.PrivateKey
var rootCertPEM []byte

func Setup() {

	s = grpc.NewServer()

	//get config
	c = &RPCConfig{}

	exist, err := env.ParseConfig("rpc", c)

	if exist && err != nil {
		panic(err)
	}
	if err != nil {
		log.Errorf("error to process config: %v", err)
	}

	if c.TLSEnabled {
		cert := c.TLSCertFile
		key := c.TLSKeyFile
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

			fmt.Printf("ca\n")
			fmt.Printf("%s\n", rootCertPEM)

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

			if c.TLSInsecureSkipVerify {

				// Create the TLS credentials
				creds = credentials.NewTLS(&tls.Config{
					Certificates:       []tls.Certificate{servTLSCert},
					ClientAuth:         tls.RequireAnyClientCert,
					ClientCAs:          certPool,
					InsecureSkipVerify: true,
				})
			}
		}

		s = grpc.NewServer(grpc.Creds(creds))
	} else {
		log.Trace("using insecure tcp connection")
		s = grpc.NewServer()
	}

}

func StartRPCServer() {

	global.Env().SystemConfig.NetworkConfig.RPCBinding = util.AutoGetAddress(global.Env().SystemConfig.NetworkConfig.RPCBinding)
	address := global.Env().SystemConfig.NetworkConfig.RPCBinding

	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Errorf("failed to listen: %v", err)
	}

	// Register reflection service on gRPC server.
	reflection.Register(s)

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Errorf("failed to serve: %v", err)
		}
	}()

	err = util.WaitServerUp(address, 30*time.Second)
	if err != nil {
		panic(err)
	}
	log.Info("rpc server listen at: ", address)
}
