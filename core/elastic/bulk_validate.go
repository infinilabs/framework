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

/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package elastic

import (
	"strings"

	log "github.com/cihub/seelog"
	"github.com/rubyniu105/framework/core/util"
)

func ValidateBulkRequest(where, body string) {
	stringLines := strings.Split(body, "\n")
	if len(stringLines) == 0 {
		log.Error("invalid json lines, empty")
		return
	}
	obj := map[string]interface{}{}
	for _, v := range stringLines {
		err := util.FromJSONBytes([]byte(v), &obj)
		if err != nil {
			log.Error("invalid json,", where, ",", util.SubString(v, 0, 512), err)
			break
		}
	}
}
