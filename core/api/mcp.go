package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"

	log "github.com/cihub/seelog"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
)

var (
	mcpAutoMutex  sync.Mutex
	mcpAutoServer *mcpserver.MCPServer
)

type mcpRouteArguments struct {
	PathParams map[string]interface{} `json:"path_params"`
	Query      map[string]interface{} `json:"query"`
	Headers    map[string]interface{} `json:"headers"`
	Body       interface{}            `json:"body"`
	RawBody    string                 `json:"raw_body"`
}

type mcpAuthContextKey struct{}

type mcpToolMetadata struct {
	Options *HandlerOptions
}

var (
	mcpAutoToolMetadata = map[string]mcpToolMetadata{}

	mcpToolAuthorizerMutex sync.RWMutex
	mcpToolAuthorizer      func(headers http.Header, options *HandlerOptions) bool
)

// RegisterMCPToolAuthorizer registers a callback used to authorize MCP tool visibility and invocation.
// Returning false from authorizer blocks both listing and calling the tool for the current request headers.
func RegisterMCPToolAuthorizer(authorizer func(headers http.Header, options *HandlerOptions) bool) {
	mcpToolAuthorizerMutex.Lock()
	mcpToolAuthorizer = authorizer
	mcpToolAuthorizerMutex.Unlock()
}

func registerMCPAutoUIHandler(cfg config.WebAppConfig) {
	if !cfg.MCP.Enabled {
		return
	}

	name := strings.TrimSpace(cfg.MCP.Name)
	if name == "" {
		name = global.Env().GetAppCapitalName() + " UI"
	}
	version := strings.TrimSpace(cfg.MCP.Version)
	if version == "" {
		version = global.Env().GetVersion()
	}

	server := mcpserver.NewMCPServer(name, version, mcpserver.WithToolFilter(filterMCPAutoToolsByPermission))
	mcpAutoMutex.Lock()
	mcpAutoServer = server
	mcpAutoToolMetadata = map[string]mcpToolMetadata{}
	mcpAutoMutex.Unlock()

	WalkMCPAutoUIMethodRoutes(func(route RegisteredUIMethodRoute) {
		addMCPAutoUIMethodTool(server, route.Route.Method, route.Route.Path, RegisteredAPIHandler{Handler: route.Handler, Options: route.Options})
	})

	opts := []mcpserver.StreamableHTTPOption{}
	if cfg.MCP.Stateless {
		opts = append(opts, mcpserver.WithStateLess(true))
	}
	if cfg.MCP.DisableStreaming {
		opts = append(opts, mcpserver.WithDisableStreaming(true))
	}

	path := strings.TrimSpace(cfg.MCP.BasePath)
	if path == "" {
		path = "/mcp"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	path = strings.TrimRight(path, "/")
	if path == "" {
		path = "/mcp"
	}

	log.Infof("register MCP auto UI handler: %s", path)
	opts = append(opts, mcpserver.WithHTTPContextFunc(func(ctx context.Context, r *http.Request) context.Context {
		return context.WithValue(ctx, mcpAuthContextKey{}, r.Header.Clone())
	}))
	uiServeMux.Handle(path, mcpserver.NewStreamableHTTPServer(server, opts...))
}

func registerMCPAutoUIMethodTool(method Method, pattern string, handler RegisteredAPIHandler) {
	if handler.Options == nil || !handler.Options.Feature(FeatureMCPAuto) {
		return
	}

	mcpAutoMutex.Lock()
	server := mcpAutoServer
	mcpAutoMutex.Unlock()
	if server == nil {
		return
	}

	addMCPAutoUIMethodTool(server, method, pattern, handler)
}

func addMCPAutoUIMethodTool(server *mcpserver.MCPServer, method Method, pattern string, handler RegisteredAPIHandler) {
	if server == nil || handler.Handler == nil || handler.Options == nil || !handler.Options.Feature(FeatureMCPAuto) {
		return
	}

	toolName := getMCPAutoToolName(method, pattern, handler.Options)
	description := getMCPAutoToolDescription(method, pattern, handler.Options)
	mcpAutoMutex.Lock()
	mcpAutoToolMetadata[toolName] = mcpToolMetadata{Options: cloneHandlerOptions(handler.Options)}
	mcpAutoMutex.Unlock()
	server.AddTool(newMCPAutoTool(toolName, description, handler.Options), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return callMCPAutoUIRoute(method, pattern, handler, request)
	})
}

