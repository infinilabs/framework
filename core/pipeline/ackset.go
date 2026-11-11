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

// doner is implemented by receipts that expose their settle signal for
// immediate settlement checks (lane receipts and the internal
// receiptImpl do).
type doner interface{ Done() <-chan error }

type receiptImpl struct{ done chan error }

func newReceipt() *receiptImpl { return &receiptImpl{done: make(chan error, 1)} }
func (r *receiptImpl) settle(err error) {
	select {
	case r.done <- err:
	default:
	}
}

// Done exposes the settle signal for non-consuming polling (AckSet.Pending).
func (r *receiptImpl) Done() <-chan error { return r.done }

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
// lane hop; duplicates are absorbed by idempotent sinks). Settlement
// ERRORS surface as Wait's return so a failed delivery fails the gate
// instead of committing the offset.
type AckSet struct {
	mu    sync.Mutex
	items []*ackItem
}

// ackItem tracks one receipt's outcome so results are readable repeatedly:
// reading a settle signal CONSUMES it (channels have no peek), so every
// read — Pending's check or Wait's block — records the outcome on the
// item the moment it happens; later calls re-read the record. Polling
// between waits can never eat a settlement error or hang a later Wait.
type ackItem struct {
	mu      sync.Mutex
	receipt Receipt
	waited  bool
	err     error
}

// settled reports whether the receipt has finished. Reading a settle
// signal CONSUMES it (channels have no peek), so the outcome is recorded
// on the item the moment it is read — every later Pending/Wait re-reads
// the record instead of draining an empty channel. Implementations
// without a Done signal count as unsettled until Wait records them.
func (it *ackItem) settled() bool {
	it.mu.Lock()
	defer it.mu.Unlock()
	if it.waited {
		return true
	}
	if d, ok := it.receipt.(doner); ok {
		select {
		case err := <-d.Done():
			it.waited, it.err = true, err
			return true
		default:
			return false
		}
	}
	return false
}

// waitOnce blocks for the receipt's outcome under ctx and records it. A
// ctx expiry/cancel is NOT a settlement (the item stays unwaited so a
// later Wait still observes the real outcome), and an already-settled
// receipt is read even when ctx is already done.
func (it *ackItem) waitOnce(ctx context.Context) error {
	it.mu.Lock()
	defer it.mu.Unlock()
	if it.waited {
		return it.err
	}
	if d, ok := it.receipt.(doner); ok {
		select {
		case err := <-d.Done():
			it.waited, it.err = true, err
			return err
		default:
		}
	}
	err := it.receipt.Wait(ctx)
	if ctx.Err() != nil && (errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)) {
		return err
	}
	it.waited, it.err = true, err
	return err
}

func (it *ackItem) outcome() error {
	it.mu.Lock()
	defer it.mu.Unlock()
	return it.err
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

// AttachTo shares this commit gate with a derived execution context while
// leaving that context's record, clone and failure-tag parameters isolated.
func (s *AckSet) AttachTo(ctx *Context) {
	if s != nil && ctx != nil {
		ctx.Set(ackSetKey, s)
	}
}

// Add registers a receipt with this set.
func (s *AckSet) Add(r *receiptImpl) { s.AddReceipt(r) }

// AddReceipt registers any Receipt implementation (lane receipts et al).
func (s *AckSet) AddReceipt(r Receipt) {
	if s == nil || r == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, &ackItem{receipt: r})
}

func (s *AckSet) snapshot() []*ackItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*ackItem, len(s.items))
	copy(out, s.items)
	return out
}

// Pending reports the unsettled receipt count. Settle signals read here
// are recorded on the item (not lost) — a later Wait re-reads the record
// instead of draining an empty channel.
func (s *AckSet) Pending() int {
	if s == nil {
		return 0
	}
	n := 0
	for _, it := range s.snapshot() {
		if !it.settled() {
			n++
		}
	}
	return n
}

// Wait blocks until every receipt settles, the timeout expires or ctx is
// canceled. The timeout bounds EVERY wait — external receipts included —
// so a stuck downstream cannot outlive the caller's ack deadline. Once all
// receipts settle it returns the first settlement error (nil when every
// delivery succeeded), re-reading recorded outcomes on repeated calls.
func (s *AckSet) Wait(ctx context.Context, timeout time.Duration) error {
	if s == nil {
		return nil
	}
	wctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	items := s.snapshot()
	for _, it := range items {
		if err := it.waitOnce(wctx); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				return ErrAckTimeout
			}
			return err
		}
	}
	// everything settled: surface the first failed delivery
	for _, it := range items {
		if err := it.outcome(); err != nil {
			return err
		}
	}
	return nil
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
