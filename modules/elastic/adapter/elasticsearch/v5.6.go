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
	"context"
	"errors"
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

func (s *ESAPIV5_6) ClearScroll(scrollId string) error {
	url := fmt.Sprintf("%s/_search/scroll", s.GetEndpoint())
	body := util.MustToJSONBytes(util.MapStr{"scroll_id": scrollId})

	resp, err := s.Request(context.Background(), util.Verb_DELETE, url, body)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return errors.New(string(resp.Body))
	}
	return nil
}