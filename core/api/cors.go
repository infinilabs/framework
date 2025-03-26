/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	"net/http"
	"sync"
)

var (
	allowOriginFuncs sync.Map
)

// RegisterAllowOriginFunc registers a function to check if the origin is allowed.
// The key is used to identify the function.
func RegisterAllowOriginFunc(key string, fn AllowOriginFunc) {
	if _, exists := allowOriginFuncs.Load(key); exists {
		panic("key already exists, my be you can remove it first")
	}
	allowOriginFuncs.Store(key, fn)
}

// RemoveAllowOriginFunc removes the function to check if the origin is allowed.
func RemoveAllowOriginFunc(key string) {
	allowOriginFuncs.Delete(key)
}

// AllowOriginFunc is a function that checks if the origin is allowed.
type AllowOriginFunc func(origin string, req *http.Request) bool

func isAllowedOrigin(origin string, req *http.Request) bool {
	isAllowed := false
	allowOriginFuncs.Range(func(key, value interface{}) bool {
		if fn, ok := value.(AllowOriginFunc); ok {
			//note: hear we pass the request to the function to allow the function to implement more complex logic
			if fn != nil && fn(origin, req) {
				isAllowed = true
				// break the loop
				return false
			}
		}
		return true
	})
	return isAllowed
}

// CORSMiddleware is a middleware that handles CORS requests.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && isAllowedOrigin(origin, r) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-API-TOKEN, APP-INTEGRATION-ID")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			// Handle preflight (OPTIONS) requests
			if r.Method == "OPTIONS" {
				// Respond with 200 OK for OPTIONS requests
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
