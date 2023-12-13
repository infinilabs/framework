/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	httprouter "infini.sh/framework/core/api/router"
	"net/http"
)

type BasicAuthFilter struct {
	Username string
	Password string
}

// BasicAuth register api with basic auth
func BasicAuth(h httprouter.Handle, requiredUser, requiredPassword string) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// Get the Basic Authentication credentials
		user, password, hasAuth := r.BasicAuth()

		if hasAuth && user == requiredUser && password == requiredPassword {
			// Delegate request to the given handle
			h(w, r, ps)
		} else {
			// Request Basic Authentication otherwise
			w.Header().Set("WWW-Authenticate", "Basic realm=Restricted")
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		}
	}
}

func (filter *BasicAuthFilter) FilterHttpRouter(pattern string, h httprouter.Handle) httprouter.Handle {
	return BasicAuth(h, filter.Username, filter.Password)
}

func (filter *BasicAuthFilter) FilterHttpHandlerFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, request *http.Request) {
		// Get the Basic Authentication credentials
		user, password, hasAuth := request.BasicAuth()
		if hasAuth && user == filter.Username && password == filter.Password {
			// Delegate request to the given handle
			handler(w, request)
			return
		}
		// Request Basic Authentication otherwise
		w.Header().Set("WWW-Authenticate", "Basic realm=Restricted")
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
	}
}
