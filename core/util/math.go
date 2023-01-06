/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package util

// Abs returns the absolute value of x.
func AbsInt64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func AbsInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
