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

/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	log "github.com/cihub/seelog"
	"golang.org/x/crypto/pkcs12"
	"golang.org/x/net/proxy"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/fasthttp"
	"infini.sh/framework/lib/fasthttp/fasthttpproxy"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

	skipVerify := tlsConfig.TLSInsecureSkipVerify
	if tlsConfig.TLSBypassMalformedCert {
		skipVerify = true // Force skip verify when dealing with malformed certs like GitLab
	}

	clientConfig := &tls.Config{
		RootCAs:            pool,
		ClientSessionCache: tls.NewLRUClientSessionCache(tlsConfig.ClientSessionCacheSize),
		InsecureSkipVerify: skipVerify,
	}

	// Handle malformed certificates with duplicate extensions (GitLab issue)
	if tlsConfig.TLSBypassMalformedCert {
		// Use our custom bypass function that won't fail on parsing errors
		clientConfig.VerifyPeerCertificate = util.GetBypassCertificateVerifyFunc()
		log.Debug("TLS certificate bypass enabled for malformed certificates (duplicate extensions)")
		// Ensure InsecureSkipVerify is true to handle parsing during handshake
		clientConfig.InsecureSkipVerify = true
	}

	if util.FileExists(tlsConfig.TLSCACertFile) {
		caCert, err := ioutil.ReadFile(tlsConfig.TLSCACertFile)
		if err != nil {
			return nil, err
		}
		pool.AppendCertsFromPEM(caCert)
	}

	// --- Load client certificate automatically (.p12 or .pem) ---
	if util.FileExists(tlsConfig.TLSCertFile) {
		ext := filepath.Ext(tlsConfig.TLSCertFile)

		switch ext {
		case ".p12", ".pfx":
			if err := loadP12Cert(clientConfig, pool, tlsConfig); err != nil {
				log.Error(err)
				return nil, err
			}

		default:
			// fallback to PEM-based
			if util.FileExists(tlsConfig.TLSKeyFile) {
				clientCert, err := tls.LoadX509KeyPair(tlsConfig.TLSCertFile, tlsConfig.TLSKeyFile)
				if err != nil {
					return nil, fmt.Errorf("load pem cert: %w", err)
				}
				clientConfig.Certificates = []tls.Certificate{clientCert}
			} else {
				// If key file missing, check if maybe a .p12 exists beside it
				p12Path := tlsConfig.TLSCertFile[:len(tlsConfig.TLSCertFile)-len(ext)] + ".p12"
				if util.FileExists(p12Path) {
					tlsConfig.TLSCertFile = p12Path
					if err := loadP12Cert(clientConfig, pool, tlsConfig); err != nil {
						return nil, err
					}
				}
			}
		}
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

func printCertInfo(cert *x509.Certificate, source string) {
	fmt.Println("-----[ TLS Certificate Loaded ]-----")
	fmt.Printf("Source File:    %s\n", source)
	fmt.Printf("Subject:        %s\n", cert.Subject.String())
	fmt.Printf("Issuer:         %s\n", cert.Issuer.String())
	fmt.Printf("Serial Number:  %s\n", cert.SerialNumber.String())
	fmt.Printf("Valid From:     %s\n", cert.NotBefore.Format(time.RFC3339))
	fmt.Printf("Valid To:       %s\n", cert.NotAfter.Format(time.RFC3339))
	fmt.Printf("DNS Names:      %v\n", cert.DNSNames)
	fmt.Printf("Email Addresses:%v\n", cert.EmailAddresses)
	fmt.Printf("IP Addresses:   %v\n", cert.IPAddresses)
	fmt.Printf("Signature Algo: %s\n", cert.SignatureAlgorithm.String())
	fmt.Println("------------------------------------")
}

func printPEMCertInfo(certFile string) {
	data, err := ioutil.ReadFile(certFile)
	if err != nil {
		fmt.Printf("Failed to read cert file %s: %v\n", certFile, err)
		return
	}

	var block *pem.Block
	for {
		block, data = pem.Decode(data)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				fmt.Printf("Failed to parse cert from %s: %v\n", certFile, err)
				continue
			}
			if global.Env().IsDebug {
				printCertInfo(cert, certFile)
			}
		}
	}
}

