/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package elasticsearch

import (
	"fmt"
	"github.com/segmentio/encoding/json"
	"infini.sh/framework/core/util"
)

type ESAPIV5_6 struct {
	ESAPIV5_4
}

func (c *ESAPIV5_6) RenderTemplate(body map[string]interface{}) ([]byte, error) {
	url := fmt.Sprintf("%s/_render/template", c.GetEndpoint())
	bytesBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	res, err := c.Request(nil, util.Verb_POST, url, bytesBody)
	return res.Body, err
}

func (c *ESAPIV5_6) SearchTemplate(body map[string]interface{}) ([]byte, error) {
	url := fmt.Sprintf("%s/_search/template", c.GetEndpoint())
	bytesBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	res, err := c.Request(nil, util.Verb_POST, url, bytesBody)
	return res.Body, err
}

func (s *ESAPIV5_6) DeleteSearchTemplate(templateID string) error {
	url := fmt.Sprintf("%s/_scripts/%s", s.GetEndpoint(), templateID)
	_, err := s.Request(nil, util.Verb_DELETE, url, nil)
	return err
}