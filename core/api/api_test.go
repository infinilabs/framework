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
	"net/http/httptest"
	"testing"
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
