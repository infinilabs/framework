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

package adapter

import (
	"crypto/tls"
	"fmt"
	"github.com/segmentio/encoding/json"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/fasthttp"
	"time"
)

func GetMajorVersion(esConfig *elastic.ElasticsearchMetadata)(string, error)  {
	esVersion, err := ClusterVersion(esConfig)
	if err != nil {
		return "", err
	}
	return esVersion.Version.Number, nil
}

func ClusterVersion(metadata *elastic.ElasticsearchMetadata) (*elastic.ClusterInformation, error) {
	url := fmt.Sprintf("%v://%v", metadata.GetSchema(), metadata.GetActiveHost())

	if metadata.Config.RequestTimeout<=0{
		metadata.Config.RequestTimeout=30
	}

	result, err := RequestTimeout("GET", url, nil, metadata, time.Duration(metadata.Config.RequestTimeout) * time.Second)
	if err != nil {
		return nil, err
	}

	if result.StatusCode!=200{
		return nil, errors.New(string(result.Body))
	}

	version := elastic.ClusterInformation{}
	err = json.Unmarshal(result.Body, &version)
	if err != nil {
		return nil, err
	}
	return &version, nil
}

func RequestTimeout(method, url string, body []byte, metadata *elastic.ElasticsearchMetadata, timeout time.Duration) (result *util.Result, err error) {

	var (
		req = fasthttp.AcquireRequest()
		res = fasthttp.AcquireResponse()
	)
	defer func() {
		fasthttp.ReleaseRequest(req)
		fasthttp.ReleaseResponse(res)
	}()

	client := &fasthttp.Client{
		MaxConnsPerHost: 1000,
		TLSConfig:       &tls.Config{InsecureSkipVerify: true},
		MaxConnWaitTimeout: timeout,
		MaxIdleConnDuration: timeout,
		WriteTimeout: timeout,
		ReadTimeout: timeout,
	}

	req.Header.SetMethod(method)
	req.SetRequestURI(url)
	req.Header.SetContentType(util.ContentTypeJson)

	acceptGzipped:=req.AcceptGzippedResponse()
	compressed:=false

	//gzip request body
	if len(body)>0{
		if !req.IsGzipped() && metadata.Config.RequestCompress {
			_, err := fasthttp.WriteGzipLevel(req.BodyWriter(), body, fasthttp.CompressBestSpeed)
			if err != nil {
				panic(err)
			}
			req.Header.Set(fasthttp.HeaderContentEncoding, "gzip")
			//compressed=true
		} else {
			req.SetBody(body)
		}
	}

	//allow to receive gzipped response
	if metadata.Config.RequestCompress{
		req.Header.Set(fasthttp.HeaderAcceptEncoding, "gzip")
		compressed=true
	}

	if metadata.Config != nil && metadata.Config.BasicAuth != nil {
		req.SetBasicAuth(metadata.Config.BasicAuth.Username, metadata.Config.BasicAuth.Password)
	}

	err = client.DoTimeout(req, res, timeout)
	if err != nil {
		return nil, err
	}

	//restore body and header
	if !acceptGzipped&&compressed{
		body:=res.GetRawBody()
		res.SwapBody(body)
		res.Header.Del(fasthttp.HeaderContentEncoding)
		res.Header.Del(fasthttp.HeaderContentEncoding2)
	}

	result = &util.Result{
		Body: res.Body(),
		StatusCode: res.StatusCode(),
	}

	return result, nil

}
