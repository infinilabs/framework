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
	"fmt"
	log "github.com/cihub/seelog"
	"strings"

	"errors"
	"infini.sh/framework/core/global"
)

type ESAPIV6 struct {
	ESAPIV5_6
}

func (c *ESAPIV6) InitDefaultTemplate(templateName,indexPrefix string) {
	c.initTemplate(templateName,indexPrefix)
}

func (c *ESAPIV6) getDefaultTemplate(indexPrefix string) string {
	template := `
{
"index_patterns": "%s*",
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
	return fmt.Sprintf(template, indexPrefix, 1, TypeName6)
}

const TypeName6 = "doc"

func (c *ESAPIV6) initTemplate(templateName,indexPrefix string) {
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
		if global.Env().IsDebug {
			log.Trace("template: ", template)
		}
		res, err := c.PutTemplate(templateName, []byte(template))
		if err != nil {
			panic(err)
		}

		if strings.Contains(string(res), "error") {
			panic(errors.New(string(res)))
		}
		if global.Env().IsDebug {
			log.Trace("put template response, ", string(res))
		}
	}
	log.Debugf("elasticsearch template successful initialized")

}
