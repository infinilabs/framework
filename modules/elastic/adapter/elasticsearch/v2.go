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

package elasticsearch

import (
	"context"
	"errors"
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/elastic/adapter"
	"time"
)

type ESAPIV2 struct {
	ESAPIV0
}

func (s *ESAPIV2) ClearScroll(scrollId string) error {
	url := fmt.Sprintf("%s/_search/scroll", s.GetEndpoint())
	body := util.MustToJSONBytes(util.MapStr{"scroll_id": []string{scrollId}})

	resp, err := s.Request(context.Background(), util.Verb_DELETE, url, body)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return errors.New(string(resp.Body))
	}
	return nil
}

func (s *ESAPIV2) NextScroll(ctx *elastic.APIContext, scrollTime string, scrollId string) ([]byte, error) {

	url := fmt.Sprintf("%s/_search/scroll", s.GetEndpoint())
	body := util.MapStr{}
	body["scroll_id"] = scrollId
	body["scroll"] = scrollTime
	bodyBytes := util.MustToJSONBytes(body)

	resp, err := adapter.RequestTimeout(ctx, util.Verb_POST, url, bodyBytes, s.metadata, time.Duration(s.metadata.Config.RequestTimeout)*time.Second)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, errors.New(string(resp.Body))
	}

	if global.Env().IsDebug {
		log.Trace("next scroll,", url, "m,", string(resp.Body))
	}

	return resp.Body, nil
}
