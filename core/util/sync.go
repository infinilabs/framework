/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package util

import "sync"

func MapLength(m *sync.Map) int {
	len := 0
	m.Range(func(k, v interface{}) bool {
		len++
		return true
	})
	return len
}
