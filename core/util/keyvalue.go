/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package util

import (
	"sort"
)

type KeyValue struct {
	Key   string
	Value int
}

func SortMapStrIntToKV(data map[string]int)[]KeyValue  {
	var keyValuePairs []KeyValue

	for k, v := range data {
		keyValuePairs = append(keyValuePairs, KeyValue{k, v})
	}

	sort.Slice(keyValuePairs, func(i, j int) bool {
		return keyValuePairs[i].Value > keyValuePairs[j].Value
	})

	return keyValuePairs
}