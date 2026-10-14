/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

// Package lane is the in-process record channel: the real-time
// counterpart of the disk queue. Payloads are *event.Event POINTERS
// (the record IS the message) — zero serialization in transit; delivery
// is at-most-once (Emit returning marks the producer's responsibility
// end), bounded memory with explicit backpressure (block by default,
// drop for best-effort taps, spill only when fast and slow paths run
// equivalent chains).
package lane

import (
	"context"
	"fmt"
	"sync"
	"time"

	"infini.sh/framework/core/event"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/queue"
	"infini.sh/framework/core/util"
)

// LaneConfig declares one lane.
type LaneConfig struct {
	Name      string `config:"name" json:"name"`
	Namespace string `config:"namespace" json:"namespace"` // record family contract
	Capacity  int    `config:"capacity" json:"capacity"`   // bounded; in-flight records
	// Overflow policy when the lane is full:
	//   block        default — Emit waits, backpressure reaches the producer
	//   drop         best-effort — discard and count
	//   spill:<queue> degrade to the disk queue (fast/slow paths must run
	//                EQUIVALENT chains — a required step on the fast path
	//                would be bypassed for spilled records)
	Overflow string `config:"overflow" json:"overflow"`
	// Owner marks dynamic lanes for cascade cleanup ("" = config-owned).
	Owner string `config:"owner,omitempty" json:"owner,omitempty"`
}

const (
	defaultCapacity = 4096
	dynamicCapacity = 512
)

// Handler consumes one delivered record.
type Handler func(rec *event.Event)

// Stats snapshots a lane's counters for observability.
type Stats struct {
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	Owner        string `json:"owner,omitempty"`
	Capacity     int    `json:"capacity"`
	Depth        int    `json:"depth"`
	Emitted      int64  `json:"emitted"`
	Delivered    int64  `json:"delivered"`
	Drops        int64  `json:"drops"`
	Spills       int64  `json:"spills"`
	BlockedWaits int64  `json:"blocked_waits"`
	Closed       bool   `json:"closed"`
}

type laneImpl struct {
	cfg     LaneConfig
	ch      chan *event.Event
	mu      sync.Mutex // guards counters + closed + pending
	stats   Stats
	spillQ  *queue.QueueConfig
	pending map[*event.Event]*laneReceipt
	quit    chan struct{}
	once    sync.Once
}

// Lane is one named bounded record channel.
type Lane interface {
	Config() LaneConfig
	Emit(rec *event.Event) error
	TryEmit(rec *event.Event) bool
	// EmitAck emits with an acknowledgment receipt (commit-gate support).
	EmitAck(rec *event.Event) pipeline.Receipt
	// SubscribeBatch consumes in micro-batches: receipts of a batch's
	// records settle only after fn(batch) returns (batch hosts flush on
	// size or idle timeout).
	SubscribeBatch(max int, flushInterval time.Duration, fn func(recs []*event.Event)) (cancel func())
	Subscribe(h Handler) (cancel func())
	Depth() int
	Snapshot() Stats
	Close() error // drain-or-drop; at-most-once semantics end here
}

func (l *laneImpl) Config() LaneConfig { return l.cfg }

func (l *laneImpl) Emit(rec *event.Event) error {
	if rec == nil {
		return fmt.Errorf("lane %s: nil record", l.cfg.Name)
	}
	l.mu.Lock()
	if l.stats.Closed {
		l.mu.Unlock()
		return fmt.Errorf("lane %s: closed", l.cfg.Name)
	}
	l.mu.Unlock()

	for {
		select {
		case l.ch <- rec:
			l.mu.Lock()
			l.stats.Emitted++
			l.mu.Unlock()
			return nil
		default:
		}
		switch l.overflowPolicy() {
		case "drop":
			l.mu.Lock()
			l.stats.Drops++
			r := l.pending[rec]
			delete(l.pending, rec)
			l.mu.Unlock()
			if r != nil {
				r.settle(nil) // explicit drop is a settlement
			}
			return nil
		case "spill":
			if err := l.spill(rec); err == nil {
				l.mu.Lock()
				l.stats.Spills++
				l.mu.Unlock()
				return nil
			} // spill failure falls through to block
		}
		// block: wait for room or shutdown
		l.mu.Lock()
		l.stats.BlockedWaits++
		l.mu.Unlock()
		select {
		case l.ch <- rec:
			l.mu.Lock()
			l.stats.Emitted++
			l.mu.Unlock()
			return nil
		case <-time.After(50 * time.Millisecond):
			l.mu.Lock()
			closed := l.stats.Closed
			l.mu.Unlock()
			if closed {
				return fmt.Errorf("lane %s: closed while blocking", l.cfg.Name)
			}
		}
	}
}

