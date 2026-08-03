package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/util"
)

func TestAuthorizeMCPToolRequest(t *testing.T) {
	RegisterMCPToolAuthorizer(nil)
	t.Cleanup(func() {
		RegisterMCPToolAuthorizer(nil)
	})

	if !authorizeMCPToolRequest(nil, &HandlerOptions{}) {
		t.Fatal("expected allow when no authorizer is registered")
	}

	RegisterMCPToolAuthorizer(func(headers http.Header, options *HandlerOptions) bool {
		return headers.Get("X-Allow") == "1"
	})

	if authorizeMCPToolRequest(http.Header{}, &HandlerOptions{}) {
		t.Fatal("expected deny when authorizer rejects headers")
	}

	if !authorizeMCPToolRequest(http.Header{"X-Allow": []string{"1"}}, &HandlerOptions{}) {
		t.Fatal("expected allow when authorizer accepts headers")
	}
}

func TestMCPAutoUIRouteCallsRegisteredHandler(t *testing.T) {
	handler := RegisteredAPIHandler{
		Options: &HandlerOptions{},
		Handler: func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
			var body map[string]interface{}
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			WriteJSON(w, util.MapStr{
				"id":            ps.MustGetParameter("id"),
				"q":             req.URL.Query().Get("q"),
				"authorization": req.Header.Get("Authorization"),
				"name":          body["name"],
			}, http.StatusCreated)
		},
	}

	result, err := callMCPAutoUIRoute(POST, "/things/:id", handler, mcp.CallToolRequest{
		Header: http.Header{"Authorization": []string{"Bearer token"}},
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{
				"path_params": map[string]interface{}{"id": "thing-1"},
				"query":       map[string]interface{}{"q": "search"},
				"body":        map[string]interface{}{"name": "test"},
			},
		},
	})
	if err != nil {
		t.Fatalf("call MCP auto UI route: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected successful MCP result: %#v", result)
	}

	structured, ok := result.StructuredContent.(util.MapStr)
	if !ok {
		t.Fatalf("expected structured map result, got %T", result.StructuredContent)
	}
	if structured["status_code"] != http.StatusCreated {
		t.Fatalf("unexpected status code: %#v", structured["status_code"])
	}

	jsonBody, ok := structured["json"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected JSON response body, got %#v", structured["json"])
	}
	if jsonBody["id"] != "thing-1" || jsonBody["q"] != "search" || jsonBody["authorization"] != "Bearer token" || jsonBody["name"] != "test" {
		t.Fatalf("unexpected JSON response: %#v", jsonBody)
	}
}

func TestNewMCPAutoToolUsesCustomSchemas(t *testing.T) {
	tool := newMCPAutoTool("custom_tool", "Custom tool", &HandlerOptions{Labels: util.MapStr{
		MCPToolInputSchema:  json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`),
		MCPToolOutputSchema: json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}}}`),
	}})

	if string(tool.RawInputSchema) == "" {
		t.Fatal("expected custom input schema")
	}
	if string(tool.RawOutputSchema) == "" {
		t.Fatal("expected custom output schema")
	}
}