func CheckP12Password(certFile, password string) error {
	data, err := ioutil.ReadFile(certFile)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	_, _, err = pkcs12.Decode(data, password)
	if err != nil {
		if strings.Contains(err.Error(), "incorrect password") {
			return fmt.Errorf("invalid password")
		}
		return fmt.Errorf("parse error: %w", err)
	}
	return nil
}

// loadP12Cert loads PKCS#12 certificate and adds it to client config
func loadP12Cert(clientConfig *tls.Config, pool *x509.CertPool, tlsConfig *config.TLSConfig) error {
	log.Trace("start loading P12 cert")
	pfxData, err := ioutil.ReadFile(tlsConfig.TLSCertFile)
	if err != nil {
		return fmt.Errorf("read p12 cert: %w", err)
	}

	// try to decode (use password if provided)
	password := tlsConfig.TLSCertPassword
	privateKey, cert, err := pkcs12.Decode(pfxData, password)
	if err != nil {
		// Check if this is a certificate parsing error (duplicate extensions)
		if strings.Contains(err.Error(), "duplicate extension") || strings.Contains(err.Error(), "certificate contains") || strings.Contains(err.Error(), "2.5.29.35") {
			log.Warnf("Client P12 certificate has duplicate extensions, attempting to load with bypass: %v", err)

			// Try our bypass function - which will give a more descriptive error
			_, _, bypassErr := util.ParsePKCS12WithDuplicateExtensionTolerance(pfxData, password)
			if bypassErr != nil {
				log.Error(bypassErr)
				return bypassErr
			}

			// If bypass somehow succeeded (it currently won't), continue
			log.Info("Client certificate loaded with bypass")

			return nil
		}
		log.Error("P12 certificate parsing error: ", err)
		return fmt.Errorf("parse p12 cert: %w", err)
	}

	tlsCert := tls.Certificate{
		PrivateKey:  privateKey,
		Certificate: [][]byte{cert.Raw},
	}

	clientConfig.Certificates = []tls.Certificate{tlsCert}
	pool.AddCert(cert)

	log.Trace("success loaded P12 cert")

	if global.Env().IsDebug {
		printCertInfo(cert, tlsConfig.TLSCertFile)
	}

	return nil
}

func NewHTTPClient(clientCfg *config.HTTPClientConfig) (*http.Client, error) {
	if clientCfg == nil {
		panic("client config is nil")
	}

	// Custom dialer with proxy logic
	dialer := &net.Dialer{
		Timeout:   util.GetDurationOrDefault(clientCfg.DialTimeout, 2*time.Second),
		DualStack: true,
	}

	transport := &http.Transport{
		MaxConnsPerHost: clientCfg.MaxConnectionPerHost,
		TLSClientConfig: SimpleGetTLSConfig(&clientCfg.TLSConfig),
		ReadBufferSize:  clientCfg.ReadBufferSize,
		WriteBufferSize: clientCfg.WriteBufferSize,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Determine whether to use a proxy for this address
			if clientCfg.Proxy.Enabled {
				if ok, cfg := clientCfg.ValidateProxy(addr); ok {
					if cfg != nil {
						log.Infof("using proxy: %v for addr: %v", util.MustToJSON(cfg), addr)
						if cfg.HTTPProxy != "" {
							// HTTP proxy
							proxyURL, err := url.Parse(cfg.HTTPProxy)
							if err != nil {
								return nil, fmt.Errorf("invalid HTTP proxy URL: %w", err)
							}
							proxy := http.ProxyURL(proxyURL)
							req := &http.Request{URL: &url.URL{Host: addr}}
							proxyURL, err = proxy(req)
							if err != nil {
								return nil, err
							}
							if proxyURL != nil {
								return dialer.DialContext(ctx, network, proxyURL.Host)
							}
						} else if cfg.Socket5Proxy != "" {
							// SOCKS5 proxy
							socksDialer, err := proxy.SOCKS5("tcp", cfg.Socket5Proxy, nil, dialer)
							if err != nil {
								return nil, fmt.Errorf("error creating SOCKS5 dialer: %w", err)
							}
							return socksDialer.Dial(network, addr)
						} else if cfg.UsingEnvironmentProxySettings {
							// Use environment proxy settings
							envProxy := http.ProxyFromEnvironment
							req := &http.Request{URL: &url.URL{Host: addr}}
							proxyURL, err := envProxy(req)
							if err != nil {
								return nil, err
							}
							if proxyURL != nil {
								return dialer.DialContext(ctx, network, proxyURL.Host)
							}
						}
					}
				}
			}

			// Connect directly
			return dialer.DialContext(ctx, network, addr)
		},
	}

	return &http.Client{
		Timeout:   util.GetDurationOrDefault(clientCfg.Timeout, 60*time.Second),
		Transport: transport,
	}, nil
}

