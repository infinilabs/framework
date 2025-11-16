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

package http_filters

import (
	"bufio"
	"bytes"
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/global"
	"io"
	"net"
	"net/http"
	"time"
)

func init() {
	api.RegisterUIFilter(&LoggingFilter{})
}

type LoggingFilter struct{}

func (f *LoggingFilter) GetPriority() int {
	// Lower priority values execute first (higher precedence)
	return 0
}

func (f *LoggingFilter) ApplyFilter(
	method string,
	pattern string,
	options *api.HandlerOptions,
	next httprouter.Handle,
) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

		start := time.Now()

		// Step 1: Capture request body if in debug mode
		var requestBody string
		if global.Env().IsDebug {
			if r.Body != nil {
				bodyBytes, err := io.ReadAll(r.Body)
				if err == nil {
					requestBody = string(bodyBytes)
				} else {
					requestBody = "[error reading request body]"
				}
				// Restore body for next handler
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}
		}

		// Step 2: Wrap ResponseWriter to capture status and response body
		recorder := &responseRecorder{
			ResponseWriter: w,
			body:           &bytes.Buffer{},
		}

		// Step 3: Call the actual handler
		next(recorder, r, ps)

		// Step 4: After handler finishes
		elapsed := time.Since(start)

		// Get client IP
		clientIP := r.RemoteAddr
		if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
			clientIP = ip
		}

		userAgent := r.UserAgent()

		var resBody string

		if global.Env().IsDebug {
			resBody = recorder.body.String()
		}

		// Step 5: Format and log the data
		log.Infof(`%s - "%s %s %s" %d %s "%s" "%s" "%s"`,
			clientIP,
			r.Method,
			r.RequestURI,
			r.Proto,
			recorder.statusCode,
			elapsed.String(),
			userAgent,
			truncate(requestBody, 1000), // optional truncation to avoid massive logs
			truncate(resBody, 1000),     // optional truncation
		)
	}
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max] + "...[truncated]"
	}
	return s
}

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (rr *responseRecorder) WriteHeader(code int) {
	if rr.statusCode <= 0 {
		rr.ResponseWriter.WriteHeader(code)
		rr.statusCode = code
	}
}

func (rr *responseRecorder) Write(b []byte) (int, error) {
	rr.body.Write(b) // Capture the response body
	return rr.ResponseWriter.Write(b)
}

func (rr *responseRecorder) Flush() {
	if f, ok := rr.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (rr *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := rr.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not support Hijacker")
}

func (rr *responseRecorder) CloseNotify() <-chan bool {
	if cn, ok := rr.ResponseWriter.(http.CloseNotifier); ok {
		return cn.CloseNotify()
	}
	// fallback: return nil channel (won't notify properly)
	return make(<-chan bool)
}
