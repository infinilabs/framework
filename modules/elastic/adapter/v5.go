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
	"errors"
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
)

type ESAPIV5 struct {
	ESAPIV2
}


func (c *ESAPIV5) InitDefaultTemplate(templateName,indexPrefix string) {
	c.initTemplate(templateName,indexPrefix)
}

func (c *ESAPIV5) getDefaultTemplate(indexPrefix string) string {
	template := `
{
"template": "%s*",
"settings": {
    "number_of_shards": %v,
    "index.mapping.total_fields.limit": 20000,
    "index.max_result_window":10000000
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

func (c *ESAPIV5) initTemplate(templateName,indexPrefix string) {
	if global.Env().IsDebug {
		log.Trace("init elasticsearch template")
	}
	if templateName==""{
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

func (s *ESAPIV5) NewScroll(indexNames string, scrollTime string, docBufferCount int, query *elastic.SearchRequest, slicedId, maxSlicedCount int) ([]byte,  error) {
	indexNames=util.UrlEncode(indexNames)

	url := fmt.Sprintf("%s/%s/_search?scroll=%s&size=%d", s.GetEndpoint(), indexNames, scrollTime, docBufferCount)
	var err error
	if maxSlicedCount > 1 {
		//log.Tracef("sliced scroll, %d of %d",slicedId,maxSlicedCount)
		err=query.Set("slice",util.MapStr{
			"id":slicedId,
			"max":maxSlicedCount,
		})
		if err != nil {
			panic(err)
		}
	}

	var jsonBody string
	if query!=nil{
		jsonBody=query.ToJSONString()
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
	cr, err := util.VersionCompare(s.GetVersion(), "5.6")
	if err != nil {
		return  err
	}
	if cr == -1 {
		//fmt.Println(s.Version, templateID)
		return s.ESAPIV2.SetSearchTemplate(templateID, body)
	}
	url := fmt.Sprintf("%s/_scripts/%s", s.GetEndpoint(), templateID)
	_, err = s.Request(nil, util.Verb_PUT, url, body)
	return err
}

func (s *ESAPIV5) DeleteSearchTemplate(templateID string) error {
	cr, err := util.VersionCompare(s.GetVersion(), "5.6")
	if err != nil {
		return  err
	}
	if cr == -1{
		//fmt.Println(s.Version, templateID)
		return s.ESAPIV2.DeleteSearchTemplate(templateID)
	}
	url := fmt.Sprintf("%s/_scripts/%s", s.GetEndpoint(), templateID)
	_, err = s.Request(nil, util.Verb_DELETE, url, nil)
	return err
}

func (s *ESAPIV5) FieldCaps(target string) ([]byte, error) {
	target=util.UrlEncode(target)
	cr, err := util.VersionCompare(s.GetVersion(), "5.4")
	if err != nil {
		return nil, err
	}
	if cr == -1 {
		return s.ESAPIV2.FieldCaps(target)
	}
	url := fmt.Sprintf("%s/%s/_field_caps?fields=*", s.GetEndpoint(), target)
	res, err := s.Request(nil, util.Verb_GET, url, nil)
	return res.Body, err
}
