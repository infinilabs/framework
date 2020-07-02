//https://ericchiang.github.io/post/go-tls/
package util

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
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

	rootCertTmpl, err := GetCertTemplate()
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

func GetClientCert(rootCert *x509.Certificate, rootKey *rsa.PrivateKey) (clientTLSCert tls.Certificate, clientCertPEM, clientKeyPEM []byte) {
	log.Trace("generate client cert")

	// create a key-pair for the client
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Errorf("generating random key: %v", err)
	}

	// create a template for the client
	clientCertTmpl, err := GetCertTemplate()
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
func GetCertTemplate() (*x509.Certificate, error) {
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
