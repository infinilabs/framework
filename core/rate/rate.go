/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package rate

import (
	"sync"

	"golang.org/x/time/rate"
)

var raters = make(map[string]map[string]*rate.Limiter)
var mu sync.Mutex

func GetRaterWithDefine(category,key string, maxQPS int) *rate.Limiter {
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
