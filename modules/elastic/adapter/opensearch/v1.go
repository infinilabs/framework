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