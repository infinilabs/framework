/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package elasticsearch

import (
	"fmt"
	"github.com/segmentio/encoding/json"
	"infini.sh/framework/core/util"
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
