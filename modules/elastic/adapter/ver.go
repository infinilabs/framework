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

var timeout=30*time.Second
func ClusterVersion(metadata *elastic.ElasticsearchMetadata) (*elastic.ClusterInformation, error) {
	url := fmt.Sprintf("%v://%v", metadata.GetSchema(), metadata.GetActiveHost())

	if metadata.Config.RequestTimeout<=0{
		metadata.Config.RequestTimeout=30
	}
	var req=fasthttp.AcquireRequest()
	var res=fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(res)


	ctx:=elastic.APIContext{
		Client:  &fasthttp.Client{
			MaxConnsPerHost: 1000,
			TLSConfig:       &tls.Config{InsecureSkipVerify: true},
			MaxConnWaitTimeout: timeout,
			MaxIdleConnDuration: timeout,
			WriteTimeout: timeout,
			ReadTimeout: timeout,
		},
		Request: req,
		Response: res,
	}
	result, err := RequestTimeout(&ctx,"GET", url, nil, metadata, time.Duration(metadata.Config.RequestTimeout) * time.Second)
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

func RequestTimeout(ctx *elastic.APIContext,method, url string, body []byte, metadata *elastic.ElasticsearchMetadata, timeout time.Duration) (result *util.Result, err error) {

	ctx.Request.Header.SetMethod(method)
	ctx.Request.SetRequestURI(url)
	ctx.Request.Header.SetContentType(util.ContentTypeJson)

	acceptGzipped:=ctx.Request.AcceptGzippedResponse()
	compressed:=false

	//gzip request body
	if len(body)>0{
		if !ctx.Request.IsGzipped() && metadata.Config.RequestCompress {
			_, err := fasthttp.WriteGzipLevel(ctx.Request.BodyWriter(), body, fasthttp.CompressBestSpeed)
			if err != nil {
				panic(err)
			}
			ctx.Request.Header.Set(fasthttp.HeaderContentEncoding, "gzip")
			//compressed=true
		} else {
			ctx.Request.SetBody(body)
		}
	}

	//allow to receive gzipped response
	if metadata.Config.RequestCompress{
		ctx.Request.Header.Set(fasthttp.HeaderAcceptEncoding, "gzip")
		compressed=true
	}

	if metadata.Config != nil && metadata.Config.BasicAuth != nil {
		ctx.Request.SetBasicAuth(metadata.Config.BasicAuth.Username, metadata.Config.BasicAuth.Password)
	}

	metadata.CheckNodeTrafficThrottle(util.UnsafeBytesToString(ctx.Request.Header.Host()),1,ctx.Request.GetRequestLength(),0)

	err = ctx.Client.DoTimeout(ctx.Request, ctx.Response, timeout)
	if err != nil {
		return nil, err
	}

	metadata.CheckNodeTrafficThrottle(util.UnsafeBytesToString(ctx.Request.Header.Host()),0,ctx.Response.GetResponseLength(),0)

	//restore body and header
	if !acceptGzipped&&compressed{
		body:=ctx.Response.GetRawBody()
		ctx.Response.SwapBody(body)
		ctx.Response.Header.Del(fasthttp.HeaderContentEncoding)
		ctx.Response.Header.Del(fasthttp.HeaderContentEncoding2)
	}

	result = &util.Result{
		Body: ctx.Response.Body(),
		StatusCode: ctx.Response.StatusCode(),
	}

	return result, nil

}
