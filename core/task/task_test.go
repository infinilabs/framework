package task

import "testing"

func TestShouldSilenceStartupTaskErrorForMissingORMHandler(t *testing.T) {
	if !shouldSilenceStartupTaskError("ORM handler is not registered") {
		t.Fatal("expected missing ORM handler startup error to be silenced")
	}
}

func TestShouldSilenceStartupTaskErrorIgnoresOtherErrors(t *testing.T) {
	if shouldSilenceStartupTaskError("queue consumer is not registered") {
		t.Fatal("expected unrelated startup errors to remain visible")
	}
}
