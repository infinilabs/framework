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
