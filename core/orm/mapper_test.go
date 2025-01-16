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
