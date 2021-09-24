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
	log "github.com/cihub/seelog"
	"github.com/segmentio/encoding/json"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/elastic/model"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/fasthttp"
	"time"
)

func GetMajorVersion(esConfig model.ElasticsearchConfig)(string, error)  {
	esVersion, err := ClusterVersion(&esConfig)
	if err != nil {
		return "", err
	}
	return esVersion.Version.Number, nil
}

func ClusterVersion(config *model.ElasticsearchConfig) (*elastic.ClusterInformation, error) {

	//req := util.NewGetRequest(fmt.Sprintf("%s", config.Endpoint), nil)
	//
	//req.SetContentType(util.ContentTypeJson)
	//
	//if config.BasicAuth != nil {
	//	req.SetBasicAuth(config.BasicAuth.Username, config.BasicAuth.Password)
	//}
	//if config.HttpProxy != "" {
	//	req.SetProxy(config.HttpProxy)
	//}
	//
	//response, err := util.ExecuteRequest(req)
	//if err != nil {
	//	return nil, err
	//}
	//
	//if response.StatusCode!=200{
	//	panic(errors.New(string(response.Body)))
	//}
	//
	//version := elastic.ClusterInformation{}
	//err = json.Unmarshal(response.Body, &version)
	//if err != nil {
	//	log.Error(string(response.Body))
	//	return nil, err
	//}
	//return &version, nil

	url := fmt.Sprintf("%s", config.Endpoint)
	result, err := RequestTimeout("GET", url, nil, config, time.Second * 3)
	if err != nil {
		return nil, err
	}

	if result.StatusCode!=200{
		return nil, errors.New(string(result.Body))
	}

	version := elastic.ClusterInformation{}
	err = json.Unmarshal(result.Body, &version)
	if err != nil {
		log.Error(string(result.Body))
		return nil, err
	}
	return &version, nil
}

func RequestTimeout(method, url string, body []byte, config *model.ElasticsearchConfig, timeout time.Duration) (result *util.Result, err error) {
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
		ReadTimeout: time.Second * 60,
	}
	req.Header.SetMethod(method)
	req.SetBody(body)
	req.SetRequestURI(url)
	req.Header.SetContentType(util.ContentTypeJson)
	if config != nil && config.BasicAuth != nil {
		req.SetBasicAuth(config.BasicAuth.Username, config.BasicAuth.Password)
	}

	err = client.DoTimeout(req, res, timeout)
	if err != nil {
		return nil, err
	}
	result = &util.Result{
		Body: res.Body(),
		StatusCode: res.StatusCode(),
	}
	return result, nil

}
