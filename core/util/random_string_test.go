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

package util

import (
	"strings"
	"sync"
	"testing"
)

// TestConcurrentRandomStrings relies on go test -race to detect data races on the shared random source.
// The assertions check string lengths and character rules.
func TestConcurrentRandomStrings(t *testing.T) {
	var workers sync.WaitGroup
	for i := 0; i < 16; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for j := 0; j < 100; j++ {
				plain := GenerateRandomString(12)
				if len(plain) != 12 || strings.Trim(plain, simpleChars) != "" {
					t.Errorf("invalid random string %q", plain)
				}
				password := GenerateSecureString(12)
				if len(password) != 12 || !ValidateSecure(password) {
					t.Error("generated password violates its policy")
				}
			}
		}()
	}
	workers.Wait()
}
