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

package elastic

import (
	"testing"
)

func TestIndexDocument_GetStringFieldFromSource(t *testing.T) {
	tests := []struct {
		name     string
		doc      *IndexDocument
		field    string
		defaultV string
		want     string
	}{
		{
			name: "field exists and is a string",
			doc: &IndexDocument{
				Source: map[string]interface{}{
					"name": "test_value",
				},
			},
			field:    "name",
			defaultV: "default",
			want:     "test_value",
		},
		{
			name: "field exists but is not a string",
			doc: &IndexDocument{
				Source: map[string]interface{}{
					"count": 123,
				},
			},
			field:    "count",
			defaultV: "default",
			want:     "default",
		},
		{
			name: "field does not exist",
			doc: &IndexDocument{
				Source: map[string]interface{}{
					"name": "test_value",
				},
			},
			field:    "missing_field",
			defaultV: "default",
			want:     "default",
		},
		{
			name: "empty source map",
			doc: &IndexDocument{
				Source: map[string]interface{}{},
			},
			field:    "name",
			defaultV: "default",
			want:     "default",
		},
		{
			name: "nil source map",
			doc: &IndexDocument{
				Source: nil,
			},
			field:    "name",
			defaultV: "default",
			want:     "default",
		},
		{
			name: "empty string value",
			doc: &IndexDocument{
				Source: map[string]interface{}{
					"name": "",
				},
			},
			field:    "name",
			defaultV: "default",
			want:     "",
		},
		{
			name: "field exists and is a different type (bool)",
			doc: &IndexDocument{
				Source: map[string]interface{}{
					"enabled": true,
				},
			},
			field:    "enabled",
			defaultV: "default",
			want:     "default",
		},
		{
			name: "field exists and is a different type (slice)",
			doc: &IndexDocument{
				Source: map[string]interface{}{
					"tags": []string{"tag1", "tag2"},
				},
			},
			field:    "tags",
			defaultV: "default",
			want:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.doc.GetStringFieldFromSource(tt.field, tt.defaultV)
			if got != tt.want {
				t.Errorf("GetStringFieldFromSource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIndexDocument_TryGetStringFieldFromSource(t *testing.T) {
	tests := []struct {
		name     string
		doc      *IndexDocument
		fields   []string
		defaultV string
		want     string
	}{
		{
			name: "first matching string field is returned",
			doc: &IndexDocument{
				Source: map[string]interface{}{
					"title": "hello",
					"name":  "world",
				},
			},
			fields:   []string{"title", "name"},
			defaultV: "default",
			want:     "hello",
		},
		{
			name: "skip non-string value and return next string",
			doc: &IndexDocument{
				Source: map[string]interface{}{
					"title": 123,
					"name":  "world",
				},
			},
			fields:   []string{"title", "name"},
			defaultV: "default",
			want:     "world",
		},
		{
			name: "field exists but is not string",
			doc: &IndexDocument{
				Source: map[string]interface{}{
					"title": 123,
				},
			},
			fields:   []string{"title"},
			defaultV: "default",
			want:     "default",
		},
		{
			name: "no fields exist",
			doc: &IndexDocument{
				Source: map[string]interface{}{
					"other": "value",
				},
			},
			fields:   []string{"title", "name"},
			defaultV: "default",
			want:     "default",
		},
		{
			name: "empty fields list",
			doc: &IndexDocument{
				Source: map[string]interface{}{
					"title": "hello",
				},
			},
			fields:   []string{},
			defaultV: "default",
			want:     "default",
		},
		{
			name: "nil source map",
			doc: &IndexDocument{
				Source: nil,
			},
			fields:   []string{"title"},
			defaultV: "default",
			want:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.doc.TryGetStringFieldFromSource(tt.fields, tt.defaultV)
			if got != tt.want {
				t.Fatalf(
					"TryGetStringFieldFromSource() = %q, want %q",
					got,
					tt.want,
				)
			}
		})
	}
}
