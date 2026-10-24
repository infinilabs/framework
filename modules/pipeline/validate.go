/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"fmt"
	"net/http"

	log "github.com/cihub/seelog"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/util"
)

// ConfigError locates one validation finding in a pipeline config.
type ConfigError struct {
	// Stage is the 0-based processor index; -1 means chain/config level.
	Stage     int    `json:"stage"`
	Processor string `json:"processor,omitempty"` // offending processor key ("if" for conditionals)
	Field     string `json:"field,omitempty"`
	Message   string `json:"message"`
}

// ValidatePipelineConfig compiles the processor chain WITHOUT running it:
// DSL shape, processor registry existence, parameter unpacking, nested
// chains (for_each / lane_consumer hosts) and record-contract placement.
//
// It is the gate before a config is persisted or deployed — invalid input
// must be rejected at the control plane, not discovered on the instance
// after distribution. Compile problems are returned as errors (reject);
// record-contract mismatches are warnings unless the config opts into
// StrictContract, matching the runtime compile semantics.
func ValidatePipelineConfig(cfg *pipeline.PipelineConfigV2) (errs []ConfigError, warns []ConfigError) {
	if cfg == nil {
		errs = append(errs, ConfigError{Stage: -1, Message: "config is required"})
		return
	}
	if cfg.Name == "" {
		errs = append(errs, ConfigError{Stage: -1, Field: "name", Message: "name is required"})
	}
	if len(cfg.Processors) == 0 {
		errs = append(errs, ConfigError{Stage: -1, Field: "processor", Message: "at least one processor is required"})
		return
	}

	procCfgs, err := cfg.GetProcessorsConfig()
	if err != nil {
		errs = append(errs, ConfigError{Stage: -1, Field: "processor", Message: err.Error()})
		return
	}

	// per-stage compile so errors carry the offending step index; a
	// panicking constructor becomes a validation error instead of
	// crashing the gate (compile must never take the control plane down)
	for i, step := range procCfgs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					errs = append(errs, ConfigError{
						Stage:     i,
						Processor: stepKey(step),
						Message:   fmt.Sprintf("constructor panicked: %v", r),
					})
				}
			}()
			if _, err := pipeline.NewPipeline([]*config.Config{step}); err != nil {
				errs = append(errs, ConfigError{
					Stage:     i,
					Processor: stepKey(step),
					Message:   err.Error(),
				})
			}
		}()
	}
	// whole-chain compile catches issues the standalone steps cannot see
	if _, err := pipeline.NewPipeline(procCfgs); err != nil && len(errs) == 0 {
		errs = append(errs, ConfigError{Stage: -1, Message: err.Error()})
	}

	// contract placement pass: strict compile surfaces mismatches; severity
	// follows the config's own StrictContract opt-in (default warn-only).
	// Skipped when the chain already failed to compile — the strict run
	// would only duplicate those errors, not reveal contract issues.
	if len(errs) == 0 {
		if _, err := pipeline.NewPipelineWithScope(procCfgs,
			pipeline.ScopeInfo{Kind: pipeline.ScopeBatch, Strict: true}, false); err != nil {
			ce := ConfigError{Stage: -1, Message: err.Error()}
			if cfg.StrictContract != nil && *cfg.StrictContract {
				errs = append(errs, ce)
			} else {
				warns = append(warns, ce)
			}
		}
	}
	return errs, warns
}

// stepKey names the processor a step config declares ("if" for
// conditionals, else the single action key).
func stepKey(step *config.Config) string {
	fields := step.GetFields()
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// validateAndReject writes a structured 400 when the config does not
// compile; returns true when the request may proceed. The body carries
// both the standard error wrapper (UIs toast `error.reason`) and the
// per-stage detail list for richer editors.
func (module *PipeModule) validateAndReject(w http.ResponseWriter, cfg *pipeline.PipelineConfigV2) bool {
	errs, warns := ValidatePipelineConfig(cfg)
	if len(errs) == 0 {
		return true
	}
	summary := "invalid pipeline config: " + errs[0].Message
	// the registry error lists every valid processor — keep the toast-size
	// summary short; errors[] carries the full detail
	if len(summary) > 300 {
		summary = summary[:300] + "..."
	}
	if len(errs) > 1 {
		summary += fmt.Sprintf(" (+%d more)", len(errs)-1)
	}
	module.WriteJSON(w, util.MapStr{
		"error":    util.MapStr{"reason": summary},
		"status":   http.StatusBadRequest,
		"errors":   errs,
		"warnings": warns,
	}, http.StatusBadRequest)
	_ = log.Warnf("rejected invalid pipeline config %q: %d error(s)", cfg.Name, len(errs))
	return false
}

// validatePipelineHandler — POST /pipeline/_validate
// Pre-flight report for editors: compiles the submitted config without
// persisting anything. Always 200; the `valid` flag carries the verdict.
func (module *PipeModule) validatePipelineHandler(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var obj pipeline.PipelineConfigV2
	if err := module.DecodeJSON(req, &obj); err != nil {
		module.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}
	errs, warns := ValidatePipelineConfig(&obj)
	module.WriteJSON(w, util.MapStr{
		"valid":    len(errs) == 0,
		"errors":   errs,
		"warnings": warns,
	}, http.StatusOK)
}
