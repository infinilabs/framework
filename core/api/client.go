/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	"crypto/tls"
	"crypto/x509"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/util"
	"io/ioutil"
	"net/http"
	"time"
)

func SimpleGetTLSConfig(tlsConfig *config.TLSConfig) *tls.Config {
	if tlsConfig != nil {
		v, _ := GetClientTLSConfig(tlsConfig)
		if v != nil {
			return v
		}
	}

	return &tls.Config{
		InsecureSkipVerify: true,
	}
}

func GetClientTLSConfig(tlsConfig *config.TLSConfig) (*tls.Config, error) {

	pool := x509.NewCertPool()
	clientConfig := &tls.Config{
		RootCAs:            pool,
		ClientSessionCache: tls.NewLRUClientSessionCache(tlsConfig.ClientSessionCacheSize),
		InsecureSkipVerify: tlsConfig.TLSInsecureSkipVerify,
	}

	if util.FileExists(tlsConfig.TLSCACertFile) {
		caCert, err := ioutil.ReadFile(tlsConfig.TLSCACertFile)
		if err != nil {
			return nil, err
		}
		pool.AppendCertsFromPEM(caCert)
	}

	if util.FileExists(tlsConfig.TLSCertFile) && util.FileExists(tlsConfig.TLSKeyFile) {
		clientCert, err := tls.LoadX509KeyPair(tlsConfig.TLSCertFile, tlsConfig.TLSKeyFile)
		if err != nil {
			return nil, err
		}
		clientConfig.Certificates = []tls.Certificate{clientCert}
	}

	if tlsConfig.DefaultDomain != "" {
		clientConfig.ServerName = tlsConfig.DefaultDomain
	} else {
		clientConfig.ServerName = "localhost"
	}

	//skip domain verify if skip tls verify
	if !tlsConfig.TLSInsecureSkipVerify {
		if tlsConfig.SkipDomainVerify {
			clientConfig.VerifyPeerCertificate = util.GetSkipHostnameVerifyFunc(pool)
		}
	}

	return clientConfig, nil

}

func NewHTTPClient(clientCfg *config.HTTPClientConfig) (*http.Client, error) {
	tr := &http.Transport{
		MaxConnsPerHost: clientCfg.MaxConnectionPerHost,
		ReadBufferSize:  clientCfg.ReadBufferSize,
		WriteBufferSize: clientCfg.WriteBufferSize,
		TLSClientConfig: SimpleGetTLSConfig(&clientCfg.TLSConfig),
	}
	return &http.Client{
		Timeout:   util.GetDurationOrDefault(clientCfg.Timeout, 60*time.Second),
		Transport: tr,
	}, nil
}
