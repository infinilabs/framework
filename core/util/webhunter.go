/*
Copyright 2016 Medcl (m AT medcl.net)

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
	"bytes"
	"compress/gzip"
	"context"
	"io/ioutil"
	"net"
	"net/http"
	uri "net/url"
	"strings"
	"time"

	"crypto/tls"
	"fmt"
	log "github.com/cihub/seelog"
	"golang.org/x/net/proxy"
	"infini.sh/framework/core/errors"
	"io"
)

const (
	Verb_GET    string = "GET"
	Verb_PUT    string = "PUT"
	Verb_POST   string = "POST"
	Verb_DELETE string = "DELETE"
	Verb_HEAD   string = "HEAD"
)

// GetHost return the host from a url
func GetHost(url string) string {

	if strings.HasPrefix(url, "//") {
		url = strings.TrimPrefix(url, "//")
	}

	array := strings.Split(url, ".")
	if len(array) > 0 {
		t := array[len(array)-1]
		isTLD := IsValidTLD(t)
		if isTLD {
			if !strings.HasPrefix(url, "http") {
				url = "http://" + url
			}
		}
	}

	if strings.Contains(url, "/") {
		if !strings.HasPrefix(url, "http") {
			url = "http://" + url
		}
	}

	uri, err := uri.Parse(url)
	if err != nil {
		log.Trace(err)
		return ""
	}

	return uri.Host
}

//GetRootUrl parse to get url root
func GetRootUrl(source *uri.URL) string {
	if strings.HasSuffix(source.Path, "/") {
		return source.Host + source.Path
	}

	index := strings.LastIndex(source.Path, "/")
	if index > 0 {
		path := source.Path[0:index]
		return source.Host + path
	}

	return source.Host + "/"
}

//FormatUrlForFilter format url, normalize url
func formatUrlForFilter(url []byte) []byte {
	src := string(url)
	log.Trace("start to normalize url:", src)
	if strings.HasSuffix(src, "/") {
		src = strings.TrimRight(src, "/")
	}
	src = strings.TrimSpace(src)
	src = strings.ToLower(src)
	return []byte(src)
}

func getUrlPathFolderWithoutFile(url []byte) []byte {
	src := string(url)
	log.Trace("start to get url's path folder:", src)
	if strings.HasSuffix(src, "/") {
		src = strings.TrimRight(src, "/")
	}
	src = strings.TrimSpace(src)
	src = strings.ToLower(src)
	return []byte(src)
}

func getUrl(url string) (string, error) {
	if !strings.HasPrefix(url, "http") {
		return url, errors.New("invalid url, " + url)
	}
	return url, nil
}

type Request struct {
	Agent       string
	Method      string
	Path        string
	Url         string
	Cookie      string
	Proxy       string
	Body        []byte
	headers     map[string]string
	ContentType string
	Compress    bool

	basicAuthUsername string
	basicAuthPassword string
	Context           context.Context
}

func NewRequest(method, url string) *Request {
	req := Request{}
	req.Url = url
	req.Method = method
	return &req
}

// NewPostRequest issue a simple http post request
func NewPostRequest(url string, body []byte) *Request {
	req := Request{}
	req.Url = url
	req.Method = Verb_POST
	if body != nil {
		req.Body = body
	}
	return &req
}

// NewPutRequest issue a simple http put request
func NewPutRequest(url string, body []byte) *Request {
	req := Request{}
	req.Url = url
	req.Method = Verb_PUT
	if body != nil {
		req.Body = body
	}
	return &req
}

// NewGetRequest issue a simple http get request
func NewGetRequest(url string, body []byte) *Request {
	req := Request{}
	req.Url = url
	if body != nil {
		req.Body = body
	}
	req.Method = Verb_GET
	return &req
}

// NewDeleteRequest issue a simple http delete request
func NewDeleteRequest(url string, body []byte) *Request {
	req := Request{}
	req.Url = url
	if body != nil {
		req.Body = body
	}
	req.Method = Verb_DELETE
	return &req
}

// SetBasicAuth set user and password for request
func (r *Request) SetBasicAuth(username, password string) *Request {
	r.basicAuthUsername = username
	r.basicAuthPassword = password
	return r
}

func (r *Request) SetContentType(contentType string) *Request {
	r.ContentType = contentType
	return r
}

func (r *Request) AddHeader(key, v string) *Request {
	if r.headers == nil {
		r.headers = map[string]string{}
	}
	r.headers[key] = v
	return r
}

func (r *Request) SetAgent(agent string) *Request {
	r.Agent = agent
	return r
}

func (r *Request) AcceptGzip() *Request {
	r.AddHeader("Accept-Encoding", "gzip")
	return r
}

func (r *Request) SetProxy(proxy string) *Request {
	r.Proxy = proxy
	return r
}

// Result is the http request result
type Result struct {
	Host       string
	Url        string
	Headers    map[string][]string
	Body       []byte
	StatusCode int
	Size       uint64
}

const userAgent = "Mozilla/5.0 (compatible; INFINI/1.0; +http://infini.sh/)"

const ContentTypeJson = "application/json;charset=utf-8"
const ContentTypeXml = "application/xml;charset=utf-8"
const ContentTypeForm = "application/x-www-form-urlencoded;charset=UTF-8"

// ExecuteRequest issue a request
func ExecuteRequest(req *Request) (result *Result, err error) {
	return ExecuteRequestWithCatchFlag(defaultClient, req, true)
}

func ExecuteRequestWithCatchFlag(client *http.Client, req *Request, catchError bool) (result *Result, err error) {

	if !catchError {
		defer func() (result *Result, err error) {
			result = &Result{}
			if err := recover(); err != nil {
				log.Errorf("error in request: %s\n", err)
				return result, errors.Errorf("error in request: %s\n", err)
			}
			return nil, err
		}()
	}

	if client == nil {
		client = defaultClient
	}

	//log.Trace("let's: " + req.Method + ", " + req.Url)

	var request *http.Request
	if req.Body != nil && len(req.Body) > 0 {
		postBytesReader := bytes.NewReader(req.Body)
		request, err = http.NewRequest(string(req.Method), req.Url, postBytesReader)
	} else {
		request, err = http.NewRequest(string(req.Method), req.Url, nil)
	}
	if req.Context != nil {
		request = request.WithContext(req.Context)
	}

	if err != nil {
		log.Errorf("error in request: %s\n", err)
		//panic(err)
		return nil, err
	}

	if req.Agent != "" {
		request.Header.Set("User-Agent", req.Agent)
	} else {
		request.Header.Set("User-Agent", userAgent)
	}

	//request.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	//request.Header.Set("Accept-Charset", "GBK,utf-8;q=0.7,*;q=0.3")

	if req.Compress {
		request.Header.Set("Accept-Encoding", "gzip,deflate")
	}

	if req.ContentType != "" {
		request.Header.Set("Content-Type", req.ContentType)
	}

	//request.Header.Set("Cache-Control", "max-age=0")
	//request.Header.Set("Connection", "keep-alive")
	request.Header.Set("Referer", req.Url)

	if req.headers != nil {
		for k, v := range req.headers {
			request.Header.Set(k, v)
		}
	}

	if req.Cookie != "" {
		log.Debug("dealing with cookie:" + req.Cookie)
		array := strings.Split(req.Cookie, ";")
		for item := range array {
			array2 := strings.Split(array[item], "=")
			if len(array2) == 2 {
				cookieObj := http.Cookie{}
				cookieObj.Name = array2[0]
				cookieObj.Value = array2[1]
				request.AddCookie(&cookieObj)
			} else {
				log.Info("error,index out of range:" + array[item])
			}
		}
	}

	if req.basicAuthUsername != "" && req.basicAuthPassword != "" {
		request.SetBasicAuth(req.basicAuthUsername, req.basicAuthPassword)
	}

	if req.Proxy != "" {
		// Create a transport that uses Tor Browser's SocksPort.  If
		// talking to a system tor, this may be an AF_UNIX socket, or
		// 127.0.0.1:9050 instead.
		tbProxyURL, err := uri.Parse(req.Proxy)
		if err != nil {
			//panic(err)
			return nil, fmt.Errorf("Failed to parse proxy URL: %v", err)
		}

		// Get a proxy Dialer that will create the connection on our
		// behalf via the SOCKS5 proxy.  Specify the authentication
		// and re-create the dialer/transport/client if tor's
		// IsolateSOCKSAuth is needed.
		tbDialer, err := proxy.FromURL(tbProxyURL, proxy.Direct)
		if err != nil {
			//panic(err)
			return nil, fmt.Errorf("Failed to obtain proxy dialer: %v", err)
		}

		// Make a http.Transport that uses the proxy dialer, and a
		// http.Client that uses the transport.
		tbTransport := &http.Transport{
			Dial: tbDialer.Dial,
			DialContext: (&net.Dialer{
				Timeout:   timeout,
				KeepAlive: timeout,
				DualStack: true,
			}).DialContext,
		}
		client.Transport = tbTransport
	}

	//handle ipv6
	if strings.Contains(request.Host, "::") {
		//TODO https://zandercodes.dev/post/how-create-ipv6-request-golang
	}

	return execute(client, request)
}

// HttpGetWithCookie issue http request with cookie
func HttpGetWithCookie(resource string, cookie string, proxy string) (*Result, error) {
	req := NewGetRequest(resource, nil)
	if cookie != "" {
		req.Cookie = cookie
	}
	if proxy != "" {
		req.Proxy = proxy
	}
	return ExecuteRequest(req)
}

// HttpGet issue a simple http get request
func HttpGet(resource string) (*Result, error) {
	req := NewGetRequest(resource, nil)
	return ExecuteRequest(req)
}

// HttpDelete issue a simple http delete request
func HttpDelete(resource string) (*Result, error) {
	req := NewDeleteRequest(resource, nil)
	return ExecuteRequest(req)
}

var timeout = 60 * time.Second
var t = &http.Transport{
	Dial: func(netw, addr string) (net.Conn, error) {
		deadline := time.Now().Add(timeout)
		c, err := net.DialTimeout(netw, addr, timeout)
		if err != nil {
			return nil, err
		}
		c.SetDeadline(deadline)
		return c, nil
	},
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   timeout,
		KeepAlive: timeout,
		DualStack: true,
	}).DialContext,
	ResponseHeaderTimeout: timeout,
	IdleConnTimeout:       timeout,
	TLSHandshakeTimeout:   timeout,
	ExpectContinueTimeout: timeout,
	DisableCompression:    true, //TODO
	DisableKeepAlives:     false,
	MaxIdleConns:          20000,
	MaxIdleConnsPerHost:   20000,
	MaxConnsPerHost:       20000,
	TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
}

var defaultClient = &http.Client{
	Transport:     t,
	Timeout:       timeout,
	CheckRedirect: nil,
}

func execute(client *http.Client, req *http.Request) (*Result, error) {
	result := &Result{}
	resp, err := client.Do(req)

	defer func() {
		if resp != nil && resp.Body != nil {
			io.Copy(ioutil.Discard, resp.Body)
			resp.Body.Close()
		}
	}()

	if err != nil {
		return result, err
	}

	if resp != nil {
		statusCode := resp.StatusCode
		result.StatusCode = statusCode

		if statusCode == 301 || statusCode == 302 {

			log.Debug("got redirect: ", req.URL, " => ", resp.Header.Get("Location"))
			location := resp.Header.Get("Location")
			if len(location) > 0 && location != req.URL.String() {
				return result, errors.NewWithPayload(err, errors.URLRedirected, location, fmt.Sprint("got redirect: ", req.URL, " => ", location))
			}
		}

		// update host, redirects may change the host
		result.Host = resp.Request.Host
		result.Url = resp.Request.URL.String()
	}

	if resp.Header != nil {

		result.Headers = map[string][]string{}
		for k, v := range resp.Header {
			result.Headers[strings.ToLower(k)] = v
		}
	}

	reader := resp.Body

	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err = gzip.NewReader(resp.Body)

		if err != nil {
			return nil, err
			//panic(err)
		}
	}

	if reader != nil {
		body, err := ioutil.ReadAll(reader)
		io.Copy(ioutil.Discard, reader)
		reader.Close()
		if err != nil {
			return result, nil
			//panic(err)
		}

		result.Body = body
		result.Size = uint64(len(body))
		return result, nil
	}

	return nil, http.ErrNotSupported
}
