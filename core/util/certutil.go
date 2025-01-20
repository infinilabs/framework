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

// https://ericchiang.github.io/post/go-tls/
package util

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	log "github.com/cihub/seelog"
	"math/big"
	"time"
)

func GetRootCert() (rootCert *x509.Certificate, rootKey *rsa.PrivateKey, rootCertPEM []byte) {
	log.Trace("generate root cert")

	var err error
	// generate a new key-pair
	rootKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Errorf("generating random key: %v", err)
	}

	rootCertTmpl, err := GetCertTemplate(nil)
	if err != nil {
		log.Errorf("creating cert template: %v", err)
	}
	// describe what the certificate will be used for
	rootCertTmpl.IsCA = true
	rootCertTmpl.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature
	rootCertTmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
	//rootCertTmpl.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}

	rootCert, rootCertPEM, err = CreateCert(rootCertTmpl, rootCertTmpl, &rootKey.PublicKey, rootKey)
	if err != nil {
		log.Errorf("error creating cert: %v", err)
	}
	//fmt.Printf("%s\n", rootCertPEM)
	//fmt.Printf("%#x\n", rootCert.Signature)
	return rootCert, rootKey, rootCertPEM
}

func GenerateServerCert(rootCert *x509.Certificate, rootKey *rsa.PrivateKey, rootCertPEM []byte, dnsNames []string) (servCertPEM, servKeyPEM []byte, err error) {
	if rootCert == nil || rootKey == nil || rootCertPEM == nil {
		return nil, nil, fmt.Errorf("empty params")
	}
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(rootCertPEM)

	// create a key-pair for the server
	servKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	// create a template for the server
	servCertTmpl, err := GetCertTemplate(dnsNames)
	if err != nil {
		return nil, nil, err
	}

	servCertTmpl.KeyUsage = x509.KeyUsageDigitalSignature
	servCertTmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}

	// create a certificate which wraps the server's public key, sign it with the root private key
	_, servCertPEM, err = CreateCert(servCertTmpl, rootCert, &servKey.PublicKey, rootKey)
	if err != nil {
		return nil, nil, err
	}

	// provide the private key and the cert
	servKeyPEM = pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(servKey),
	})
	return servCertPEM, servKeyPEM, nil
}

func GetClientCert(rootCert *x509.Certificate, rootKey any) (clientTLSCert tls.Certificate, clientCertPEM, clientKeyPEM []byte) {
	log.Trace("generate client cert")

	// create a key-pair for the client
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Errorf("generating random key: %v", err)
	}

	// create a template for the client
	clientCertTmpl, err := GetCertTemplate(nil)
	if err != nil {
		log.Errorf("creating cert template: %v", err)
	}
	clientCertTmpl.KeyUsage = x509.KeyUsageDigitalSignature
	clientCertTmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}

	// the root cert signs the cert by again providing its private key
	_, clientCertPEM, err = CreateCert(clientCertTmpl, rootCert, &clientKey.PublicKey, rootKey)
	if err != nil {
		log.Errorf("error creating cert: %v", err)
	}

	// encode and load the cert and private key for the client
	clientKeyPEM = pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey),
	})

	//fmt.Printf("client_cert\n")
	//fmt.Printf("%s\n", clientCertPEM)
	//fmt.Printf("client_key\n")
	//fmt.Printf("%s\n", clientKeyPEM)

	clientTLSCert, err = tls.X509KeyPair(clientCertPEM, clientKeyPEM)
	if err != nil {
		log.Errorf("invalid key pair: %v", err)
	}

	return clientTLSCert, clientCertPEM, clientKeyPEM
}

// helper function to create a cert template with a serial number and other required fields
func GetCertTemplateWithSingleDomain(dnsNames string) (*x509.Certificate, error) {
	if dnsNames == "" {
		dnsNames = "localhost"
	}
	return GetCertTemplate([]string{dnsNames})
}

func GetCertTemplate(dnsNames []string) (*x509.Certificate, error) {
	log.Trace("generate cert template")

	// generate a random serial number (a real cert authority would have some logic behind this)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, errors.New("failed to generate serial number: " + err.Error())
	}

	tmpl := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{Organization: []string{"INFINI.LTD"}},
		SignatureAlgorithm:    x509.SHA256WithRSA,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0), // valid for an year
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
	}
	return &tmpl, nil
}

func CreateCert(template, parent *x509.Certificate, pub interface{}, parentPriv interface{}) (
	cert *x509.Certificate, certPEM []byte, err error) {

	log.Trace("create cert")

	certDER, err := x509.CreateCertificate(rand.Reader, template, parent, pub, parentPriv)
	if err != nil {
		return
	}
	// parse the resulting certificate so we can use it again
	cert, err = x509.ParseCertificate(certDER)
	if err != nil {
		return
	}
	// PEM encode the certificate (this is a standard TLS encoding)
	b := pem.Block{Type: "CERTIFICATE", Bytes: certDER}
	certPEM = pem.EncodeToMemory(&b)
	return
}

func GetSkipHostnameVerifyFunc(pool *x509.CertPool) func([][]byte, [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		// If this is the first handshake on a connection, process and
		// (optionally) verify the server's certificates.
		certs := make([]*x509.Certificate, len(rawCerts))
		for i, asn1Data := range rawCerts {
			cert, err := x509.ParseCertificate(asn1Data)
			if err != nil {
				return fmt.Errorf("failed to parse certificate from server: %w", err)
			}
			certs[i] = cert
		}

		opts := x509.VerifyOptions{
			Roots:         pool,
			CurrentTime:   time.Now(),
			DNSName:       "", // <- skip hostname verification
			Intermediates: x509.NewCertPool(),
		}

		for i, cert := range certs {
			if i == 0 {
				continue
			}
			opts.Intermediates.AddCert(cert)
		}
		_, err := certs[0].Verify(opts)
		return err
	}
}

func ParsePrivateKey(der []byte) (any, error) {
	priKey, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		priKey, err = x509.ParsePKCS1PrivateKey(der)
	}
	return priKey, err
}