func newMCPAutoTool(name, description string, options *HandlerOptions) mcp.Tool {
	var tool mcp.Tool
	if rawSchema := getRawJSONLabel(options, MCPToolInputSchema); len(rawSchema) > 0 {
		tool = mcp.NewToolWithRawSchema(name, description, rawSchema)
	} else {
		tool = mcp.NewToolWithRawSchema(name, description, json.RawMessage(`{
			"type":"object",
			"properties":{
				"path_params":{"type":"object","additionalProperties":true,"description":"Values for route parameters such as :id."},
				"query":{"type":"object","additionalProperties":true,"description":"Query string parameters."},
				"headers":{"type":"object","additionalProperties":true,"description":"Additional request headers."},
				"body":{"description":"JSON request body for POST, PUT, PATCH, or DELETE handlers."},
				"raw_body":{"type":"string","description":"Raw request body. Used when body is not set."}
			},
			"additionalProperties":false
		}`))
	}

	if rawOutputSchema := getRawJSONLabel(options, MCPToolOutputSchema); len(rawOutputSchema) > 0 {
		tool.RawOutputSchema = rawOutputSchema
	}
	return tool
}

func callMCPAutoUIRoute(method Method, pattern string, handler RegisteredAPIHandler, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !authorizeMCPToolRequest(request.Header, handler.Options) {
		return mcp.NewToolResultError("permission denied"), nil
	}

	arguments, err := decodeMCPRouteArguments(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	path, params, err := buildMCPAutoRequestPath(pattern, arguments.PathParams)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	body, err := encodeMCPAutoRequestBody(arguments)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	requestURL := &url.URL{Path: path, RawQuery: encodeMCPAutoQuery(arguments.Query)}
	req := httptest.NewRequest(string(method), requestURL.String(), bytes.NewReader(body))
	copyMCPAutoHeaders(req.Header, request.Header)
	copyMCPAutoHeaderMap(req.Header, arguments.Headers)
	if len(body) > 0 && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	recorder := httptest.NewRecorder()
	getWrappedHandler(string(method), pattern, handler)(recorder, req, params)
	response := recorder.Result()
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := util.MapStr{
		"status_code": response.StatusCode,
		"headers":     response.Header,
		"body":        string(responseBody),
	}
	var jsonBody interface{}
	if len(responseBody) > 0 && json.Unmarshal(responseBody, &jsonBody) == nil {
		result["json"] = jsonBody
	}

	if response.StatusCode >= http.StatusBadRequest {
		toolResult := mcp.NewToolResultStructured(result, string(responseBody))
		toolResult.IsError = true
		return toolResult, nil
	}
	return mcp.NewToolResultStructured(result, string(responseBody)), nil
}

func decodeMCPRouteArguments(request mcp.CallToolRequest) (mcpRouteArguments, error) {
	var arguments mcpRouteArguments
	raw := request.GetRawArguments()
	if raw == nil {
		return arguments, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return arguments, fmt.Errorf("failed to marshal MCP tool arguments: %w", err)
	}
	if err := json.Unmarshal(data, &arguments); err != nil {
		return arguments, fmt.Errorf("failed to decode MCP tool arguments: %w", err)
	}
	return arguments, nil
}

func buildMCPAutoRequestPath(pattern string, pathParams map[string]interface{}) (string, httprouter.Params, error) {
	path := pattern
	matches := routeParamPattern.FindAllStringSubmatch(pattern, -1)
	params := make(httprouter.Params, 0, len(matches))
	for _, match := range matches {
		name := match[1]
		value, ok := pathParams[name]
		if !ok {
			return "", nil, fmt.Errorf("missing required path parameter %q", name)
		}
		valueString := fmt.Sprint(value)
		params = append(params, httprouter.Param{Key: name, Value: valueString})
		path = strings.ReplaceAll(path, ":"+name, url.PathEscape(valueString))
	}
	return path, params, nil
}

func encodeMCPAutoRequestBody(arguments mcpRouteArguments) ([]byte, error) {
	if arguments.Body != nil {
		return json.Marshal(arguments.Body)
	}
	if arguments.RawBody != "" {
		return []byte(arguments.RawBody), nil
	}
	return nil, nil
}

func encodeMCPAutoQuery(query map[string]interface{}) string {
	values := url.Values{}
	for key, value := range query {
		switch typed := value.(type) {
		case []interface{}:
			for _, item := range typed {
				values.Add(key, fmt.Sprint(item))
			}
		default:
			values.Set(key, fmt.Sprint(value))
		}
	}
	return values.Encode()
}

func copyMCPAutoHeaders(target http.Header, source http.Header) {
	for key, values := range source {
		for _, value := range values {
			target.Add(key, value)
		}
	}
}

func copyMCPAutoHeaderMap(target http.Header, headers map[string]interface{}) {
	for key, value := range headers {
		switch typed := value.(type) {
		case []interface{}:
			for _, item := range typed {
				target.Add(key, fmt.Sprint(item))
			}
		default:
			target.Set(key, fmt.Sprint(value))
		}
	}
}

func getMCPAutoToolName(method Method, pattern string, options *HandlerOptions) string {
	if options != nil && options.Labels != nil {
		if value, ok := options.Labels[MCPToolName]; ok {
			if name := strings.TrimSpace(fmt.Sprint(value)); name != "" {
				return sanitizeMCPToolName(name)
			}
		}
	}
	return sanitizeMCPToolName(strings.ToLower(string(method)) + "_" + strings.Trim(pattern, "/"))
}

func getMCPAutoToolDescription(method Method, pattern string, options *HandlerOptions) string {
	if options != nil && options.Labels != nil {
		if value, ok := options.Labels[MCPToolDescription]; ok {
			if description := strings.TrimSpace(fmt.Sprint(value)); description != "" {
				return description
			}
		}
	}
	return fmt.Sprintf("Call %s %s", method, pattern)
}

func getRawJSONLabel(options *HandlerOptions, label string) json.RawMessage {
	if options == nil || options.Labels == nil {
		return nil
	}
	value, ok := options.Labels[label]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case json.RawMessage:
		return typed
	case []byte:
		return json.RawMessage(typed)
	case string:
		return json.RawMessage(typed)
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return nil
		}
		return data
	}
}

