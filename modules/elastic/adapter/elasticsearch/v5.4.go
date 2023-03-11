/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package elasticsearch

import (
	"fmt"
	"infini.sh/framework/core/util"
)

type ESAPIV5_4 struct {
	ESAPIV5
}

func (s *ESAPIV5_4) FieldCaps(target string) ([]byte, error) {
	target=util.UrlEncode(target)

	url := fmt.Sprintf("%s/%s/_field_caps?fields=*", s.GetEndpoint(), target)
	res, err := s.Request(nil, util.Verb_GET, url, nil)
	return res.Body, err
}