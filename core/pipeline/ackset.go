/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"context"
	"errors"
	"sync"
	"time"

	"infini.sh/framework/core/param"
)

// ErrAckTimeout marks a commit-gate wait that expired with unsettled
// receipts — the caller must NOT commit the offset (redelivery follows).
var ErrAckTimeout = errors.New("ack set: unsettled receipts at timeout")

// Receipt is one pending delivery confirmation: resolved by whoever
// finishes processing the emitted record (settlement = output/dead-letter
// push or explicit end-of-chain).
type Receipt interface {
	Wait(ctx context.Context) error
}

type receiptImpl struct{ done chan error }

func newReceipt() *receiptImpl { return &receiptImpl{done: make(chan error, 1)} }
func (r *receiptImpl) settle(err error) {
	select {
	case r.done <- err:
	default:
	}
}
func (r *receiptImpl) Wait(ctx context.Context) error {
	select {
	case err := <-r.done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ackSetKey carries the AckSet on the emitting chain's context.
const ackSetKey param.ParaKey = "__ack_set"

// AckSet is the commit gate: lane outputs attach receipts; the queue
// consumer waits for all of them before committing its offset. Unsettled
// receipts (crash downstream, stuck chain) surface as ErrAckTimeout —
// the offset stays put and the batch replays (at-least-once across the
// lane hop; duplicates are absorbed by idempotent sinks).
type AckSet struct {
	mu       sync.Mutex
	receipts []*receiptImpl
	external []Receipt
}

// AttachAckSet installs a fresh set on the context (idempotent).
func AttachAckSet(ctx *Context) *AckSet {
	if ctx == nil {
		return nil
	}
	if existing, ok := ctx.Get(ackSetKey).(*AckSet); ok {
		return existing
	}
	s := &AckSet{}
	ctx.Set(ackSetKey, s)
	return s
}

// AckSetOf returns the context's set (nil when none attached).
func AckSetOf(ctx *Context) *AckSet {
	if ctx == nil {
		return nil
	}
	s, _ := ctx.Get(ackSetKey).(*AckSet)
	return s
}

// Add registers a receipt with this set.
func (s *AckSet) Add(r *receiptImpl) {
	if s == nil || r == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.receipts = append(s.receipts, r)
}

// AddReceipt registers any Receipt implementation (lane receipts et al).
func (s *AckSet) AddReceipt(r Receipt) {
	if s == nil || r == nil {
		return
	}
	if own, ok := r.(*receiptImpl); ok {
		s.Add(own)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.external = append(s.external, r)
}

// Pending reports the unsettled receipt count.
func (s *AckSet) Pending() int {
	if s == nil {
		return 0
	}
	n := 0
	for _, r := range s.snapshot() {
		select {
		case <-r.done:
		default:
			n++
		}
	}
	for _, r := range s.externalList() {
		if !receiptSettled(r) {
			n++
		}
	}
	return n
}

// Wait blocks until every receipt settles or the timeout expires.
func (s *AckSet) Wait(ctx context.Context, timeout time.Duration) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	ext := append([]Receipt(nil), s.external...)
	s.mu.Unlock()
	for _, r := range ext {
		if err := r.Wait(ctx); err != nil {
			return err
		}
		s.mu.Lock()
		for i, e := range s.external {
			if e == r {
				s.external = append(s.external[:i], s.external[i+1:]...)
				break
			}
		}
		s.mu.Unlock()
	}
	deadline := time.After(timeout)
	for {
		pending := s.Pending()
		if pending == 0 {
			return nil
		}
		select {
		case <-deadline:
			return ErrAckTimeout
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Millisecond):
		}
	}
}

func (s *AckSet) externalList() []Receipt {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Receipt(nil), s.external...)
}

func (s *AckSet) snapshot() []*receiptImpl {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*receiptImpl, len(s.receipts))
	copy(out, s.receipts)
	return out
}

// receiptSettled reports whether an external Receipt has settled when
// the implementation exposes its signal (lane receipts do); unresolved
// implementations count as pending.
func receiptSettled(r Receipt) bool {
	type doner interface{ Done() <-chan error }
	if d, ok := r.(doner); ok {
		select {
		case <-d.Done():
			return true
		default:
			return false
		}
	}
	return false
}

// ─── dry-run interception ────────────────────────────────────────────────────

// DryRunWrite is one captured side effect (queue/lane/attachment write
// intercepted during a dry run — never delivered).
type DryRunWrite struct {
	Kind    string // queue | lane | attachment (host-specific labels welcome)
	Target  string
	Records int
	Preview string
}

// dryRunCtxKey rides the Go context chain: processor helpers derive plain
// contexts from ctx.Context and never see the pipeline wrapper's parameter
// store, so the state must travel with the embedded chain to stay visible
// under orm/attachment layers.
type dryRunCtxKey struct{}

// EnableDryRun marks the context as a dry run; when capture is true,
// output processors append intercepted writes to sink instead of
// delivering them.
func EnableDryRun(ctx *Context, sink *[]DryRunWrite, capture bool) {
	if ctx == nil {
		return
	}
	st := &dryRunState{sink: sink, capture: capture}
	ctx.Context = context.WithValue(ctx.Context, dryRunCtxKey{}, st)
}

type dryRunState struct {
	sink    *[]DryRunWrite
	capture bool
}

func dryRunStateFrom(ctx context.Context) *dryRunState {
	if ctx == nil {
		return nil
	}
	st, _ := ctx.Value(dryRunCtxKey{}).(*dryRunState)
	return st
}

// InDryRun reports whether this context runs under a dry-run replay
// (EnableDryRun was called), regardless of capture mode.
func InDryRun(ctx *Context) bool {
	if ctx == nil {
		return false
	}
	return dryRunStateFrom(ctx.Context) != nil
}

// InDryRunCtx is the context-chain variant for code holding a derived
// plain context (below processors that re-wrap the parent).
func InDryRunCtx(ctx context.Context) bool {
	return dryRunStateFrom(ctx) != nil
}

// DryRunCapture returns the sink when this context runs in capturing
// dry-run mode (nil = deliver for real).
func DryRunCapture(ctx *Context) *[]DryRunWrite {
	if ctx == nil {
		return nil
	}
	return dryRunSinkFrom(ctx.Context)
}

// DryRunCaptureCtx is the context-chain variant for code deep under a
// pipeline (orm/attachment layers holding derived contexts).
func DryRunCaptureCtx(ctx context.Context) *[]DryRunWrite {
	return dryRunSinkFrom(ctx)
}

func dryRunSinkFrom(ctx context.Context) *[]DryRunWrite {
	st := dryRunStateFrom(ctx)
	if st == nil || !st.capture {
		return nil
	}
	return st.sink
}
