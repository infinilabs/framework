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

package elasticsearch

import (
	"errors"
	"fmt"

	log "github.com/cihub/seelog"
	"github.com/segmentio/encoding/json"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
)

type ESAPIV5 struct {
	ESAPIV2
}

func (c *ESAPIV5) InitDefaultTemplate(templateName, indexPrefix string) {
	c.initTemplate(templateName, indexPrefix)
}

func (c *ESAPIV5) getDefaultTemplate(indexPrefix string) string {
	template := `
{
"template": "%s*",
"settings": {
    "number_of_shards": %v,
    "index.mapping.total_fields.limit": 20000,
    "index.max_result_window":10000000,
	"index.analysis.analyzer": {
            "suggest_text_search": {
              "filter": [
                "word_delimiter"
              ],
              "tokenizer": "classic"
            }
	}
  },
  "mappings": {
    "%s": {
      "dynamic_templates": [
        {
          "strings": {
            "match_mapping_type": "string",
            "mapping": {
              "type": "keyword",
              "ignore_above": 256
            }
          }
        }
      ]
    }
  }
}
`
	return fmt.Sprintf(template, indexPrefix, 1, TypeName5)
}

func (c *ESAPIV5) initTemplate(templateName, indexPrefix string) {
	if global.Env().IsDebug {
		log.Trace("init elasticsearch template")
	}
	if templateName == "" {
		templateName = global.Env().GetAppLowercaseName()
	}
	exist, err := c.TemplateExists(templateName)
	if err != nil {
		panic(err)
	}
	if !exist {
		template := c.getDefaultTemplate(indexPrefix)
		log.Trace("template: ", template)
		res, err := c.PutTemplate(templateName, []byte(template))
		if err != nil {
			panic(err)
		}
		if global.Env().IsDebug {
			log.Trace("put template response, %v", string(res))
		}
	}
	log.Debugf("elasticsearch template successful initialized")

}

const TypeName5 = "doc"

func (s *ESAPIV5) NewScroll(indexNames string, scrollTime string, docBufferCount int, query *elastic.SearchRequest, slicedId, maxSlicedCount int) ([]byte, error) {
	indexNames = util.UrlEncode(indexNames)

	url := fmt.Sprintf("%s/%s/_search?scroll=%s&size=%d", s.GetEndpoint(), indexNames, scrollTime, docBufferCount)
	var err error
	if maxSlicedCount > 1 {
		//log.Tracef("sliced scroll, %d of %d",slicedId,maxSlicedCount)
		err = query.Set("slice", util.MapStr{
			"id":  slicedId,
			"max": maxSlicedCount,
		})
		if err != nil {
			panic(err)
		}
	}

	var jsonBody string
	if query != nil {
		jsonBody = query.ToJSONString()
	}

	resp, err := s.Request(nil, util.Verb_POST, url, util.UnsafeStringToBytes(jsonBody))

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, errors.New(string(resp.Body))
	}

	return resp.Body, err
}

func (s *ESAPIV5) SetSearchTemplate(templateID string, body []byte) error {
	ver := s.GetVersion()
	if ver.Distribution == "" {
		cr, err := util.VersionCompare(ver.Number, "5.6")
		if err != nil {
			return err
		}
		if cr == -1 {
			//fmt.Println(s.Version, templateID)
			return s.ESAPIV2.SetSearchTemplate(templateID, body)
		}
	}

	url := fmt.Sprintf("%s/_scripts/%s", s.GetEndpoint(), templateID)
	_, err := s.Request(nil, util.Verb_PUT, url, body)
	return err
}

func (c *ESAPIV5) CatNodes(colStr string) ([]elastic.CatNodeResponse, error) {
	ver := c.GetVersion()
	path := "_cat/nodes?format=json&full_id"
	if ver.Number == "5.0.0" && (ver.Distribution == elastic.Elasticsearch || ver.Distribution == "") {
		//https://github.com/elastic/elasticsearch/issues/21266
		path = "_cat/nodes?format=json"
	}
	url := fmt.Sprintf("%s/%s", c.GetEndpoint(), path)
	if colStr != "" {
		url = fmt.Sprintf("%s&h=%s", url, colStr)
	}
	resp, err := c.Request(nil, util.Verb_GET, url, nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, errors.New(string(resp.Body))
	}

	data := []elastic.CatNodeResponse{}
	err = json.Unmarshal(resp.Body, &data)
	return data, err
}



func (c *ESAPIV5) Update(indexName, docType string, id interface{}, data interface{}, refresh string) (*elastic.InsertResponse, error) {

	if docType == "" {
		docType = TypeName5
	}

	indexName = util.UrlEncode(indexName)

	url := fmt.Sprintf("%s/%s/%s/%s/_update", c.GetEndpoint(), indexName, docType, id)

	if id == "" {
		panic(errors.New("id is required"))
	}
	if refresh != "" {
		url = fmt.Sprintf("%s?refresh=%s", url, refresh)
	}

	js:=util.MapStr{}
	js["doc"]=data
	js["detect_noop"]=false
	js["doc_as_upsert"]=true

	resp, err := c.Request(nil, util.Verb_POST, url, util.MustToJSONBytes(js))
	if err != nil {
		return nil, err
	}

	if global.Env().IsDebug {
		log.Trace("indexing response: ", string(resp.Body))
	}

	esResp := &elastic.InsertResponse{}
	err = json.Unmarshal(resp.Body, esResp)
	if err != nil {
		return &elastic.InsertResponse{}, err
	}
	if !(esResp.Result == "created" || esResp.Result == "updated" || esResp.Shards.Successful > 0) {
		return nil, errors.New(string(resp.Body))
	}

	return esResp, nil
}
