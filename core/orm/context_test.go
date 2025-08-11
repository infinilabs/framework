package orm

import (
	"context"
	"infini.sh/framework/core/param"
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

type ctxKey string

func TestSameKeyShadowing(t *testing.T) {
	key := ctxKey("userID")
	parent := context.Background()

	ctx1 := context.WithValue(parent, key, "first")
	ctx2 := context.WithValue(ctx1, key, "second")
	ctx3 := context.WithValue(ctx2, key, "third")

	// Nearest value wins
	if got := ctx3.Value(key); got != "third" {
		t.Errorf("expected third, got %v", got)
	}

	if got := ctx2.Value(key); got != "second" {
		t.Errorf("expected second, got %v", got)
	}

	if got := ctx1.Value(key); got != "first" {
		t.Errorf("expected first, got %v", got)
	}

	// Parent has nothing
	if got := parent.Value(key); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

type testKeyType string

const keyUserID param.ParaKey = "user_id"

func TestContextPriority(t *testing.T) {
	// Base context with context.Context carrying value
	baseCtx := context.WithValue(context.Background(), keyUserID, "fromContext")

	// Our extended ORM context
	ctx := NewContextWithParent(baseCtx)

	// Set a different value in param.Parameters
	ctx.Set(keyUserID, "fromParams")

	// Should return value from embedded context.Context (higher priority)
	val := ctx.Get(keyUserID)
	if val != "fromContext" {
		t.Errorf("expected 'fromContext', got %v", val)
	}
}

func TestFallbackToParams(t *testing.T) {
	ctx := NewContext()

	// Nothing in context.Context
	// Set in Parameters
	ctx.Set(keyUserID, "onlyFromParams")

	val := ctx.Get(keyUserID)
	if val != "onlyFromParams" {
		t.Errorf("expected 'onlyFromParams', got %v", val)
	}
}

func TestMissingKey(t *testing.T) {
	ctx := NewContext()

	val := ctx.Get(keyUserID)
	if val != nil {
		t.Errorf("expected nil, got %v", val)
	}
}
