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

package util

import (
	"testing"
)

func TestStringToFloat(t *testing.T) {
	tests := []struct {
		input          string
		expectedOutput float64
		expectError    bool
	}{
		{"3.14159", 3.14159, false},
		{"123.456", 123.456, false},
		{"-789.123", -789.123, false},
		{"0", 0, false},
		{"", 0, true},
		{"invalid", 0, true},
	}

	for _, test := range tests {
		output, err := StringToFloat(test.input)
		if (err != nil) != test.expectError {
			t.Errorf("StringToFloat(%q) expected error: %v, got: %v", test.input, test.expectError, err)
		}
		if output != test.expectedOutput {
			t.Errorf("StringToFloat(%q) expected: %v, got: %v", test.input, test.expectedOutput, output)
		}
	}
}
