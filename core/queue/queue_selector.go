/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package queue

import "fmt"

type QueueSelector struct {
	Labels map[string]interface{} `config:"labels,omitempty"`
	Ids    []string               `config:"ids,omitempty"`
	Keys   []string               `config:"keys,omitempty"`
}

func (s *QueueSelector) ToString() string {
	return fmt.Sprintf("ids:%v, keys:%v, labels:%v", s.Ids, s.Keys, s.Labels)
}

