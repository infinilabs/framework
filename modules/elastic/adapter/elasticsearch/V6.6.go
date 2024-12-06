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

/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package elasticsearch

import (
	"fmt"
	"github.com/rubyniu105/framework/core/util"
	"github.com/segmentio/encoding/json"
	"strings"
)

type ESAPIV6_6 struct {
	ESAPIV6
}

func (s *ESAPIV6_6) UpdateMapping(indexName string, docType string, mappings []byte) ([]byte, error) {
	indexName = util.UrlEncode(indexName)
	if docType == "" {
		docType = TypeName6
	}

	url := fmt.Sprintf("%s/%s/%s/_mapping", s.GetEndpoint(), indexName, docType)

	resp, err := s.Request(nil, util.Verb_POST, url, mappings)

	if err != nil {
		panic(err)
	}

	return resp.Body, nil
}

func (s *ESAPIV6_6) GetILMPolicy(target string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/_ilm/policy", s.GetEndpoint())
	if target != "" {
		url = fmt.Sprintf("%s/%s", url, target)
	}
	resp, err := s.Request(nil, util.Verb_GET, url, nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf(string(resp.Body))
	}

	data := map[string]interface{}{}
	err = json.Unmarshal(resp.Body, &data)
	return data, err
}

func (s *ESAPIV6_6) PutILMPolicy(target string, policyConfig []byte) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return fmt.Errorf("policy name can not be empty")
	}
	url := fmt.Sprintf("%s/_ilm/policy/%s", s.GetEndpoint(), target)

	resp, err := s.Request(nil, util.Verb_PUT, url, policyConfig)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf(string(resp.Body))
	}

	return nil
}

func (s *ESAPIV6_6) DeleteILMPolicy(target string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return fmt.Errorf("policy name can not be empty")
	}
	url := fmt.Sprintf("%s/_ilm/policy/%s", s.GetEndpoint(), target)
	resp, err := s.Request(nil, util.Verb_DELETE, url, nil)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf(string(resp.Body))
	}

	return nil
}
