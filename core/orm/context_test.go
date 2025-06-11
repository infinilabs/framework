package orm

import (
	"testing"
)

func TestGetCollapseField(t *testing.T) {
	// 1. nil context -> should return ""
	if got := GetCollapseField(nil); got != "" {
		t.Errorf("GetCollapseField(nil) = %q; want empty string", got)
	}

	// 2. context with no ctxCollapseFieldKey -> should return ""
	ctx := NewContext()
	if got := GetCollapseField(ctx); got != "" {
		t.Errorf("GetCollapseField(ctx without key) = %q; want empty string", got)
	}

	// 3. context with ctxCollapseFieldKey but value is nil -> should return ""
	ctx.SetValue(ctxCollapseFieldKey, nil)
	if got := GetCollapseField(ctx); got != "" {
		t.Errorf("GetCollapseField(ctx with key=nil) = %q; want empty string", got)
	}

	// 4. context with ctxCollapseFieldKey but value is not a string (e.g., int) -> should return ""
	ctx.SetValue(ctxCollapseFieldKey, 123)
	if got := GetCollapseField(ctx); got != "" {
		t.Errorf("GetCollapseField(ctx with key=int) = %q; want empty string", got)
	}

	// 5. context with ctxCollapseFieldKey with a string value -> should return that string
	want := "collapsed_field_value"
	ctx.SetValue(ctxCollapseFieldKey, want)
	if got := GetCollapseField(ctx); got != want {
		t.Errorf("GetCollapseField(ctx with string) = %q; want %q", got, want)
	}
}
