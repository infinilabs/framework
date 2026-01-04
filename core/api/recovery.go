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
	"net/http"
	"runtime"
	"runtime/debug"

	log "github.com/cihub/seelog"
	"github.com/segmentio/encoding/json"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
)

type recoveryHandler struct {
	handler    http.Handler
	printStack bool
}

// RecoveryOption provides a functional approach to define
// configuration for a handler; such as setting the logging
// whether or not to print stack traces on panic.
type RecoveryOption func(http.Handler)

func parseRecoveryOptions(h http.Handler, opts ...RecoveryOption) http.Handler {
	for _, option := range opts {
		option(h)
	}

	return h
}

// RecoveryHandler is HTTP middleware that recovers from a panic,
// logs the panic, writes http.StatusInternalServerError, and
// continues to the next handler.
func RecoveryHandler(opts ...RecoveryOption) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		r := &recoveryHandler{handler: h, printStack: true}
		return parseRecoveryOptions(r, opts...)
	}
}

// PrintRecoveryStack is a functional option to enable
// or disable printing stack traces on panic.
func PrintRecoveryStack(print bool) RecoveryOption {
	return func(h http.Handler) {
		r := h.(*recoveryHandler)
		r.printStack = print
	}
}

func (h recoveryHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			code := http.StatusInternalServerError
			var v string
			switch r.(type) {
			case error:
				err := r.(error)
				v = err.Error()
				code = errors.HTTPCode(err)
			case runtime.Error:
				v = r.(runtime.Error).Error()
			case string:
				v = r.(string)
			}

			errObj := util.MapStr{
				"status": code,
				"reason": v,
			}

			if global.Env().SystemConfig.APIConfig.VerboseErrorRootCause {
				errObj["root_cause"] = string(debug.Stack())
			}

			var payload = util.MapStr{
				"error": errObj,
			}
			payloadBytes, jerr := json.Marshal(payload)
			if jerr != nil {
				log.Error(jerr)
			}

			w.WriteHeader(code)
			w.Write(payloadBytes)

			if global.Env().IsDebug {
				h.log(true, v)
			}
		}
	}()

	h.handler.ServeHTTP(w, req)
}

func (h recoveryHandler) log(printStack bool, v ...interface{}) {
	log.Error(v)
	if printStack {
		stack := string(debug.Stack())
		log.Error(stack)
	}
}
