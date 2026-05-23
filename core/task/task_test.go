package task

import "testing"

func TestShouldSilenceStartupTaskErrorWithoutORMHandler(t *testing.T) {
	if !shouldSilenceStartupTaskError("ORM handler is not registered") {
		t.Fatal("expected missing ORM handler startup error to be silenced")
	}
}

func TestShouldSilenceStartupTaskErrorIgnoresOtherMessages(t *testing.T) {
	if shouldSilenceStartupTaskError("failed to scheduled interval task") {
		t.Fatal("expected unrelated task error to keep normal logging")
	}
}
