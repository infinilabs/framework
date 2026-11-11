/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infinilabs.com */

package pipeline

import (
	"strings"
	"testing"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/util"
)

// batchScopedFixture declares a ScopeBatch contract — legal at a chain
// root, illegal inside a record sub-chain. It exists to prove strict
// compile reaches through for_each into the nested chain.
type batchScopedFixture struct{}

func (batchScopedFixture) Name() string { return "test_batch_scoped" }

func (batchScopedFixture) Process(*pipeline.Context) error { return nil }

func (batchScopedFixture) RecordContract() pipeline.RecordContract {
	return pipeline.RecordContract{Scope: pipeline.ScopeBatch}
}

func init() {
	// for_each registers during module setup, which these unit tests skip
	pipeline.RegisterProcessorPlugin("for_each", NewForEachProcessor)
	pipeline.RegisterProcessorPlugin("test_batch_scoped",
		func(*config.Config) (pipeline.Processor, error) { return batchScopedFixture{}, nil })
	pipeline.RegisterProcessorPlugin("test_registry_scoped",
		func(*config.Config) (pipeline.Processor, error) { return registryContractFixture{}, nil })
	// registry-declared contract, instance implements nothing — the path
	// every product processor takes (RegisterProcessorContract)
	pipeline.RegisterProcessorContract("test_registry_scoped",
		pipeline.RecordContract{Scope: pipeline.ScopeBatch})
}

// registryContractFixture carries NO ContractProvider method; its
// contract lives only in the registry.
type registryContractFixture struct{}

func (registryContractFixture) Name() string { return "test_registry_scoped" }

func (registryContractFixture) Process(*pipeline.Context) error { return nil }

func forEachWithBatchScopedStep(t *testing.T) *config.Config {
	t.Helper()
	c, err := config.NewConfigFrom(util.MapStr{
		"for_each": util.MapStr{
			"processor": []interface{}{util.MapStr{"test_batch_scoped": util.MapStr{}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestForEach_SubChainStrictnessThreaded(t *testing.T) {
	step := forEachWithBatchScopedStep(t)

	// non-strict: the nested placement mismatch only warns, the chain
	// compiles — this is the legacy behavior the default flip keeps
	if _, err := pipeline.NewPipeline([]*config.Config{step}); err != nil {
		t.Fatalf("non-strict compile should accept the nested mismatch: %v", err)
	}

	// strict: the violation inside the sub-chain must surface as a
	// compile error (threaded through ApplyScope)
	_, err := pipeline.NewPipelineWithScope([]*config.Config{step},
		pipeline.ScopeInfo{Kind: pipeline.ScopeBatch, Strict: true}, false)
	if err == nil || !strings.Contains(err.Error(), "requires batch scope") {
		t.Fatalf("strict compile must reject the nested violation, got: %v", err)
	}
}

func TestValidatePipelineConfig_NestedContractWarns(t *testing.T) {
	// the validation gate feeds the audit tool: without scope threading
	// a nested mismatch never reached its warnings list
	cfg := &pipeline.PipelineConfigV2{
		Name: "strict-threading-probe",
		Processors: []map[string]interface{}{
			{"for_each": util.MapStr{
				"processor": []interface{}{util.MapStr{"test_batch_scoped": util.MapStr{}}},
			}},
		},
	}
	errs, warns := ValidatePipelineConfig(cfg)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(warns) == 0 || !strings.Contains(warns[0].Message, "requires batch scope") {
		t.Fatalf("nested contract mismatch must warn, got warns=%v", warns)
	}
}

func TestValidatePlacement_RegistryDeclaredContract(t *testing.T) {
	// registry-declared contracts (the product-processor path) must drive
	// placement too — the instance-only assertion silently passed them
	c, err := config.NewConfigFrom(util.MapStr{
		"for_each": util.MapStr{
			"processor": []interface{}{util.MapStr{"test_registry_scoped": util.MapStr{}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	step := []*config.Config{c}

	if _, err := pipeline.NewPipeline(step); err != nil {
		t.Fatalf("non-strict compile should accept the mismatch: %v", err)
	}
	_, err = pipeline.NewPipelineWithScope(step,
		pipeline.ScopeInfo{Kind: pipeline.ScopeBatch, Strict: true}, false)
	if err == nil || !strings.Contains(err.Error(), "requires batch scope") {
		t.Fatalf("strict compile must honor registry-declared contracts, got: %v", err)
	}
}