// laneReceipt satisfies pipeline.Receipt.
type laneReceipt struct{ done chan error }

func (r *laneReceipt) Wait(ctx context.Context) error {
	select {
	case err := <-r.done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Done exposes the settle signal for commit-gate polling.
func (r *laneReceipt) Done() <-chan error { return r.done }

func (r *laneReceipt) settle(err error) {
	select {
	case r.done <- err:
	default:
	}
}

// EmitAck emits and returns a receipt settled when the record's
// processing completes in the subscriber (handler return = settlement:
// output_queue/dead_letter pushes happen inside the handler's chain).
// Failed emissions settle with the error immediately.
func (l *laneImpl) EmitAck(rec *event.Event) pipeline.Receipt {
	r := &laneReceipt{done: make(chan error, 1)}
	if rec == nil {
		r.settle(fmt.Errorf("lane %s: nil record", l.cfg.Name))
		return r
	}
	l.mu.Lock()
	if l.stats.Closed {
		l.mu.Unlock()
		r.settle(fmt.Errorf("lane %s: closed", l.cfg.Name))
		return r
	}
	if l.pending == nil {
		l.pending = map[*event.Event]*laneReceipt{}
	}
	l.pending[rec] = r
	l.mu.Unlock()
	if err := l.Emit(rec); err != nil {
		l.settlePending(rec, err)
	}
	return r
}

func (l *laneImpl) settlePending(rec *event.Event, err error) {
	l.mu.Lock()
	r := l.pending[rec]
	delete(l.pending, rec)
	l.mu.Unlock()
	if r != nil {
		r.settle(err)
	}
}

func (l *laneImpl) TryEmit(rec *event.Event) bool {
	select {
	case l.ch <- rec:
		l.mu.Lock()
		l.stats.Emitted++
		l.mu.Unlock()
		return true
	default:
		return false
	}
}

func (l *laneImpl) overflowPolicy() string {
	if l.cfg.Overflow != "" {
		return l.cfg.Overflow
	}
	return "block"
}

// spill encodes the record (three-level codec resolution, mirroring the
// lane_consumer output bridge) and degrades it to the disk queue.
func (l *laneImpl) spill(rec *event.Event) error {
	if l.spillQ == nil {
		qname := util.TrimSpaces(l.cfg.Overflow[len("spill:"):])
		if qname == "" {
			return fmt.Errorf("lane %s: spill target missing", l.cfg.Name)
		}
		cfg := &queue.QueueConfig{Name: qname}
		l.spillQ = queue.SmartGetOrInitConfig(cfg)
	}
	codec := l.resolveCodec(rec)
	b, err := codec.Encode(rec)
	if err != nil {
		return err
	}
	return queue.Push(l.spillQ, b)
}

// resolveCodec picks the output codec: the record's _codec hint, then
// this lane's namespace primary codec, then the default.
func (l *laneImpl) resolveCodec(rec *event.Event) pipeline.RecordCodec {
	if id := pipeline.CodecHint(rec); id != "" {
		if c := pipeline.GetRecordCodec(id); c.Name() == id {
			return c
		}
	}
	if l.cfg.Namespace != "" {
		for _, ci := range pipeline.ListRecordCodecs() {
			if ci.Namespace == l.cfg.Namespace {
				return pipeline.GetRecordCodec(ci.Name)
			}
		}
	}
	return pipeline.GetRecordCodec("")
}

func (l *laneImpl) Subscribe(h Handler) (cancel func()) {
	return l.SubscribeBatch(1, 0, func(recs []*event.Event) {
		for _, rec := range recs {
			h(rec)
		}
	})
}

// SubscribeBatch delivers micro-batches: a batch flushes when it reaches
// max records or after flushInterval of idleness. Receipts of every record
// in the batch settle AFTER fn returns — batch processing completion is
// the settlement point, keeping EmitAck's commit-gate semantics exact for
// batching consumers.
func (l *laneImpl) SubscribeBatch(max int, flushInterval time.Duration, fn func(recs []*event.Event)) (cancel func()) {
	if max <= 0 {
		max = 1
	}
	go func() {
		var batch []*event.Event
		var receipts []*laneReceipt
		timer := time.NewTimer(0)
		if !timer.Stop() {
			<-timer.C
		}
		flush := func() {
			if len(batch) == 0 {
				return
			}
			l.mu.Lock()
			l.stats.Delivered += int64(len(batch))
			l.mu.Unlock()
			fn(batch)
			for _, r := range receipts {
				r.settle(nil)
			}
			batch = nil
			receipts = nil
		}
		for {
			var flushC <-chan time.Time
			if len(batch) > 0 && flushInterval > 0 {
				flushC = timer.C
			}
			select {
			case rec, ok := <-l.ch:
				if !ok {
					flush()
					return
				}
				l.mu.Lock()
				r := l.pending[rec]
				delete(l.pending, rec)
				l.mu.Unlock()
				batch = append(batch, rec)
				if r != nil {
					receipts = append(receipts, r)
				}
				if len(batch) >= max {
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					flush()
					continue
				}
				if flushInterval > 0 && len(batch) == 1 {
					timer.Reset(flushInterval)
				}
			case <-flushC:
				flush()
			case <-l.quit:
				// drain what is already buffered, then stop
				for {
					select {
					case rec, ok := <-l.ch:
						if !ok {
							flush()
							return
						}
						l.mu.Lock()
						r := l.pending[rec]
						delete(l.pending, rec)
						l.mu.Unlock()
						batch = append(batch, rec)
						if r != nil {
							receipts = append(receipts, r)
						}
					default:
						flush()
						return
					}
				}
			}
		}
	}()
	return func() { /* subscriber cancel: lanes are single-consumer v1 */ }
}

func (l *laneImpl) Depth() int { return len(l.ch) }

func (l *laneImpl) Snapshot() Stats {
	l.mu.Lock()
	defer l.mu.Unlock()
	s := l.stats
	s.Name, s.Namespace, s.Owner, s.Capacity = l.cfg.Name, l.cfg.Namespace, l.cfg.Owner, l.cfg.Capacity
	s.Depth = len(l.ch)
	return s
}

func (l *laneImpl) Close() error {
	l.once.Do(func() {
		l.mu.Lock()
		l.stats.Closed = true
		for rec, r := range l.pending {
			r.settle(fmt.Errorf("lane %s: closed with record in flight", l.cfg.Name))
			delete(l.pending, rec)
		}
		l.mu.Unlock()
		close(l.quit)
	})
	return nil
}

// ─── registry ────────────────────────────────────────────────────────────────

var (
	regMu    sync.RWMutex
	registry = map[string]Lane{}
)

// Create registers a lane (duplicate name errors; runtime callers react,
// config-plane init uses the framework's fail-fast conventions upstream).
func Create(cfg LaneConfig) (Lane, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("lane name is empty")
	}
	if cfg.Capacity <= 0 {
		cfg.Capacity = defaultCapacity
		if cfg.Owner != "" {
			cfg.Capacity = dynamicCapacity // dynamic lanes default small
		}
	}
	regMu.Lock()
	defer regMu.Unlock()
	if _, exists := registry[cfg.Name]; exists {
		return nil, fmt.Errorf("lane %s already exists", cfg.Name)
	}
	l := &laneImpl{cfg: cfg, ch: make(chan *event.Event, cfg.Capacity), quit: make(chan struct{})}
	registry[cfg.Name] = l
	return l, nil
}

// GetOrCreate returns the named lane, creating it lazily on first Emit
// (dynamic pipelines; owner tags it for cascade cleanup).
func GetOrCreate(cfg LaneConfig) Lane {
	regMu.RLock()
	l, ok := registry[cfg.Name]
	regMu.RUnlock()
	if ok {
		return l
	}
	l, err := Create(cfg)
	if err != nil {
		regMu.RLock()
		existing := registry[cfg.Name]
		regMu.RUnlock()
		return existing // lost the race: the winner's instance wins
	}
	return l
}

// Get returns the named lane (nil when absent).
func Get(name string) Lane {
	regMu.RLock()
	defer regMu.RUnlock()
	return registry[name]
}

// Delete removes a lane; refuses while records are in flight unless force.
func Delete(name string, force bool) error {
	regMu.Lock()
	l, ok := registry[name]
	if !ok {
		return nil
	}
	regMu.Unlock()
	if !force && l.Depth() > 0 {
		return fmt.Errorf("lane %s has %d in-flight records (force to drop)", name, l.Depth())
	}
	l.Close()
	regMu.Lock()
	delete(registry, name)
	regMu.Unlock()
	return nil
}

// DeleteOwner cascades dynamic-lane cleanup (stream delete → lanes die).
func DeleteOwner(owner string) int {
	regMu.RLock()
	names := make([]string, 0)
	for n, l := range registry {
		if l.Config().Owner == owner && owner != "" {
			names = append(names, n)
		}
	}
	regMu.RUnlock()
	for _, n := range names {
		_ = Delete(n, true)
	}
	return len(names)
}

// List snapshots all lanes for observability.
func List() []Stats {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]Stats, 0, len(registry))
	for _, l := range registry {
		out = append(out, l.Snapshot())
	}
	return out
}
