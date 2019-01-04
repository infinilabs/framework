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

package util

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/stretchr/testify/assert"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"testing"
)

func TestGetCert(t *testing.T) {
	rootCert, rootKey, rootCertPEM := GetRootCert()

	ok := func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("HI!")) }

	// create a key-pair for the server
	servKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Errorf("generating random key: %v", err)
	}

	// create a template for the server
	servCertTmpl, err := GetCertTemplate()
	if err != nil {
		log.Errorf("creating cert template: %v", err)
	}
	servCertTmpl.KeyUsage = x509.KeyUsageDigitalSignature
	servCertTmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	servCertTmpl.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}

	// create a certificate which wraps the server's public key, sign it with the root private key
	_, servCertPEM, err := CreateCert(servCertTmpl, rootCert, &servKey.PublicKey, rootKey)
	if err != nil {
		log.Errorf("error creating cert: %v", err)
	}

	// provide the private key and the cert
	servKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(servKey),
	})
	servTLSCert, err := tls.X509KeyPair(servCertPEM, servKeyPEM)
	if err != nil {
		log.Errorf("invalid key pair: %v", err)
	}

	// create a pool of trusted certs
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(rootCertPEM)

	clientTLSCert, _, _ := GetClientCert(rootCert, rootKey)

	authedClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:      certPool,
				Certificates: []tls.Certificate{clientTLSCert},
			},
		},
	}

	s := httptest.NewUnstartedServer(http.HandlerFunc(ok))
	s.TLS = &tls.Config{
		Certificates: []tls.Certificate{servTLSCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
	}

	s.StartTLS()

	resp, err := authedClient.Get(s.URL)
	s.Close()
	assert.Equal(t, nil, err)

	dump, err := httputil.DumpResponse(resp, true)
	if err != nil {
		log.Errorf("could not dump response: %v", err)
	}
	fmt.Printf("%s\n", dump)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 3, int(resp.ContentLength))

	//// Load the certificates from disk
	//certificate, err := tls.LoadX509KeyPair(crt, key)
	//if err != nil {
	//	log.Errorf("could not load server key pair: %s", err)
	//	panic(err)
	//}
	//
	//// Create a certificate pool from the certificate authority
	//certPool := x509.NewCertPool()
	//ca, err := ioutil.ReadFile(ca)
	//if err != nil {
	//	log.Errorf("could not read ca certificate: %s", err)
	//	panic(err)
	//}
	//
	//// Append the client certificates from the CA
	//if ok := certPool.AppendCertsFromPEM(ca); !ok {
	//	log.Errorf("failed to append client certs")
	//	panic(err)
	//}
}
