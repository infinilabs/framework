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

package opensearch

import (
	"fmt"
	"github.com/segmentio/encoding/json"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/elastic/adapter/elasticsearch"
	"strings"
)

type APIV1 struct {
	elasticsearch.ESAPIV8
}

func (s *APIV1) GetILMPolicy(target string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/_plugins/_ism/policies", s.GetEndpoint())
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

func (s *APIV1) PutILMPolicy(target string, policyConfig []byte) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return fmt.Errorf("policy name can not be empty")
	}
	url := fmt.Sprintf("%s/_plugins/_ism/policies/%s", s.GetEndpoint(), target)

	resp, err := s.Request(nil, util.Verb_PUT, url, policyConfig)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return fmt.Errorf(string(resp.Body))
	}

	return nil
}

func (s *APIV1) DeleteILMPolicy(target string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return fmt.Errorf("policy name can not be empty")
	}
	url := fmt.Sprintf("%s/_plugins/_ism/policies/%s", s.GetEndpoint(), target)
	resp, err := s.Request(nil, util.Verb_DELETE, url, nil)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf(string(resp.Body))
	}

	return nil
}