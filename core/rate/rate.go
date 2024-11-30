// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package rate

import (
	"sync"
	"golang.org/x/time/rate"
	"time"
)

var raters = make(map[string]map[string]*rate.Limiter)
var mu sync.Mutex

func GetRateLimiterPerSecond(category,key string, maxQPS int) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	_,ok:=raters[category]
	if !ok{
		raters[category]=map[string]*rate.Limiter{}
	}

	limiter, exists := raters[category][key]
	if !exists {
		limiter = rate.NewLimiter(rate.Limit(maxQPS), maxQPS)
		raters[category][key] = limiter
	}

	return limiter
}

func GetRateLimiter(category,key string,limit,burstLimit int,interval time.Duration) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	if burstLimit<limit{
		burstLimit=limit
	}

	_,ok:=raters[category]
	if !ok{
		raters[category]=map[string]*rate.Limiter{}
	}

	limiter, exists := raters[category][key]
	if !exists {
		rt := rate.Every(interval / time.Duration(limit))
		limiter = rate.NewLimiter(rt, burstLimit)
		raters[category][key] = limiter
	}

	return limiter
}
