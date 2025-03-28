/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
)

var (
	allowOriginFuncs sync.Map
)

// RegisterAllowOriginFunc registers a function to check if the origin is allowed.
// The key is used to identify the function.
func RegisterAllowOriginFunc(key string, fn AllowOriginFunc) {
	if _, exists := allowOriginFuncs.Load(key); exists {
		panic("key already exists, maybe you can remove it first")
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

type CorsConfig struct {
	AllowHeaders     []string // Allowed headers
	AllowMethods     []string // Allowed methods
	AllowCredentials bool     // Allow credentials
}

// CORSMiddleware is a middleware that handles CORS requests.
func CORSMiddleware(next http.Handler, config CorsConfig) http.Handler {
	if len(config.AllowMethods) == 0 {
		config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	}
	if len(config.AllowHeaders) == 0 {
		config.AllowHeaders = []string{"Authorization", "Content-Type"}
	}
	allowHeaders := strings.Join(config.AllowHeaders, ", ")
	allowMethods := strings.Join(config.AllowMethods, ", ")
	allowCredentials := fmt.Sprintf("%v", config.AllowCredentials)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && (r.Method == http.MethodOptions || isAllowedOrigin(origin, r)) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", allowMethods)         //eg: "GET, POST, PUT, DELETE, OPTIONS"
			w.Header().Set("Access-Control-Allow-Headers", allowHeaders)         //eg: "Authorization, Content-Type, X-API-TOKEN, APP-INTEGRATION-ID"
			w.Header().Set("Access-Control-Allow-Credentials", allowCredentials) //eg: "true"
			// Handle preflight (OPTIONS) requests
			if r.Method == "OPTIONS" && r.Header.Get("Access-Control-Request-Method") != "" {
				// Respond with 200 OK for OPTIONS requests
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
