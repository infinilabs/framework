/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package util

import (
	"github.com/r3labs/diff/v2"
)

func DiffTwoObject(a, b interface{}) (diff.Changelog, error) {
	return diff.Diff(a, b, diff.DisableStructValues(), diff.AllowTypeMismatch(true),diff.SliceOrdering(false))
}
