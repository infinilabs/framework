/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"fmt"
)

// Granularity declares the execution granularity a processor supports.
// It is DECLARED, never detected at runtime (D2): the compiler routes by
// it and the catalog surfaces it for the canvas.
type Granularity int

const (
	// GranularityRecord is the default: only the per-record path is
	// meaningful (grok, mutate, ...).
	GranularityRecord Granularity = iota
	// GranularityBatchCapable: batch + single-record paths are both valid
	// (batch preferred when the host provides one). Implementing
	// BatchProcessor implies the single-record fallback contract.
	GranularityBatchCapable
	// GranularityBatchOnly: only the batch channel is meaningful
	// (cross-record aggregation). Must be a DIRECT member of a record
	// host's sub-chain — nesting under if/then is a compile error there,
	// because the batch channel cannot reach it.
	GranularityBatchOnly
)

func (g Granularity) String() string {
	switch g {
	case GranularityBatchCapable:
		return "batch_capable"
	case GranularityBatchOnly:
		return "batch_only"
	default:
		return "record"
	}
}

// RecordContract is the optional declaration processors make about where
// they may run and what they need. Unconstrained processors (the entire
// historical inventory) implement nothing and pass every placement.
type RecordContract struct {
	// Scope is the scope the processor REQUIRES. ScopeRecord demands a
	// record host (for_each/lane_consumer sub-chain); ScopeBatch demands
	// the chain root / consumer sub-chain. Only set what you require.
	Scope ScopeKind
	// Namespace is the record family the processor depends on
	// ("document", "attachment"); "" accepts any namespace.
	Namespace string
	// RequiredFields are optional hard assertions on record paths
	// ("document.content"), enforced by the runtime guard and by dry-run
	// sample validation — never by placement (data does not exist at
	// compile time).
	RequiredFields []string
	// Granularity is the declared execution granularity (see Granularity).
	Granularity Granularity
}

// ContractProvider is implemented by processors that declare a contract.
type ContractProvider interface {
	RecordContract() RecordContract
}

// validatePlacement checks one constructed processor against the compile
// scope. Returns a descriptive error; the caller decides error-vs-warning
// (strict mode) — this function stays pure.
func validatePlacement(p Processor, stepIdx int, scope ScopeInfo, direct bool) error {
	cp, ok := p.(ContractProvider)
	if !ok {
		return nil
	}
	c := cp.RecordContract()
	where := fmt.Sprintf("processor [%s] at step %d", p.Name(), stepIdx+1)

	if c.Scope == ScopeRecord && scope.Kind != ScopeRecord {
		return fmt.Errorf("%s requires record scope (inside a for_each/lane_consumer sub-chain), found %s scope", where, scope)
	}
	if c.Scope == ScopeBatch && scope.Kind != ScopeBatch {
		return fmt.Errorf("%s requires batch scope (chain root / consumer sub-chain), found %s scope", where, scope)
	}
	if c.Scope == ScopeRecord && c.Namespace != "" && scope.Kind == ScopeRecord && scope.Namespace != c.Namespace {
		return fmt.Errorf("%s requires record namespace [%s], the host sub-chain provides [%s]", where, c.Namespace, scope.Namespace)
	}
	if c.Granularity == GranularityBatchOnly && scope.Kind == ScopeRecord && !direct {
		return fmt.Errorf("%s declares batch_only granularity and must be a direct member of the record host's processor list (batch channel cannot reach nested chains)", where)
	}
	return nil
}

// CheckContract is the runtime guard (defense in depth for chains that
// bypass compile-time validation — legacy stored configs, ad-hoc runs).
// Processors declaring a contract call it at the top of Process and hand
// the error to their host's on_failure policy.
func CheckContract(ctx *Context, c RecordContract) error {
	if ctx == nil {
		return nil
	}
	if c.Scope == ScopeRecord {
		if _, ok := CurrentRecord(ctx); !ok {
			return fmt.Errorf("contract requires record scope, no current record in context")
		}
		for _, f := range c.RequiredFields {
			if rec, ok := CurrentRecord(ctx); ok {
				if _, err := rec.Fields.GetValue(f); err != nil {
					return fmt.Errorf("contract requires field [%s], missing on current record", f)
				}
			}
		}
	}
	return nil
}
