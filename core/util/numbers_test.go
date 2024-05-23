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

