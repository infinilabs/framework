/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"sync"
	"time"

	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
)

// StepEvent is the canonical description of one processor invocation on one
// batch/record. It is the single source for the three observability
// projections (D5): dry-run captures it fully (snapshots + diff), runtime
// metrics aggregate it (counters/latency histograms), and the flight
// recorder keeps a sampled ring of it (errors always kept).
type StepEvent struct {
	TraceID string // record-level trace id ("" when absent)
	// InstanceID identifies the executing node. Multi-instance
	// deployments attribute events per instance (cluster views aggregate
	// them); emitStep stamps it, so it is "" only in unit tests that
	// build events by hand.
	InstanceID string
	Pipeline   string      // pipeline/task name
	Step       int         // 0-based index within the executing chain
	Processor  string      // processor Name()
	Scope      ScopeInfo   // batch | record + namespace ("" = unset)
	Before     util.MapStr // optional pre-state snapshot (nil unless captured)
	After      util.MapStr // optional post-state snapshot (nil unless captured)
	Duration   time.Duration
	Outcome    Outcome
	Err        string
}

// Outcome classifies how a step ended.
type Outcome string

const (
	OutcomeOK              Outcome = "ok"
	OutcomeError           Outcome = "error"
	OutcomeDropped         Outcome = "dropped"
	OutcomeSkippedContract Outcome = "skipped-contract"

	// OutcomeTakeover marks a singleton lock takeover (a control-plane
	// lifecycle event, not a chain step): this instance acquired a lock
	// whose previous holder's lease expired. Not an error, but always
	// retained by the flight recorder (any non-ok outcome is).
	OutcomeTakeover Outcome = "lock-takeover"
)

// ExecutionObserver receives StepEvents as chains execute. Observers must
// be cheap and non-blocking: they run on pipeline hot paths. The nil/empty
// registry is a zero-cost fast path — production without observability
// installs nothing and pays one len() check per step.
type ExecutionObserver interface {
	OnStep(e *StepEvent)
}

// ScopeInfo carries the compile/run-time scope a step executed in. Batch
// scope is the chain root (messages convention); record scope is inside a
// record host (for_each / lane_consumer) with the namespace of the active
// codec/lane.
type ScopeInfo struct {
	Kind      ScopeKind
	Namespace string
	// Strict turns contract mismatches into compile errors instead of
	// warnings (threaded from PipelineConfigV2.StrictContract by hosts).
	Strict bool
}

type ScopeKind int

const (
	// ScopeAny is the zero value: a contract without a scope requirement
	// accepts every placement (the "undeclared = pass" rule).
	ScopeAny ScopeKind = iota
	ScopeBatch
	ScopeRecord
)

func (s ScopeInfo) String() string {
	if s.Kind == ScopeRecord {
		return "record:" + s.Namespace
	}
	return "batch"
}

var (
	observersMu sync.RWMutex
	observers   []ExecutionObserver
)

// RegisterObserver installs a global execution observer (idempotent per
// instance). Cheap enough to call from tests and tooling; the empty
// registry keeps the hot path free.
func RegisterObserver(o ExecutionObserver) {
	if o == nil {
		return
	}
	observersMu.Lock()
	defer observersMu.Unlock()
	for _, existing := range observers {
		if existing == o {
			return
		}
	}
	observers = append(observers, o)
}

// UnregisterObserver removes a previously installed observer.
func UnregisterObserver(o ExecutionObserver) {
	observersMu.Lock()
	defer observersMu.Unlock()
	for i, existing := range observers {
		if existing == o {
			observers = append(observers[:i], observers[i+1:]...)
			return
		}
	}
}

func hasObservers() bool {
	observersMu.RLock()
	defer observersMu.RUnlock()
	return len(observers) > 0
}

// emitStep fans a StepEvent out to the installed observers. Caller fills
// the event; emitStep only stamps what it can derive (TraceID from the
// current record when in record scope, InstanceID from the local node).
func emitStep(e *StepEvent, ctx *Context) {
	if e.InstanceID == "" {
		e.InstanceID = global.Env().SystemConfig.NodeConfig.ID
	}
	if ctx != nil {
		if rec, ok := CurrentRecord(ctx); ok {
			e.TraceID = TraceID(rec)
		}
	}
	observersMu.RLock()
	defer observersMu.RUnlock()
	for _, o := range observers {
		o.OnStep(e)
	}
}
