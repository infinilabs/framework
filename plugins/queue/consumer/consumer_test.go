package consumer

import (
	"context"
	"testing"

	"infini.sh/framework/core/pipeline"
)

func TestIsExpectedQueueShutdownPanicRequiresShutdownSignal(t *testing.T) {
	ctx := &pipeline.Context{Context: context.Background()}
	if isExpectedQueueShutdownPanic("module closed", ctx) {
		t.Fatal("expected module closed without shutdown or cancellation to remain an error")
	}
	if isExpectedQueueShutdownPanic("boom", ctx) {
		t.Fatal("expected unrelated panic message to remain an error")
	}
}

func TestIsExpectedQueueShutdownPanicTreatsCanceledContextAsExpected(t *testing.T) {
	baseCtx, cancel := context.WithCancel(context.Background())
	ctx := &pipeline.Context{Context: baseCtx}
	cancel()

	if !isExpectedQueueShutdownPanic("module closed", ctx) {
		t.Fatal("expected module closed during context cancellation to be treated as shutdown noise")
	}
}

func TestGetRecoveredMessage(t *testing.T) {
	if got := getRecoveredMessage("boom"); got != "boom" {
		t.Fatalf("unexpected string recovery message: %q", got)
	}
	if got := getRecoveredMessage(context.Canceled); got != context.Canceled.Error() {
		t.Fatalf("unexpected error recovery message: %q", got)
	}
}
