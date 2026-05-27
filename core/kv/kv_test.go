package kv

import "testing"

func TestHasStore(t *testing.T) {
	previousHandler := handler
	previousStores := stores
	defer func() {
		handler = previousHandler
		stores = previousStores
	}()

	handler = nil
	stores = nil

	if HasStore("elastic") {
		t.Fatal("expected store lookup to be false before registration")
	}

	Register("elastic", nil)

	if !HasStore("elastic") {
		t.Fatal("expected store lookup to be true after registration")
	}
}
