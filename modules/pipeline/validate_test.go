/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	_ "infini.sh/framework/plugins/http"
	_ "infini.sh/framework/plugins/smtp"
	"strings"
	"testing"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/pipeline"
)

// paramCheckedProc mirrors a real plugin constructor: it rejects configs
// missing required fields or carrying unknown ones.
func init() {
	pipeline.RegisterProcessorPlugin("validate_test_echo", func(cfg *config.Config) (pipeline.Processor, error) {
		if !cfg.HasField("message") {
			return nil, errNew("validate_test_echo: `message` is required")
		}
		return nopProc{}, nil
	})
}

type nopProc struct{}

func (p nopProc) Name() string                  { return "validate_test_echo" }
func (p nopProc) Process(*pipeline.Context) error { return nil }

type testError struct{ msg string }

func (e testError) Error() string { return e.msg }

func errNew(msg string) error { return testError{msg} }

func cfgOf(procs ...map[string]interface{}) *pipeline.PipelineConfigV2 {
	return &pipeline.PipelineConfigV2{Name: "t", Processors: procs}
}

func TestValidateRejectsUnknownProcessor(t *testing.T) {
	errs, _ := ValidatePipelineConfig(cfgOf(map[string]interface{}{
		"no_such_processor_xyz": map[string]interface{}{"x": 1},
	}))
	if len(errs) == 0 {
		t.Fatal("expected rejection for unknown processor")
	}
	if errs[0].Processor != "no_such_processor_xyz" {
		t.Fatalf("expected processor attribution, got %+v", errs[0])
	}
	if errs[0].Stage != 0 {
		t.Fatalf("expected stage 0, got %d", errs[0].Stage)
	}
	if !strings.Contains(errs[0].Message, "does not exist") {
		t.Fatalf("expected registry error, got %q", errs[0].Message)
	}
}

func TestValidateRejectsInvalidParameters(t *testing.T) {
	errs, _ := ValidatePipelineConfig(cfgOf(map[string]interface{}{
		"validate_test_echo": map[string]interface{}{}, // missing required `message`
	}))
	if len(errs) == 0 {
		t.Fatal("expected rejection for missing required parameter")
	}
	if !strings.Contains(errs[0].Message, "`message` is required") {
		t.Fatalf("expected param error, got %q", errs[0].Message)
	}
}

func TestValidateRejectsEmptyChain(t *testing.T) {
	errs, _ := ValidatePipelineConfig(cfgOf())
	if len(errs) == 0 {
		t.Fatal("expected rejection for empty processor chain")
	}
	if errs[0].Field != "processor" {
		t.Fatalf("expected processor field error, got %+v", errs[0])
	}
}

func TestValidateRejectsMissingName(t *testing.T) {
	errs, _ := ValidatePipelineConfig(&pipeline.PipelineConfigV2{
		Processors: []map[string]interface{}{
			{"validate_test_echo": map[string]interface{}{"message": "hi"}},
		},
	})
	if len(errs) == 0 || errs[0].Field != "name" {
		t.Fatalf("expected name error, got %+v", errs)
	}
}

func TestValidateAcceptsValidChain(t *testing.T) {
	errs, _ := ValidatePipelineConfig(cfgOf(
		map[string]interface{}{"validate_test_echo": map[string]interface{}{"message": "a"}},
		map[string]interface{}{"validate_test_echo": map[string]interface{}{"message": "b"}},
	))
	if len(errs) != 0 {
		t.Fatalf("valid chain rejected: %+v", errs)
	}
}

func TestValidateAttributesStageIndex(t *testing.T) {
	errs, _ := ValidatePipelineConfig(cfgOf(
		map[string]interface{}{"validate_test_echo": map[string]interface{}{"message": "ok"}},
		map[string]interface{}{"validate_test_echo": map[string]interface{}{}}, // stage 1 broken
	))
	if len(errs) != 1 {
		t.Fatalf("expected exactly one error, got %+v", errs)
	}
	if errs[0].Stage != 1 {
		t.Fatalf("expected stage 1 attribution, got %d", errs[0].Stage)
	}
}

func TestValidateRejectsMalformedStep(t *testing.T) {
	// a step with two action keys violates the one-action-per-step DSL rule
	errs, _ := ValidatePipelineConfig(cfgOf(map[string]interface{}{
		"validate_test_echo": map[string]interface{}{"message": "x"},
		"extra_key":          map[string]interface{}{"y": 2},
	}))
	if len(errs) == 0 {
		t.Fatal("expected rejection for multi-action step")
	}
}

// TestValidateHttpRequiresHosts and TestValidateSmtpRequiresServer pin
// the required-field checks added by the empty-config audit.
func TestValidateHttpRequiresHosts(t *testing.T) {
	errs, _ := ValidatePipelineConfig(cfgOf(map[string]interface{}{
		"http": map[string]interface{}{"path": "/x"},
	}))
	if len(errs) == 0 || !strings.Contains(errs[0].Message, "requires at least one host") {
		t.Fatalf("expected hosts requirement, got %+v", errs)
	}
}

func TestValidateSmtpRequiresServer(t *testing.T) {
	errs, _ := ValidatePipelineConfig(cfgOf(map[string]interface{}{
		"smtp": map[string]interface{}{},
	}))
	if len(errs) == 0 || !strings.Contains(errs[0].Message, "requires at least one server") {
		t.Fatalf("expected server requirement, got %+v", errs)
	}
}

// TestRuntimeSnapshotReportsFailures pins instance-side failure
// visibility: a config that fails to compile shows up as FAILED with
// the reason instead of silently disappearing from the runtime report.
func TestRuntimeSnapshotReportsFailures(t *testing.T) {
	m := &PipeModule{}
	m.recordFailure("ghost-pipeline", errNew("the processor nope does not exist"))

	snap := m.RuntimeSnapshot()
	got, ok := snap["ghost-pipeline"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected failure entry, got %v", snap)
	}
	if got["state"] != "FAILED" {
		t.Fatalf("expected FAILED, got %v", got["state"])
	}
	if got["reason"] != "the processor nope does not exist" {
		t.Fatalf("expected reason, got %v", got["reason"])
	}

	// a pipeline that starts clears its stale failure entry
	m.recordSuccess("ghost-pipeline")
	snap = m.RuntimeSnapshot()
	if _, still := snap["ghost-pipeline"]; still {
		t.Fatal("failure entry should clear after success")
	}
}