func sanitizeMCPToolName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, ":", "_")
	name = mcpToolNamePattern.ReplaceAllString(name, "_")
	name = strings.Trim(name, "_")
	if name == "" {
		return "ui_handler"
	}
	return name
}

func filterMCPAutoToolsByPermission(ctx context.Context, tools []mcp.Tool) []mcp.Tool {
	headers, _ := ctx.Value(mcpAuthContextKey{}).(http.Header)
	if headers == nil {
		headers = http.Header{}
	}

	mcpAutoMutex.Lock()
	metadata := make(map[string]mcpToolMetadata, len(mcpAutoToolMetadata))
	for name, entry := range mcpAutoToolMetadata {
		metadata[name] = entry
	}
	mcpAutoMutex.Unlock()

	filtered := make([]mcp.Tool, 0, len(tools))
	for _, tool := range tools {
		entry, ok := metadata[tool.Name]
		if !ok {
			filtered = append(filtered, tool)
			continue
		}
		if authorizeMCPToolRequest(headers, entry.Options) {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

func authorizeMCPToolRequest(headers http.Header, options *HandlerOptions) bool {
	mcpToolAuthorizerMutex.RLock()
	authorizer := mcpToolAuthorizer
	mcpToolAuthorizerMutex.RUnlock()
	if authorizer == nil {
		return true
	}
	if headers == nil {
		headers = http.Header{}
	}
	return authorizer(headers, options)
}

var (
	routeParamPattern  = regexp.MustCompile(`:([A-Za-z0-9_]+)`)
	mcpToolNamePattern = regexp.MustCompile(`[^A-Za-z0-9_]+`)
)
