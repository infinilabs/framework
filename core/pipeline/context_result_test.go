package pipeline

import (
	"errors"
	"testing"
)

func TestGetResultStateReturnsFinishedAfterStoppedCompletedRun(t *testing.T) {
	ctx := AcquireContext(PipelineConfigV2{})
	ctx.Started()
	ctx.Finished()
	ctx.Stopped()

	if got := ctx.GetResultState(); got != FINISHED {
		t.Fatalf("expected FINISHED result state, got %q", got)
	}
	if got := ctx.GetResultError(); got != "" {
		t.Fatalf("expected empty result error, got %q", got)
	}
}

func TestGetResultStateReturnsFailedAfterStoppedFailedRun(t *testing.T) {
	ctx := AcquireContext(PipelineConfigV2{})
	ctx.Started()
	ctx.Failed(errors.New("boom"))
	ctx.Stopped()

	if got := ctx.GetResultState(); got != FAILED {
		t.Fatalf("expected FAILED result state, got %q", got)
	}
	if got := ctx.GetResultError(); got == "" {
		t.Fatal("expected result error for failed run")
	}
}

func TestGetResultStateReturnsStoppedForManualStop(t *testing.T) {
	ctx := AcquireContext(PipelineConfigV2{})
	ctx.Started()
	ctx.Stopping()
	ctx.Stopped()

	if got := ctx.GetResultState(); got != STOPPED {
		t.Fatalf("expected STOPPED result state, got %q", got)
	}
	if got := ctx.GetResultError(); got != "" {
		t.Fatalf("expected empty result error, got %q", got)
	}
}

func TestGetResultErrorIncludesProcessErrors(t *testing.T) {
	ctx := AcquireContext(PipelineConfigV2{})
	ctx.Started()
	ctx.RecordError(errors.New("slice failed"))
	ctx.Finished()

	if got := ctx.GetResultError(); got == "" {
		t.Fatal("expected process error to be surfaced")
	}
}
