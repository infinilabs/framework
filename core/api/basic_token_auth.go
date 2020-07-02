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
