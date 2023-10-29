/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package util

import (
	"sort"
)

type KeyValue struct {
	Key   string
	Value int64
	Payload interface{}
}

func SortMapStrIntToKV(data map[string]int) []KeyValue {
	var keyValuePairs []KeyValue

	for k, v := range data {
		keyValuePairs = append(keyValuePairs, KeyValue{k, int64(v),nil})
	}

	keyValuePairs = SortKeyValueArray(keyValuePairs,false)

	return keyValuePairs
}

//sort by value desc by default
func SortKeyValueArray(keyValuePairs []KeyValue,revers bool) []KeyValue {
	sort.Slice(keyValuePairs, func(i, j int) bool {
		if revers{
			return keyValuePairs[i].Value < keyValuePairs[j].Value
		}else{
			return keyValuePairs[i].Value > keyValuePairs[j].Value
		}
	})
	return keyValuePairs
}