// DiagnoseP12Certificate loads and diagnoses a P12 certificate file
func DiagnoseP12Certificate(certFile, password string) error {
	if !util.FileExists(certFile) {
		return fmt.Errorf("P12 file not found: %s", certFile)
	}

	log.Infof("=== DIAGNOSING P12 CERTIFICATE: %s ===", certFile)

	pfxData, err := ioutil.ReadFile(certFile)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	log.Infof("File size: %d bytes", len(pfxData))

	// Try to decode with provided password
	privateKey, cert, err := pkcs12.Decode(pfxData, password)
	if err != nil {
		log.Errorf("P12 DECODE FAILED:")
		log.Errorf("Error: %v", err)

		// Analyze the error
		errStr := err.Error()
		if strings.Contains(errStr, "duplicate extension") {
			log.Error("⚠️  YOUR CLIENT CERTIFICATE has duplicate extensions (AUTHORITY KEY IDENTIFIER)")
			log.Error("   This means YOUR P12 file is malformed, not the GitLab server")
			log.Error("   You need to regenerate your client certificate with a proper Certificate Authority")
		}
		if strings.Contains(errStr, "wrong password") || strings.Contains(errStr, "password") {
			log.Error("⚠️  Password is incorrect for this P12 file")
		}
		if strings.Contains(errStr, "passthrough") {
			log.Error("⚠️  P12 file format issue - may be corrupted or in wrong format")
		}

		return fmt.Errorf("client P12 certificate parsing failed: %w", err)
	}

	// Success - certificate loaded fine
	log.Info("✅ CLIENT CERTIFICATE PARSED SUCCESSFULLY")
	log.Infof("Subject: %s", cert.Subject)
	log.Infof("Issuer: %s", cert.Issuer)
	log.Infof("Serial Number: %s", cert.SerialNumber)
	log.Infof("Valid From: %s to %s", cert.NotBefore.Format("2006-01-02"), cert.NotAfter.Format("2006-01-02"))

	if privateKey != nil {
		log.Info("✅ Private key loaded successfully")
	} else {
		log.Warn("⚠️  No private key found in P12 file")
	}

	return nil
}

var (
	defaultHttpClient     *http.Client
	defaultFastHttpClient *fasthttp.Client
	fastHTTPClients       = sync.Map{}
	httpClients           = sync.Map{}
	clientInitLocker      = sync.RWMutex{}
)

func GetHttpClient(name string) *http.Client {
	if v, ok := httpClients.Load(name); ok {
		x, ok := v.(*http.Client)
		if ok && x != nil {
			return x
		}
	}

	//init client and save to store
	if cfg, ok := global.Env().SystemConfig.HTTPClientConfig[name]; ok {
		clientInitLocker.Lock()
		defer clientInitLocker.Unlock()
		v, ok := httpClients.Load(name)
		if !ok {
			client, err := NewHTTPClient(&cfg)
			if client != nil && err == nil {
				httpClients.Store(name, client)
				return client
			}
		}
		x, ok := v.(*http.Client)
		if ok {
			return x
		}
	}

	if global.Env().IsDebug {
		log.Debugf("http client setting [%v] not found, using default", name)
	}

	if defaultHttpClient == nil {
		panic("default http client should not be nil")
	}

	return defaultHttpClient
}

func GetFastHttpClient(name string) *fasthttp.Client {

	if v, ok := fastHTTPClients.Load(name); ok {
		x, ok := v.(*fasthttp.Client)
		if ok && x != nil {
			return x
		}
	}

	if global.Env().IsDebug {
		log.Debugf("fasthttp client setting [%v] not found, using default", name)
	}

	//init client and save to store
	if cfg, ok := global.Env().SystemConfig.HTTPClientConfig[name]; ok {
		clientInitLocker.Lock()
		defer clientInitLocker.Unlock()
		v, ok := fastHTTPClients.Load(name)
		if !ok {
			client := getFastHTTPClient(&cfg)
			if client != nil {
				fastHTTPClients.Store(name, client)
				return client
			}
		}
		x, ok := v.(*fasthttp.Client)
		if ok {
			return x
		}
	}

	if defaultFastHttpClient == nil {
		panic("default http client should not be nil")
	}

	return defaultFastHttpClient
}

