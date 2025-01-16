/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package orm

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMapToStructWithJSONUnmarshal(t *testing.T) {
	type SampleStruct struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	tests := []struct {
		name        string
		source      []byte
		target      interface{}
		expected    SampleStruct
		expectError bool
	}{
		{
			name:   "Valid JSON",
			source: []byte(`{"id":1,"name":"John","email":"john@example.com"}`),
			target: &SampleStruct{},
			expected: SampleStruct{
				ID:    1,
				Name:  "John",
				Email: "john@example.com",
			},
			expectError: false,
		},
		{
			name:        "Invalid JSON",
			source:      []byte(`{"id":1,"name":"John","email":"john@example.com"`), // Missing closing brace
			target:      &SampleStruct{},
			expected:    SampleStruct{},
			expectError: true,
		},
		{
			name:        "Target is not a pointer",
			source:      []byte(`{"id":1,"name":"John","email":"john@example.com"}`),
			target:      SampleStruct{},
			expected:    SampleStruct{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MapToStructWithJSONUnmarshal(tt.source, tt.target)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, *(tt.target.(*SampleStruct)))
			}
		})
	}
}
