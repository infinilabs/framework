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
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/config"
)

func TestStripPrefix(t *testing.T) {
	// Define the test cases in a table
	testCases := []struct {
		name                  string // A descriptive name for the test case
		prefix                string // The prefix to pass to our function
		requestPath           string // The path of the incoming mock request
		expectedPathInHandler string // The path the final handler should receive
	}{
		{
			name:                  "should strip prefix when present",
			prefix:                "/api",
			requestPath:           "/api/users",
			expectedPathInHandler: "/users",
		},
		{
			name:                  "should pass through when prefix does not match",
			prefix:                "/api",
			requestPath:           "/healthz",
			expectedPathInHandler: "/healthz", // Key behavior of this custom function
		},
		{
			name:                  "should handle path being prefix with trailing slash",
			prefix:                "/v1",
			requestPath:           "/v1/",
			expectedPathInHandler: "/", // Standard http.StripPrefix behavior
		},
		{
			name:                  "should do nothing for an empty prefix",
			prefix:                "",
			requestPath:           "/users",
			expectedPathInHandler: "/users",
		},
		{
			name:                  "should not match partial prefix",
			prefix:                "/api",
			requestPath:           "/apiv2/users",
			expectedPathInHandler: "/apiv2/users", // Should pass through unmodified
		},
	}

	// Iterate over the test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// A variable to capture the path seen by the final handler
			var receivedPath string

			// Create a simple, final handler that records the path it receives
			finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedPath = r.URL.Path
				w.WriteHeader(http.StatusOK) // Send a success response
			})

			// Create the handler we are testing by wrapping our final handler
			handlerToTest := StripPrefix(tc.prefix, finalHandler)

			// Create a new mock request with the specified path
			req := httptest.NewRequest("GET", tc.requestPath, nil)

			// Create a ResponseRecorder to record the response
			rr := httptest.NewRecorder()

			// Serve the request using our handler
			handlerToTest.ServeHTTP(rr, req)

			// Assertions
			// 1. Check if the status code is what we expect (always OK in these cases)
			if status := rr.Code; status != http.StatusOK {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, http.StatusOK)
			}

			// 2. Check if the final handler received the correctly modified path
			if receivedPath != tc.expectedPathInHandler {
				t.Errorf("final handler received wrong path: got %q want %q",
					receivedPath, tc.expectedPathInHandler)
			}
		})
	}
}

func TestServeRegisteredAPIRequest(t *testing.T) {
	path := fmt.Sprintf("/__copilot_test__/api/%s/:id", t.Name())
	HandleAPIMethod(GET, path, func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(ps.MustGetParameter("id") + ":" + req.URL.Query().Get("q")))
	})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("%s/value?q=ok", fmt.Sprintf("/__copilot_test__/api/%s", t.Name())), nil)
	recorder := httptest.NewRecorder()

	ServeRegisteredAPIRequest(recorder, req)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if recorder.Body.String() != "value:ok" {
		t.Fatalf("unexpected body: %s", recorder.Body.String())
	}
}

func TestServeRegisteredAPIRequestAllowsNestedDispatch(t *testing.T) {
	innerPath := fmt.Sprintf("/__copilot_test__/api/%s/inner", t.Name())
	outerPath := fmt.Sprintf("/__copilot_test__/api/%s/outer", t.Name())

	HandleAPIMethod(GET, innerPath, func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("inner-ok"))
	})
	HandleAPIMethod(GET, outerPath, func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		innerReq := httptest.NewRequest(http.MethodGet, innerPath, nil)
		innerRecorder := httptest.NewRecorder()
		ServeRegisteredAPIRequest(innerRecorder, innerReq)
		w.WriteHeader(innerRecorder.Code)
		_, _ = w.Write(innerRecorder.Body.Bytes())
	})

	req := httptest.NewRequest(http.MethodGet, outerPath, nil)
	recorder := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)
		ServeRegisteredAPIRequest(recorder, req)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("nested dispatch timed out")
	}

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if recorder.Body.String() != "inner-ok" {
		t.Fatalf("unexpected body: %s", recorder.Body.String())
	}
}

func TestSyncRuntimePublishAddressUsesActualListenAddressWhenUnset(t *testing.T) {
	cfg := config.NetworkConfig{}

	syncRuntimePublishAddress(&cfg, "0.0.0.0:2901")

	if cfg.Publish != "0.0.0.0:2901" {
		t.Fatalf("expected runtime publish address to be updated, got %q", cfg.Publish)
	}
}

func TestSyncRuntimePublishAddressPreservesExplicitPublishAddress(t *testing.T) {
	cfg := config.NetworkConfig{Publish: "gateway.example:8443"}

	syncRuntimePublishAddress(&cfg, "0.0.0.0:2901")

	if cfg.Publish != "gateway.example:8443" {
		t.Fatalf("expected explicit publish address to be preserved, got %q", cfg.Publish)
	}
}
