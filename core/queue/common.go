/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package queue

import "time"

type ConsumerInstanceInfo struct {
	ID        string    `config:"id" json:"id,omitempty"`
	Timestamp time.Time `config:"timestamp" json:"timestamp,omitempty"`
}

