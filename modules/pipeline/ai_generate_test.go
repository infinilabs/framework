/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infini.ltd */

package pipeline

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/util"
)

// stubGenerator is the ChainGenerator test double.
type stubGenerator struct {
	name string
	err  error
}

func (s stubGenerator) Name() string { return s.name }
func (s stubGenerator) Generate(req ChainGenerateRequest) ([]map[string]interface{}, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []map[string]interface{}{{"test_upper": util.MapStr{}}}, nil
}

func TestChainGeneratorRegistry(t *testing.T) {
	if err := RegisterChainGeneratorOrReplace(stubGenerator{name: "test_gen_a"}); err != nil {
		t.Fatal(err)
	}
	if err := RegisterChainGeneratorOrReplace(nil); err == nil {
		t.Fatal("nil generator must be rejected")
	}
	if err := RegisterChainGeneratorOrReplace(stubGenerator{name: ""}); err == nil {
		t.Fatal("empty name must be rejected")
	}
	if g, ok := GetChainGenerator("test_gen_a"); !ok || g.Name() != "test_gen_a" {
		t.Fatal("registered generator must resolve")
	}
	if _, ok := GetChainGenerator("missing"); ok {
		t.Fatal("unregistered name must not resolve")
	}
	if err := RegisterChainGeneratorOrReplace(stubGenerator{name: "test_gen_a", err: fmt.Errorf("v2")}); err != nil {
		t.Fatal(err)
	}
	if g, _ := GetChainGenerator("test_gen_a"); g.(stubGenerator).err == nil {
		t.Fatal("replace must overwrite")
	}
}

func callAIGenerate(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	m := &PipeModule{}
	req := httptest.NewRequest(http.MethodPost, "/pipeline/ai-generate", strings.NewReader(body))
	w := httptest.NewRecorder()
	m.postAIGenerateHandler(w, req, httprouter.Params{})
	return w
}

func TestAIGenerateHandler_UnimplementedWithoutGenerator(t *testing.T) {
	// no generator carries this unique name; simulate a bare build by
	// asking for a name that cannot exist
	w := callAIGenerate(t, `{"generator":"__none__","documents":["{}"]}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown generator must be 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "unknown chain generator") {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestAIGenerateHandler_HappyPathAndValidation(t *testing.T) {
	if err := RegisterChainGeneratorOrReplace(stubGenerator{name: "test_gen_ai"}); err != nil {
		t.Fatal(err)
	}

	w := callAIGenerate(t, `{"generator":"test_gen_ai","documents":["{\"title\":\"a\"}"],"requirements":"add tags"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("generate must succeed, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"test_upper"`) || !strings.Contains(w.Body.String(), `"generator":"test_gen_ai"`) {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}

	// empty documents → 400
	if w := callAIGenerate(t, `{"documents":[]}`); w.Code != http.StatusBadRequest {
		t.Fatalf("empty documents must be 400, got %d", w.Code)
	}
	// bad document JSON → 400
	if w := callAIGenerate(t, `{"documents":["not-json"]}`); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid document JSON must be 400, got %d", w.Code)
	}
	// bad mode → 400
	if w := callAIGenerate(t, `{"documents":["{}"],"mode":"nope"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("bad mode must be 400, got %d", w.Code)
	}
	// refine without current_chain → 400
	if w := callAIGenerate(t, `{"documents":["{}"],"mode":"refine"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("refine without chain must be 400, got %d", w.Code)
	}
	// generator failure → 502
	if err := RegisterChainGeneratorOrReplace(stubGenerator{name: "test_gen_ai", err: fmt.Errorf("llm down")}); err != nil {
		t.Fatal(err)
	}
	if w := callAIGenerate(t, `{"generator":"test_gen_ai","documents":["{}"]}`); w.Code != http.StatusBadGateway {
		t.Fatalf("generator error must be 502, got %d", w.Code)
	}
	// restore the working stub
	if err := RegisterChainGeneratorOrReplace(stubGenerator{name: "test_gen_ai"}); err != nil {
		t.Fatal(err)
	}
}

// TestAIGenerateHandler_AmbiguousDefault: with a second generator the
// empty-name default must refuse instead of guessing.
func TestAIGenerateHandler_AmbiguousDefault(t *testing.T) {
	if err := RegisterChainGeneratorOrReplace(stubGenerator{name: "test_gen_b"}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		chainGenMu.Lock()
		delete(chainGens, "test_gen_b")
		chainGenMu.Unlock()
	}()
	w := callAIGenerate(t, `{"documents":["{}"]}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("ambiguous default must be 400, got %d: %s", w.Code, w.Body.String())
	}
	// explicit name still works
	w = callAIGenerate(t, `{"generator":"test_gen_ai","documents":["{}"]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("explicit generator must work, got %d", w.Code)
	}
}
