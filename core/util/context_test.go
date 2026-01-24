package util

import (
	"context"
	"testing"
	"time"
)

func TestCloneContextValues_Basic(t *testing.T) {
	parent := context.WithValue(context.Background(), "foo", "bar")
	parent = context.WithValue(parent, "user", 123)

	ctx := CloneContextValues(parent, "foo", "user")

	if got := ctx.Value("foo"); got != "bar" {
		t.Fatalf("expected foo=bar, got %v", got)
	}
	if got := ctx.Value("user"); got != 123 {
		t.Fatalf("expected user=123, got %v", got)
	}
}

func TestCloneContextValues_MissingKeyIgnored(t *testing.T) {
	parent := context.WithValue(context.Background(), "foo", "bar")

	ctx := CloneContextValues(parent, "foo", "missing")

	if got := ctx.Value("foo"); got != "bar" {
		t.Fatalf("expected foo=bar, got %v", got)
	}
	if got := ctx.Value("missing"); got != nil {
		t.Fatalf("expected missing=nil, got %v", got)
	}
}

func TestCloneContextValues_IndependentFromParentCancel(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	parent = context.WithValue(parent, "trace_id", "abc-123")

	ctx := CloneContextValues(parent, "trace_id")

	cancel()

	// Parent should be cancelled
	select {
	case <-parent.Done():
		// ok
	default:
		t.Fatalf("expected parent to be cancelled")
	}

	// New context should NOT be cancelled
	select {
	case <-ctx.Done():
		t.Fatalf("expected new cloned context NOT to be cancelled")
	default:
		// ok
	}

	if got := ctx.Value("trace_id"); got != "abc-123" {
		t.Fatalf("expected trace_id to be copied, got %v", got)
	}
}

func TestCloneContextValues_DoesNotCopyDeadline(t *testing.T) {
	parent, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	parent = context.WithValue(parent, "key", "value")

	ctx := CloneContextValues(parent, "key")

	// New context should have NO deadline
	_, hasDeadline := ctx.Deadline()
	if hasDeadline {
		t.Fatalf("expected no deadline on new context")
	}

	if ctx.Value("key") != "value" {
		t.Fatalf("value not copied")
	}
}
