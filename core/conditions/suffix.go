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

package conditions

import (
	"errors"
	"fmt"

	logger "github.com/cihub/seelog"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
)

// Suffix is a Condition for checking field if the field whether end with specify string or not.
type Suffix struct {
	Field string
	Data  string
}

func NewSuffixCondition(fields map[string]interface{}) (hasFieldsCondition Suffix, err error) {
	c := Suffix{}
	if len(fields) == 1 {
		for field, value := range util.MapStr(fields).Flatten() {
			c.Field = field
			var ok bool
			c.Data, ok = value.(string)
			if !ok {
				return c, errors.New("invalid in parameters")
			}
			break
		}
	} else {
		return c, errors.New("invalid in parameters")
	}
	return c, nil
}

// Check determines whether the given event matches this condition
func (c Suffix) Check(event ValuesMap) bool {
	isDebug := global.Env().IsDebug
	value, err := event.GetValue(c.Field)
	if err != nil {
		if isDebug {
			logger.Warnf("'%s' does not exist: %s", c.Field, err)
		}
		return false
	}
	str, ok := value.(string)
	if ok {
		if util.SuffixStr(str, c.Data) {
			return true
		}
	}

	if isDebug {
		logger.Warnf("'%s' does not has expected suffix: %v", c.Field, value)
	}
	return false
}

func (c Suffix) String() string {
	return fmt.Sprintf("field: %v suffix: %v", c.Field, c.Field)
}
