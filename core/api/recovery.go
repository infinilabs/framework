/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	log "github.com/cihub/seelog"
	"github.com/segmentio/encoding/json"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
	"net/http"
	"runtime"
	"runtime/debug"
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
			w.WriteHeader(http.StatusInternalServerError)

			var v string
			switch r.(type) {
			case error:
				v = r.(error).Error()
			case runtime.Error:
				v = r.(runtime.Error).Error()
			case string:
				v = r.(string)
			}

			errObj:=util.MapStr{
				"status":http.StatusInternalServerError,
				"reason":v,
			}

			if global.Env().SystemConfig.APIConfig.VerboseErrorRootCause{
				errObj["root_cause"]=string(debug.Stack())
			}

			var payload = util.MapStr{
				"error": errObj,
			}
			payloadBytes, jerr := json.Marshal(payload)
			if jerr != nil {
				log.Error(jerr)
			}
			w.Write(payloadBytes)

			if global.Env().IsDebug{
				h.log(v)
			}
		}
	}()

	h.handler.ServeHTTP(w, req)
}

func (h recoveryHandler) log(v ...interface{}) {
	log.Error(v)
	if h.printStack {
		stack := string(debug.Stack())
		log.Error(stack)
	}
}
