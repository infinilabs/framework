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

/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	"crypto/subtle"
	httprouter "infini.sh/framework/core/api/router"
	"net/http"
	"strings"

	"infini.sh/framework/core/model"
	configcommon "infini.sh/framework/modules/configs/common"
)

type BasicAuthFilter struct {
	Username string
	Password string
}

var loadManagedAccessTokenFromKeystore = func() (string, error) {
	return configcommon.LoadTokenFromKeystore(configcommon.AgentAccessTokenKeystoreKey)
}

// BasicAuth register api with basic auth
func BasicAuth(h httprouter.Handle, requiredUser, requiredPassword string) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		if validateManagedAccessToken(r) {
			h(w, r, ps)
			return
		}
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
		if validateManagedAccessToken(request) {
			handler(w, request)
			return
		}
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

func validateManagedAccessToken(req *http.Request) bool {
	tokenValue := ExtractBearerOrAPIToken(req)
	if tokenValue == "" {
		return false
	}
	expectedToken, err := loadManagedAccessTokenFromKeystore()
	if err != nil || expectedToken == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expectedToken), []byte(tokenValue)) == 1
}

func ValidateManagedAccessTokenRequest(req *http.Request) bool {
	return validateManagedAccessToken(req)
}

func ExtractBearerOrAPIToken(req *http.Request) string {
	if req == nil {
		return ""
	}
	tokenValue := strings.TrimSpace(req.Header.Get(model.API_TOKEN))
	if tokenValue != "" {
		return tokenValue
	}
	authHeader := strings.TrimSpace(req.Header.Get("Authorization"))
	if len(authHeader) < len("Bearer ")+1 || !strings.EqualFold(authHeader[:len("Bearer ")], "Bearer ") {
		return ""
	}
	return strings.TrimSpace(authHeader[len("Bearer "):])
}
