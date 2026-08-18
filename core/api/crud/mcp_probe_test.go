package crud

import (
	"net/http"
	"testing"

	httprouter "infini.sh/framework/core/api/router"

	"infini.sh/framework/core/api"
)

func TestMCPProbe(t *testing.T) {
	api.HandleUIMethod(api.POST, "/mcpprobe/", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {},
		api.RequirePermission("generic#x/read"), api.MCPTool("probe_create", "probe"))

	api.HandleUIMethod(api.POST, "/mcpprobe2/", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {},
		api.MCPTool("probe2_create", "probe2"))

	found := map[string]bool{}
	api.WalkMCPAutoUIMethodRoutes(func(route api.RegisteredUIMethodRoute) {
		if route.Route.Path == "/mcpprobe/" || route.Route.Path == "/mcpprobe2/" {
			found[route.Route.Path] = true
		}
	})
	if !found["/mcpprobe/"] || !found["/mcpprobe2/"] {
		t.Fatalf("MCP routes not walked: %+v", found)
	}
	t.Log("MCP option + walk OK")
}
