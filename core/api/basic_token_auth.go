/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import "net/http"

const headerBasicToken = "X-INFINI-BASIC-TOKEN"

type BasicTokenTransport struct {
	Token     string
	Transport http.RoundTripper
}

func (t *BasicTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := new(http.Request)
	*req2 = *req
	req2.Header = make(http.Header, len(req.Header))
	for k, s := range req.Header {
		req2.Header[k] = append([]string(nil), s...)
	}

	if t.Token != "" {
		req2.Header.Set(headerBasicToken, t.Token)
	}
	return t.transport().RoundTrip(req2)
}

func (t *BasicTokenTransport) Client() *http.Client {
	return &http.Client{Transport: t}
}

func (t *BasicTokenTransport) transport() http.RoundTripper {
	if t.Transport != nil {
		return t.Transport
	}
	return http.DefaultTransport
}
