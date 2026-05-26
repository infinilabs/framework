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
