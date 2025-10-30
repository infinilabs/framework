/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package util

import "github.com/emirpasic/gods/sets/hashset"

func IsSuperset(a, b *hashset.Set) bool {
	for _, item := range b.Values() {
		if !a.Contains(item) {
			return false
		}
	}
	return true
}
