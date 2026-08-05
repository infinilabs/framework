---
title: "MCP Server"
weight: 55
---

# MCP (Model Context Protocol) Server

The INFINI Framework provides built-in [Model Context Protocol](https://modelcontextprotocol.io/) (MCP) server support, enabling AI assistants like Claude Desktop, Cursor, and any MCP-compatible client to interact with your application's APIs as callable tools.

## Overview

The framework offers two complementary patterns for exposing MCP tools:

- **Auto-exposure (recommended)**: Annotate existing REST routes with `api.MCPTool()` — the framework automatically generates tool schemas, handles JSON-RPC dispatch, and enforces per-tool RBAC.
- **Standalone server**: Build a custom `MCPServer` with typed tool schemas for fine-grained control over parameter descriptions, read-only hints, and loopback handlers.

Both patterns mount on the Streamable HTTP transport at `/mcp` by default.

## Quick Start

### 1. Enable MCP in config

```yaml
web:
  mcp:
    enabled: true
    # base_path: /mcp       # default endpoint path
    # stateless: true       # stateless mode (no session management)
    # name: "My App"        # server name shown to MCP clients
    # version: "1.0.0"      # server version
```

### 2. Annotate routes with MCPTool

```go
api.HandleUIMethod(api.GET, "/api/things", handler.ListThings, api.RequireLogin(),
    api.MCPTool("list_things", "List all things with optional filtering"))
```

### 3. Connect an MCP client

```json
{
  "mcpServers": {
    "my-app": {
      "url": "http://localhost:29000/mcp",
      "transport": "streamable-http"
    }
  }
}
```

That's it — the framework auto-mounts the MCP server, generates tool schemas from your routes, and enforces authentication.

---

## Auto-Exposure Mode

### How it works

When `web.mcp.enabled` is `true`, the framework:

1. Creates a `StreamableHTTPServer` and mounts it at `web.mcp.base_path` (default `/mcp`).
2. Walks all registered UI routes tagged with `MCPTool` (or `MCPAuto`).
3. For each tagged route, auto-generates a tool with:
   - **Name**: the `name` argument from `MCPTool()`, or auto-derived from method+path (e.g. `get_things_id`).
   - **Description**: the `description` argument, or a default.
   - **Input schema**: structured object with `path_params`, `query`, `headers`, `body`, `raw_body`.
4. Each tool call bridges to the real HTTP route via an in-process handler (no network hop).

### Minimal example — GET with no parameters

```go
api.HandleUIMethod(api.GET, "/api/status", handler.GetStatus, api.RequireLogin(),
    api.MCPTool("get_status", "Get the current system status"))
```

The MCP client sees a tool named `get_status` with no required arguments.

### Path parameters

```go
api.HandleUIMethod(api.GET, "/api/things/:id", handler.GetThing, api.RequireLogin(),
    api.MCPTool("get_thing", "Get details of a specific thing by ID"))
```

The tool auto-includes a `path_params` argument:
```json
{
  "path_params": { "id": "thing-123" }
}
```

### Query parameters

```go
api.HandleUIMethod(api.GET, "/api/things", handler.ListThings, api.RequireLogin(),
    api.MCPTool("list_things", "List things, optionally filtered by status"))
```

The client can pass:
```json
{
  "query": { "status": "active", "page": "1" }
}
```

### POST with body

```go
api.HandleUIMethod(api.POST, "/api/things", handler.CreateThing, api.RequireLogin(),
    api.MCPTool("create_thing", "Create a new thing"))
```

The client sends:
```json
{
  "body": { "name": "My Thing", "type": "example" }
}
```

### Auto-generated input schema

For each auto-exposed tool, the framework synthesizes a JSON-schema object:

| Property | Type | Description |
|---|---|---|
| `path_params` | `object` | URL path parameters (e.g. `{id}` from `/things/:id`) |
| `query` | `object` | URL query string parameters |
| `headers` | `object` | Custom HTTP headers to forward |
| `body` | `object` | JSON request body (parsed object) |
| `raw_body` | `string` | Raw request body (for non-JSON payloads) |

All properties are optional (`additionalProperties: true`). The MCP client chooses which to populate.

### Tool result structure

Each tool call returns a structured result:

```json
{
  "status_code": 200,
  "headers": { "Content-Type": "application/json" },
  "body": "{\"id\":\"thing-123\",\"name\":\"...\"}",
  "json": { "id": "thing-123", "name": "..." }
}
```

- `json` is populated only when the response body is valid JSON.
- HTTP status >= 400 sets `isError: true` on the tool result.

---

## Configuration Reference

All MCP settings live under `web.mcp` in the YAML config:

```yaml
web:
  mcp:
    enabled: true            # Enable the MCP server (default: false)
    base_path: /mcp          # HTTP endpoint path (default: /mcp)
    name: "My App"           # Server name shown to MCP clients
    version: "1.0.0"         # Server version
    stateless: true          # Stateless mode — no session tracking
    disable_streaming: false # Disable SSE streaming (use plain HTTP responses)
```

| Field | Default | Description |
|---|---|---|
| `enabled` | `false` | Master switch for the MCP server |
| `base_path` | `/mcp` | The HTTP path where the MCP endpoint is mounted |
| `name` | `"<AppName> UI"` | Server name in MCP protocol handshake |
| `version` | App version | Server version in MCP protocol handshake |
| `stateless` | `false` | Stateless mode — each request is independent, no session ID tracking |
| `disable_streaming` | `false` | Disable Server-Sent Events streaming transport |

---

## Permission Control

MCP tools inherit the same RBAC as their underlying routes. When a route is registered with `api.RequirePermission(...)`, the MCP authorization filter automatically:

1. **Hides** the tool from clients who lack permission (filtered out of `tools/list`).
2. **Blocks** calls to the tool with a "permission denied" error.

```go
api.HandleUIMethod(api.POST, "/api/things/:id/delete", handler.DeleteThing,
    api.RequireLogin(),
    api.RequirePermission("things", "delete"),
    api.MCPTool("delete_thing", "Delete a thing by ID"))
```

The built-in `MCPAuthorizer` filter validates the user's session from the MCP client's HTTP headers and checks permissions against the route's `RequirePermission` keys. Admin role bypasses all checks.

### Custom authorizer

Register a custom authorizer for advanced scenarios:

```go
api.RegisterMCPToolAuthorizer(func(headers http.Header, options *api.HandlerOptions) bool {
    // Return false to hide/block this tool for this request
    token := headers.Get("Authorization")
    return validateToken(token)
})
```

---

## Custom Tool Schemas

For routes where the auto-generated schema is too generic, provide explicit JSON schemas:

```go
api.HandleUIMethod(api.POST, "/api/search", handler.Search,
    api.RequireLogin(),
    api.MCPTool("search", "Full-text search across all documents"),
    api.Label(api.MCPToolInputSchema, json.RawMessage(`{
        "type": "object",
        "properties": {
            "query": { "type": "string", "description": "Search query text" },
            "limit": { "type": "integer", "description": "Max results (default 10)", "default": 10 }
        },
        "required": ["query"]
    }`)),
)
```

When a custom `MCPToolInputSchema` is provided, it replaces the auto-generated schema entirely.

---

## Standalone Server Mode

When you need full control over tool definitions — typed parameters, read-only hints, custom descriptions — build a standalone `MCPServer` and mount it manually.

### When to use this mode

- You want typed, named parameters (not generic `path_params`/`query`/`body`)
- You need `readOnlyHint` / `destructiveHint` annotations for GET vs POST
- You want a separate tool catalog from your REST routes
- You need loopback handlers that transform parameters before calling internal APIs

### Complete example

```go
import (
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
    "infini.sh/framework/core/api"
)

// 1. Define your tools with typed parameters.
type toolDef struct {
    name   string
    desc   string
    method string
    path   string
    params []paramDef
}

type paramDef struct {
    name     string
    desc     string
    required bool
}

var myTools = []toolDef{
    {
        name:   "search_logs",
        desc:   "Search logs by keyword and time range",
        method: "GET",
        path:   "/api/logs",
        params: []paramDef{
            {name: "q", desc: "Search keyword", required: true},
            {name: "from", desc: "Start time (ISO 8601)"},
            {name: "to", desc: "End time (ISO 8601)"},
        },
    },
}

// 2. Build the MCP server with typed tool schemas.
func buildMCPServer() *server.MCPServer {
    s := server.NewMCPServer("MyApp", "1.0.0",
        server.WithToolCapabilities(true),
        server.WithInstructions("MyApp API — manage things and search logs."),
    )
    for _, def := range myTools {
        def := def
        s.AddTool(buildTool(def), makeHandler(def))
    }
    return s
}

// 3. Generate typed tool schemas with read-only hints.
func buildTool(def toolDef) mcp.Tool {
    opts := []mcp.ToolOption{mcp.WithDescription(def.desc)}
    for _, p := range def.params {
        propOpts := []mcp.PropertyOption{mcp.Description(p.desc)}
        if p.required {
            propOpts = append(propOpts, mcp.Required())
        }
        opts = append(opts, mcp.WithString(p.name, propOpts...))
    }
    // Mark GET tools as read-only for client-side confirmation behavior.
    if def.method == "GET" {
        opts = append(opts,
            mcp.WithReadOnlyHintAnnotation(true),
            mcp.WithDestructiveHintAnnotation(false),
        )
    }
    return mcp.NewTool(def.name, opts...)
}

// 4. Bridge tool calls to your internal REST API via loopback HTTP.
func makeHandler(def toolDef) server.ToolHandlerFunc {
    return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        // Build URL with query params from typed arguments.
        params := req.Params.Arguments.(map[string]interface{})
        u, _ := url.Parse("http://127.0.0.1:29000" + def.path)
        q := u.Query()
        for _, p := range def.params {
            if v, ok := params[p.name]; ok {
                q.Set(p.name, fmt.Sprintf("%v", v))
            }
        }
        u.RawQuery = q.Encode()

        // Call your own REST API.
        resp, err := http.Get(u.String())
        if err != nil {
            return mcp.NewToolResultError(err.Error()), nil
        }
        defer resp.Body.Close()
        body, _ := io.ReadAll(resp.Body)
        if resp.StatusCode >= 400 {
            return mcp.NewToolResultError(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))), nil
        }
        return mcp.NewToolResultText(string(body)), nil
    }
}

// 5. Mount the server in your route registration.
func setupRoutes(apiServer *api.Handler) {
    mcpServer := server.NewStreamableHTTPServer(buildMCPServer(), server.WithStateLess(true))
    api.HandleUIFuncMethod(api.POST, "/mcp", mcpServer.ServeHTTP)
    // GET returns a help page for browser visitors.
    api.HandleUIFuncMethod(api.GET, "/mcp", serveHelpPage)
}
```

---

## Browser Compatibility

Since `/mcp` is an MCP endpoint (POST + JSON-RPC), browser GET requests will not produce a useful response. Register a GET handler that returns a helpful HTML page:

```go
api.HandleUIFuncMethod(api.GET, "/mcp", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`<h1>MCP Server</h1>
<p>This endpoint is for MCP clients. Use POST with Accept: application/json.</p>`))
})
```

For invalid POST requests (missing the MCP Accept header), use a global interceptor:

```go
type mcpInterceptor struct{}

func (i *mcpInterceptor) Match(r *http.Request) bool {
    if r.URL.Path != "/mcp" || r.Method != http.MethodPost {
        return false
    }
    accept := r.Header.Get("Accept")
    return !strings.Contains(accept, "application/json") &&
           !strings.Contains(accept, "text/event-stream")
}

func (i *mcpInterceptor) PreHandle(ctx context.Context, w http.ResponseWriter, r *http.Request) (context.Context, error) {
    // Return HTML help page for non-MCP POST requests.
    serveHelpPage(w, r)
    return ctx, fmt.Errorf("response sent") // stop the handler chain
}

func (i *mcpInterceptor) PostHandle(_ context.Context, _ http.ResponseWriter, _ *http.Request) {}
func (i *mcpInterceptor) Name() string { return "mcp-browser-compat" }

// Register:
api.AddGlobalInterceptors(&mcpInterceptor{})
```

---

## Client Configuration

### Claude Desktop

Edit `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "my-app": {
      "url": "http://localhost:29000/mcp",
      "transport": "streamable-http"
    }
  }
}
```

### Cursor

Add to Cursor's MCP settings:

```json
{
  "mcpServers": {
    "my-app": {
      "url": "http://localhost:29000/mcp"
    }
  }
}
```

### curl

Test the MCP server with curl:

```bash
# Initialize handshake
curl -X POST http://localhost:29000/mcp \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2024-11-05",
      "capabilities": {},
      "clientInfo": {"name": "test", "version": "1.0"}
    }
  }'

# List available tools
curl -X POST http://localhost:29000/mcp \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/list",
    "params": {}
  }'

# Call a tool
curl -X POST http://localhost:29000/mcp \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{
    "jsonrpc": "2.0",
    "id": 3,
    "method": "tools/call",
    "params": {
      "name": "list_things",
      "arguments": {
        "query": { "status": "active" }
      }
    }
  }'
```

---

## Testing

### Unit test pattern

Test auto-exposed MCP routes using the internal `callMCPAutoUIRoute` function:

```go
func TestMyMCPTool(t *testing.T) {
    // Register the route (done in your init/setup).
    api.HandleUIMethod(api.GET, "/api/things/:id", testHandler,
        api.MCPTool("get_thing", "Get a thing"))

    // Simulate a tool call.
    result, err := api.CallMCPAutoUIRoute("get_thing", mcp.CallToolParams{
        Arguments: map[string]interface{}{
            "path_params": map[string]interface{}{"id": "thing-1"},
            "query":       map[string]interface{}{"detail": "full"},
        },
    })

    require.NoError(t, err)
    require.False(t, result.IsError)

    // The structured content contains the HTTP response.
    content := result.Content[0].(mcp.TextContent)
    assert.Contains(t, content.Text, "thing-1")
}
```

### Testing authorization

```go
func TestMCPToolRequiresAuth(t *testing.T) {
    // Without auth headers, the tool call should be denied.
    result, _ := api.CallMCPAutoUIRoute("get_thing", mcp.CallToolParams{
        Arguments: map[string]interface{}{
            "path_params": map[string]interface{}{"id": "thing-1"},
        },
    })
    assert.True(t, result.IsError)
}
```

---

## API Reference

### Options

| Function | Description |
|---|---|
| `api.MCPTool(name, description string)` | Mark a route as an MCP tool with a custom name and description |
| `api.MCPAuto()` | Mark a route for auto MCP exposure (name/description auto-generated) |
| `api.Label(api.MCPToolInputSchema, schema)` | Override the auto-generated input schema |
| `api.Label(api.MCPToolOutputSchema, schema)` | Set a custom output schema |
| `api.RequirePermission(resource, action)` | Enforce per-tool RBAC (inherited by MCP layer) |

### Server functions

| Function | Description |
|---|---|
| `api.RegisterMCPToolAuthorizer(fn)` | Register a custom tool authorization filter |
| `api.WalkMCPAutoUIMethodRoutes(fn)` | Iterate over all MCP-tagged routes (for introspection) |
| `api.WalkRegisteredUIMethodRoutes(fn)` | Iterate over all registered UI routes |

### Constants

| Constant | Description |
|---|---|
| `api.FeatureMCPAuto` | Feature flag for auto MCP exposure |
| `api.MCPToolName` | Label key for tool name |
| `api.MCPToolDescription` | Label key for tool description |
| `api.MCPToolInputSchema` | Label key for custom input schema |
| `api.MCPToolOutputSchema` | Label key for custom output schema |
