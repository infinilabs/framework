/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"strings"
	"testing"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/util"
)

// ---- contract test doubles (registered like real plugins) -------------------

type contractProc struct {
	name     string
	contract RecordContract
}

func (p *contractProc) Name() string { return p.name }
func (p *contractProc) Process(ctx *Context) error {
	return CheckContract(ctx, p.contract)
}
func (p *contractProc) RecordContract() RecordContract { return p.contract }

func init() {
	RegisterProcessorPlugin("test_record_only", func(c *config.Config) (Processor, error) {
		return &contractProc{"test_record_only", RecordContract{Scope: ScopeRecord}}, nil
	})
	RegisterProcessorPlugin("test_doc_only", func(c *config.Config) (Processor, error) {
		return &contractProc{"test_doc_only", RecordContract{Scope: ScopeRecord, Namespace: "document"}}, nil
	})
	RegisterProcessorPlugin("test_batch_scope", func(c *config.Config) (Processor, error) {
		return &contractProc{"test_batch_scope", RecordContract{Scope: ScopeBatch}}, nil
	})
	RegisterProcessorPlugin("test_batch_only", func(c *config.Config) (Processor, error) {
		return &contractProc{"test_batch_only", RecordContract{Scope: ScopeRecord, Granularity: GranularityBatchOnly}}, nil
	})
	RegisterProcessorPlugin("test_unconstrained", func(c *config.Config) (Processor, error) {
		return &contractProc{"test_unconstrained", RecordContract{}}, nil
	})
}

func cfgList(t *testing.T, name string) []*config.Config {
	t.Helper()
	c, err := config.NewConfigFrom([]util.MapStr{{name: util.MapStr{}}})
	if err != nil {
		t.Fatal(err)
	}
	var list []*config.Config
	if err := c.Unpack(&list); err != nil {
		t.Fatal(err)
	}
	return list
}

func TestContract_RecordScopeInBatchChainStrict(t *testing.T) {
	scope := ScopeInfo{Kind: ScopeBatch, Strict: true}
	_, err := newPipelineWithScope(cfgList(t, "test_record_only"), scope, false)
	if err == nil || !strings.Contains(err.Error(), "requires record scope") {
		t.Fatalf("want record-scope error, got %v", err)
	}
}

func TestContract_RecordScopeInBatchChainNonStrictWarns(t *testing.T) {
	// legacy default: NewPipeline never hard-fails on contracts
	procs, err := NewPipeline(cfgList(t, "test_record_only"))
	if err != nil {
		t.Fatalf("legacy NewPipeline must compile contract mismatches (warn only): %v", err)
	}
	if len(procs.List) != 1 {
		t.Fatal("processor dropped in non-strict mode")
	}
}

func TestContract_NamespaceMismatch(t *testing.T) {
	scope := ScopeInfo{Kind: ScopeRecord, Namespace: "otel-generic", Strict: true}
	_, err := newPipelineWithScope(cfgList(t, "test_doc_only"), scope, true)
	if err == nil || !strings.Contains(err.Error(), "namespace") {
		t.Fatalf("want namespace mismatch error, got %v", err)
	}
	// matching namespace compiles
	scope.Namespace = "document"
	if _, err := newPipelineWithScope(cfgList(t, "test_doc_only"), scope, true); err != nil {
		t.Fatalf("matching namespace must compile: %v", err)
	}
}

func TestContract_BatchScopeInRecordChain(t *testing.T) {
	scope := ScopeInfo{Kind: ScopeRecord, Namespace: "", Strict: true}
	_, err := newPipelineWithScope(cfgList(t, "test_batch_scope"), scope, true)
	if err == nil || !strings.Contains(err.Error(), "requires batch scope") {
		t.Fatalf("want batch-scope error, got %v", err)
	}
}

func TestContract_BatchOnlyNestedIsRejected(t *testing.T) {
	// direct member of a record host sub-chain: OK
	direct := ScopeInfo{Kind: ScopeRecord, Namespace: "", Strict: true}
	if _, err := newPipelineWithScope(cfgList(t, "test_batch_only"), direct, true); err != nil {
		t.Fatalf("batch_only as direct member must compile: %v", err)
	}
	// nested under if.then (same scope, non-direct): compile error
	c, err := config.NewConfigFrom([]util.MapStr{{"if": util.MapStr{"equals": util.MapStr{"x": "1"}},
		"then": []util.MapStr{{"test_batch_only": util.MapStr{}}}}})
	if err != nil {
		t.Fatal(err)
	}
	var list []*config.Config
	if err := c.Unpack(&list); err != nil {
		t.Fatal(err)
	}
	if _, err := newPipelineWithScope(list, direct, true); err == nil || !strings.Contains(err.Error(), "batch_only") {
		t.Fatalf("want batch_only nesting error, got %v", err)
	}
}

func TestContract_UnconstrainedPassesEverywhere(t *testing.T) {
	for _, scope := range []ScopeInfo{
		{Kind: ScopeBatch, Strict: true},
		{Kind: ScopeRecord, Namespace: "anything", Strict: true},
	} {
		if _, err := newPipelineWithScope(cfgList(t, "test_unconstrained"), scope, true); err != nil {
			t.Fatalf("unconstrained processor rejected in %v: %v", scope, err)
		}
	}
}