func getFastHTTPClient(clientCfg *config.HTTPClientConfig) *fasthttp.Client {

	if clientCfg == nil {
		panic("client config is nil")
	}

	return &fasthttp.Client{
		MaxConnsPerHost: clientCfg.MaxConnectionPerHost,
		TLSConfig:       SimpleGetTLSConfig(&clientCfg.TLSConfig),
		ReadTimeout:     util.GetDurationOrDefault(clientCfg.ReadTimeout, 60*time.Second),
		WriteTimeout:    util.GetDurationOrDefault(clientCfg.ReadTimeout, 60*time.Second),
		DialDualStack:   true,
		ReadBufferSize:  clientCfg.ReadBufferSize,
		WriteBufferSize: clientCfg.WriteBufferSize,
		Dial: func(addr string) (net.Conn, error) {

			// Determine whether to use a proxy for this address
			if clientCfg.Proxy.Enabled {
				if ok, cfg := clientCfg.ValidateProxy(addr); ok {
					if cfg != nil {

						log.Debugf("using proxy: %v for addr: %v", util.MustToJSON(cfg), addr)

						if cfg.HTTPProxy != "" {
							dialer := fasthttpproxy.FasthttpHTTPDialer(cfg.HTTPProxy)
							return dialer(addr)
						} else if cfg.Socket5Proxy != "" {
							dialer := fasthttpproxy.FasthttpSocksDialer(cfg.Socket5Proxy)
							return dialer(addr)
						} else if cfg.UsingEnvironmentProxySettings {
							proxyDialer := fasthttpproxy.FasthttpProxyHTTPDialerTimeout(util.GetDurationOrDefault(clientCfg.DialTimeout, 2*time.Second))
							return proxyDialer(addr)
						}
					}
				}
			}

			// Connect directly
			return fasthttp.DialTimeout(addr, util.GetDurationOrDefault(clientCfg.DialTimeout, 2*time.Second))
		},
	}
}

// UpdateProxyEnvironment sets the system environment variables based on the ProxyConfig.
// It will override any existing environment variables with the new values provided in the config.
func UpdateProxyEnvironment(cfg *config.ProxyConfig) {

	if cfg.HTTPProxy != "" {
		_ = os.Setenv("HTTP_PROXY", cfg.HTTPProxy)
		_ = os.Setenv("HTTPS_PROXY", cfg.HTTPProxy)
	} else {
		// Clear the proxy if not set in the config
		_ = os.Unsetenv("HTTP_PROXY")
		_ = os.Unsetenv("HTTPS_PROXY")
	}

	if cfg.Socket5Proxy != "" {
		// Optionally handle SOCKS5 proxies as an environment variable if needed
		_ = os.Setenv("SOCKS5_PROXY", cfg.Socket5Proxy)
	} else {
		_ = os.Unsetenv("SOCKS5_PROXY")
	}

	if cfg.UsingEnvironmentProxySettings {
		// Ensure other env-based proxy settings are respected
		if os.Getenv("HTTP_PROXY") == "" || os.Getenv("HTTPS_PROXY") == "" {
			_ = os.Setenv("HTTP_PROXY", cfg.HTTPProxy)
			_ = os.Setenv("HTTPS_PROXY", cfg.HTTPProxy)
		}
	} else {
		// Clear NO_PROXY if using manual configuration
		_ = os.Unsetenv("NO_PROXY")
	}
}

func init() {
	global.RegisterInitCallback(func() {

		//init the default client
		clientCfg := global.Env().GetHTTPClientConfig("default", "")
		defaultFastHttpClient = getFastHTTPClient(clientCfg)

		var err error
		defaultHttpClient, err = NewHTTPClient(clientCfg)
		if err != nil {
			panic(err)
		}

		if clientCfg.Proxy.Enabled && clientCfg.Proxy.OverrideSystemProxy {
			log.Debugf("override system proxy settings: %v %s", util.MustToJSON(clientCfg.Proxy.DefaultProxyConfig))
			UpdateProxyEnvironment(&clientCfg.Proxy.DefaultProxyConfig)
		}

	})
}
