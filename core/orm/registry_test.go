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

import "testing"

type testSchemaAlpha struct{}
type testSchemaBeta struct{}

func TestRegisterSchemaWithIndexNameDeduplicatesSameSchema(t *testing.T) {
	original := registeredSchemas
	registeredSchemas = nil
	t.Cleanup(func() {
		registeredSchemas = original
	})

	if err := RegisterSchemaWithIndexName(testSchemaAlpha{}, "test-index"); err != nil {
		t.Fatalf("expected first registration to succeed, got %v", err)
	}
	if err := RegisterSchemaWithIndexName(&testSchemaAlpha{}, "test-index"); err != nil {
		t.Fatalf("expected duplicate registration to be ignored, got %v", err)
	}
	if len(registeredSchemas) != 1 {
		t.Fatalf("expected exactly one registered schema, got %d", len(registeredSchemas))
	}
}

func TestRegisterSchemaWithIndexNameRejectsDifferentSchemaForSameIndex(t *testing.T) {
	original := registeredSchemas
	registeredSchemas = nil
	t.Cleanup(func() {
		registeredSchemas = original
	})

	if err := RegisterSchemaWithIndexName(testSchemaAlpha{}, "test-index"); err != nil {
		t.Fatalf("expected first registration to succeed, got %v", err)
	}
	if err := RegisterSchemaWithIndexName(testSchemaBeta{}, "test-index"); err == nil {
		t.Fatal("expected conflicting registration to fail")
	}
}
