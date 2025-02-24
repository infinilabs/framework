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
	"fmt"
	log "github.com/cihub/seelog"
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

	if global.Env().IsDebug {
		log.Debugf("http client setting [%v] not found, using default", name)
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

						log.Infof("using proxy: %v for addr: %v", util.MustToJSON(cfg), addr)

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

		if  clientCfg.Proxy.Enabled&& clientCfg.Proxy.OverrideSystemProxy {
			log.Debugf("override system proxy settings: %v %s", util.MustToJSON(clientCfg.Proxy.DefaultProxyConfig))
			UpdateProxyEnvironment(&clientCfg.Proxy.DefaultProxyConfig)
		}

	})
}
