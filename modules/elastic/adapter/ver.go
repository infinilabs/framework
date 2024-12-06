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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rubyniu105/framework/core/elastic"
	"github.com/rubyniu105/framework/core/errors"
	"github.com/rubyniu105/framework/core/util"
	"github.com/rubyniu105/framework/lib/fasthttp"
	"github.com/segmentio/encoding/json"
)

func GetMajorVersion(esConfig *elastic.ElasticsearchMetadata) (elastic.Version, error) {
	esVersion, err := ClusterVersion(esConfig)
	if err != nil {
		return elastic.Version{}, err
	}
	ver := elastic.Version{
		Number:       esVersion.Version.Number,
		Distribution: esVersion.Version.Distribution,
	}
	if ver.Number != "" {
		vs := strings.Split(ver.Number, ".")
		n, err := util.ToInt(vs[0])
		if err != nil {
			panic(err)
		}
		ver.Major = n
	}
	return ver, nil
}

var timeout = 30 * time.Second

func ClusterVersion(metadata *elastic.ElasticsearchMetadata) (*elastic.ClusterInformation, error) {
	url := fmt.Sprintf("%v://%v", metadata.GetSchema(), metadata.GetActiveHost())
	if metadata.Config.RequestTimeout <= 0 {
		metadata.Config.RequestTimeout = 5
	}

	req := util.Request{Method: fasthttp.MethodGet, Url: url}
	if metadata.Config.BasicAuth != nil {
		req.SetBasicAuth(metadata.Config.BasicAuth.Username, metadata.Config.BasicAuth.Password.Get())
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(metadata.Config.RequestTimeout)*time.Second)
	req.Context = ctx
	defer cancel()

	res, err := util.ExecuteRequestWithCatchFlag(nil, &req, true)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != 200 {
		return nil, errors.New(string(res.Body))
	}

	version := elastic.ClusterInformation{}
	err = json.Unmarshal(res.Body, &version)
	if err != nil {
		return nil, err
	}
	if version.Version.Distribution == "" {
		version.Version.Distribution = elastic.Elasticsearch
	}
	return &version, nil
}

func RequestTimeout(ctx *elastic.APIContext, method, url string, body []byte, metadata *elastic.ElasticsearchMetadata, timeout time.Duration) (result *util.Result, err error) {

	ctx.Request.Header.SetMethod(method)
	ctx.Request.SetRequestURI(url)
	ctx.Request.Header.SetContentType(util.ContentTypeJson)

	acceptGzipped := ctx.Request.AcceptGzippedResponse()
	compressed := false

	//gzip request body
	if len(body) > 0 {
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
	if metadata.Config.RequestCompress {
		ctx.Request.Header.Set(fasthttp.HeaderAcceptEncoding, "gzip")
		compressed = true
	}

	if metadata.Config != nil && metadata.Config.BasicAuth != nil {
		ctx.Request.SetBasicAuth(metadata.Config.BasicAuth.Username, metadata.Config.BasicAuth.Password.Get())
	}

	metadata.CheckNodeTrafficThrottle(util.UnsafeBytesToString(ctx.Request.Header.Host()), 1, ctx.Request.GetRequestLength(), 0)

	err = ctx.Client.DoTimeout(ctx.Request, ctx.Response, timeout)
	if err != nil {
		return nil, err
	}

	metadata.CheckNodeTrafficThrottle(util.UnsafeBytesToString(ctx.Request.Header.Host()), 0, ctx.Response.GetResponseLength(), 0)

	//restore body and header
	if !acceptGzipped && compressed {
		body := ctx.Response.GetRawBody()
		ctx.Response.SwapBody(body)
		ctx.Response.Header.Del(fasthttp.HeaderContentEncoding)
		ctx.Response.Header.Del(fasthttp.HeaderContentEncoding2)
	}

	result = &util.Result{
		Body:       ctx.Response.Body(),
		StatusCode: ctx.Response.StatusCode(),
	}

	return result, nil

}

func GetClusterUUID(clusterID string) (string, error) {
	meta := elastic.GetMetadata(clusterID)
	if meta == nil {
		return "", fmt.Errorf("metadata can not be mepty")
	}
	if meta.ClusterState != nil {
		return meta.ClusterState.ClusterUUID, nil
	}
	if meta.Config != nil && meta.Config.ClusterUUID != "" {
		return meta.Config.ClusterUUID, nil
	}
	clusterInfo, err := ClusterVersion(meta)
	if err != nil {
		return "", err
	}
	if meta.Config != nil {
		meta.Config.ClusterUUID = clusterInfo.ClusterUUID
	}
	return clusterInfo.ClusterUUID, nil
}
