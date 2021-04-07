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
