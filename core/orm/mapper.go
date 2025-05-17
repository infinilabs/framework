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

package orm

import (
	"errors"
	"infini.sh/framework/core/util"
	"reflect"
)

func MapToStructWithJSONBytesUnmarshal(sourceBytes []byte, targetRef interface{}) error {

	// Ensure target is a pointer
	if reflect.ValueOf(targetRef).Kind() != reflect.Ptr {
		return errors.New("target must be a pointer")
	}

	// Unmarshal the JSON into the target struct
	if err := util.FromJSONBytes(sourceBytes, targetRef); err != nil {
		return err
	}

	return nil
}

func MapToStructWithMap(source map[string]interface{}, targetRef interface{}) error {
	sourceBytes, err := util.ToJSONBytes(source)
	if err != nil {
		return err
	}

	return MapToStructWithJSONBytesUnmarshal(sourceBytes, targetRef)
}
