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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/util"
	"strings"
)

type ESAPIV5 struct {
	ESAPIV2
}

func (s *ESAPIV5) NewScroll(indexNames string, scrollTime string, docBufferCount int, query string, slicedId, maxSlicedCount int, fields string) (scroll interface{}, err error) {
	url := fmt.Sprintf("%s/%s/_search?scroll=%s&size=%d", s.Config.Endpoint, indexNames, scrollTime, docBufferCount)

	var jsonBody []byte
	if len(query) > 0 || maxSlicedCount > 0 || len(fields) > 0 {
		queryBody := map[string]interface{}{}

		if len(fields) > 0 {
			if !strings.Contains(fields, ",") {
				return nil, errors.New("")
			} else {
				queryBody["_source"] = strings.Split(fields, ",")
			}
		}

		if len(query) > 0 {
			queryBody["query"] = map[string]interface{}{}
			queryBody["query"].(map[string]interface{})["query_string"] = map[string]interface{}{}
			queryBody["query"].(map[string]interface{})["query_string"].(map[string]interface{})["query"] = query
		}

		if maxSlicedCount > 1 {
			//log.Tracef("sliced scroll, %d of %d",slicedId,maxSlicedCount)
			queryBody["slice"] = map[string]interface{}{}
			queryBody["slice"].(map[string]interface{})["id"] = slicedId
			queryBody["slice"].(map[string]interface{})["max"] = maxSlicedCount
		}

		jsonArray, err := json.Marshal(queryBody)
		if err != nil {
			panic(err)
		} else {
			jsonBody = jsonArray
		}
	}

	resp, err := s.Request(util.Verb_POST, url, jsonBody)

	if err != nil {
		panic(err)
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, errors.New(string(resp.Body))
	}

	if err != nil {
		panic(err)
		return nil, err
	}

	scroll = &elastic.ScrollResponse{}
	err = json.Unmarshal(resp.Body, scroll)
	if err != nil {
		panic(err)
		return nil, err
	}

	return scroll, err
}

func (s *ESAPIV5) NextScroll(scrollTime string, scrollId string) (interface{}, error) {
	id := bytes.NewBufferString(scrollId)

	url := fmt.Sprintf("%s/_search/scroll?scroll=%s&scroll_id=%s", s.Config.Endpoint, scrollTime, id)
	resp, err := s.Request(util.Verb_GET, url, nil)

	if err != nil {
		panic(err)
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, errors.New(string(resp.Body))
	}

	scroll := &elastic.ScrollResponse{}
	err = json.Unmarshal(resp.Body, &scroll)
	if err != nil {
		panic(err)
		return nil, err
	}

	return scroll, nil
}

func (s *ESAPIV5) SetSearchTemplate(templateID string, body []byte) error {
	if s.Version < "5.6" {
		fmt.Println(s.Version, templateID)
		return s.ESAPIV0.SetSearchTemplate(templateID, body)
	}
	url := fmt.Sprintf("%s/_scripts/%s", s.Config.Endpoint, templateID)
	_, err := s.Request(util.Verb_PUT, url, body)
	return err
}

func (s *ESAPIV5) DeleteSearchTemplate(templateID string) error {
	if s.Version < "5.6" {
		fmt.Println(s.Version, templateID)
		return s.ESAPIV0.DeleteSearchTemplate(templateID)
	}
	url := fmt.Sprintf("%s/_scripts/%s", s.Config.Endpoint, templateID)
	_, err := s.Request(util.Verb_DELETE, url, nil)
	return err
}