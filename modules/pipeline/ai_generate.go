/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infini.ltd */

package pipeline

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"

	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/util"
)

// Chain generation is product logic (LLM backend, prompts, processor
// catalog wording); the endpoint is not. Products register a
// ChainGenerator at init and the unified POST /pipeline/ai-generate stays
// product-agnostic — the same shape as the unified dry-run endpoint.

// ChainGenerateRequest is the product-agnostic input: parsed sample
// documents, a natural-language requirement and (refine mode) the chain
// to edit.
type ChainGenerateRequest struct {
	Context      context.Context
	Documents    []util.MapStr
	Requirements string
	// Mode: "generate" (default) | "refine"
	Mode         string
	CurrentChain []map[string]interface{}
}

// ChainGenerator turns samples plus a requirement into a processor chain
// (entries in the same shape dry-run takes: single-key processor maps or
// if/then/else). Generators return raw entries; validation and replay
// stay with the caller (the unified dry-run endpoint).
type ChainGenerator interface {
	Name() string
	Generate(req ChainGenerateRequest) ([]map[string]interface{}, error)
}

var (
	chainGenMu sync.RWMutex
	chainGens  = map[string]ChainGenerator{}
)

// RegisterChainGeneratorOrReplace registers (or replaces) a generator by
// name; the empty name is reserved for the framework.
func RegisterChainGeneratorOrReplace(g ChainGenerator) error {
	if g == nil || g.Name() == "" {
		return fmt.Errorf("chain generator must have a non-empty name")
	}
	chainGenMu.Lock()
	defer chainGenMu.Unlock()
	chainGens[g.Name()] = g
	return nil
}

// GetChainGenerator resolves a generator by name.
func GetChainGenerator(name string) (ChainGenerator, bool) {
	chainGenMu.RLock()
	defer chainGenMu.RUnlock()
	g, ok := chainGens[name]
	return g, ok
}

// ChainGeneratorNames lists registered generators (sorted).
func ChainGeneratorNames() []string {
	chainGenMu.RLock()
	defer chainGenMu.RUnlock()
	names := make([]string, 0, len(chainGens))
	for n := range chainGens {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// postAIGenerateHandler — POST /pipeline/ai-generate.
// body: {generator?, documents: [raw-JSON-string...], requirements?,
// mode?, current_chain?} → {chain} or {chain, generators} on 501/400.
// 501 when no generator is registered; 400 for bad input or an ambiguous
// empty generator name with multiple registrations; 502 when the
// generator fails. Validation/replay is the caller's follow-up call to
// /pipeline/dry-run.
func (module *PipeModule) postAIGenerateHandler(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var body struct {
		Generator    string                   `json:"generator"`
		Documents    []string                 `json:"documents"`
		Requirements string                   `json:"requirements"`
		Mode         string                   `json:"mode"`
		CurrentChain []map[string]interface{} `json:"current_chain"`
	}
	if err := module.DecodeJSON(req, &body); err != nil {
		module.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(body.Documents) == 0 {
		module.WriteError(w, "documents (non-empty string array of sample document JSON) is required", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(body.Generator)
	if name == "" {
		names := ChainGeneratorNames()
		switch len(names) {
		case 0:
			module.WriteError(w, "no chain generator registered in this build", http.StatusNotImplemented)
			return
		case 1:
			name = names[0]
		default:
			module.WriteError(w, "multiple chain generators registered, specify one of: "+strings.Join(names, ", "), http.StatusBadRequest)
			return
		}
	}
	gen, ok := GetChainGenerator(name)
	if !ok {
		module.WriteError(w, "unknown chain generator "+name+" (registered: "+strings.Join(ChainGeneratorNames(), ", ")+")", http.StatusBadRequest)
		return
	}

	mode := body.Mode
	if mode == "" {
		mode = "generate"
	}
	if mode != "generate" && mode != "refine" {
		module.WriteError(w, "mode must be generate or refine", http.StatusBadRequest)
		return
	}
	if mode == "refine" && len(body.CurrentChain) == 0 {
		module.WriteError(w, "refine requires current_chain", http.StatusBadRequest)
		return
	}

	// sample documents must be JSON objects — generators promise real docs
	docs := make([]util.MapStr, 0, len(body.Documents))
	for i, raw := range body.Documents {
		var doc util.MapStr
		if err := util.FromJSONBytes([]byte(raw), &doc); err != nil {
			module.WriteError(w, fmt.Sprintf("documents[%d] is not a valid JSON object", i), http.StatusBadRequest)
			return
		}
		docs = append(docs, doc)
	}

	chain, err := gen.Generate(ChainGenerateRequest{
		Context:      req.Context(),
		Documents:    docs,
		Requirements: body.Requirements,
		Mode:         mode,
		CurrentChain: body.CurrentChain,
	})
	if err != nil {
		module.WriteError(w, truncateGenError(err.Error()), http.StatusBadGateway)
		return
	}
	module.WriteJSON(w, util.MapStr{"chain": chain, "generator": name}, http.StatusOK)
}

func truncateGenError(s string) string {
	const cap = 300
	if len(s) <= cap {
		return s
	}
	return s[:cap] + "…"
}
