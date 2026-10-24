/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

/* for_each under a dry-run replay: the dryrun harness binds one decoded
 * record and no message batch. The host must adopt the bound record (the
 * zero-decode typed entry) and actually run the sub-chain on it — before
 * this path existed, for_each silently no-opped in every dry-run, so
 * migrated legacy chains replayed as "no effect". */

import (
	"testing"

	"infini.sh/framework/core/event"
	"infini.sh/framework/core/pipeline"
)

func TestForEach_DryRunAdoptsBoundRecord(t *testing.T) {
	p := buildForEach(t, nil)

	rec := event.AcquireEvent()
	rec.Fields = map[string]interface{}{"message": "hello"}
	ctx := &pipeline.Context{Context: t.Context()}
	ctx.Set(pipeline.RecordContextKey, rec)
	sink := &[]pipeline.DryRunWrite{}
	pipeline.EnableDryRun(ctx, sink, true)

	if err := p.Process(ctx); err != nil {
		t.Fatal(err)
	}

	if got := rec.Fields["message"]; got != "PROCESSED:hello" {
		t.Fatalf("sub-chain did not run on the adopted record: message=%v", got)
	}
	// the harness's record must still be the bound record afterwards
	bound, ok := pipeline.CurrentRecord(ctx)
	if !ok || bound != rec {
		t.Fatal("bound record not restored after dry-run adoption")
	}
}

// outside dry-run the historical behavior stands: no batch, no-op
func TestForEach_NoBatchOutsideDryRunSkips(t *testing.T) {
	p := buildForEach(t, nil)

	rec := event.AcquireEvent()
	rec.Fields = map[string]interface{}{"message": "hello"}
	ctx := &pipeline.Context{Context: t.Context()}
	ctx.Set(pipeline.RecordContextKey, rec)

	if err := p.Process(ctx); err != nil {
		t.Fatal(err)
	}
	if got := rec.Fields["message"]; got != "hello" {
		t.Fatalf("non-dry-run run must not touch the bound record: message=%v", got)
	}
}
