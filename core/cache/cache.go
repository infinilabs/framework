/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package cache

import "time"

type ICache interface {
	Exists(key string) (bool, error)
	Get(key string) ([]byte, bool)
	SetTTL(key string, data []byte, ttl time.Duration) error
	Set(key string, data []byte) error
}

