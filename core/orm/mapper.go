/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package orm

import (
	"errors"
	"infini.sh/framework/core/util"
	"reflect"
)

func MapToStructWithJSONUnmarshal(source []byte, targetRef interface{}) error {

	// Ensure target is a pointer
	if reflect.ValueOf(targetRef).Kind() != reflect.Ptr {
		return errors.New("target must be a pointer")
	}

	// Unmarshal the JSON into the target struct
	if err := util.FromJSONBytes(source, targetRef); err != nil {
		return err
	}

	return nil
}
