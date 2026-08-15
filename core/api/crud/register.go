/* Copyright © INFINI LTD. All rights reserved. */

package crud

import (
	"fmt"

	"infini.sh/framework/core/api"
)

// RegisterCRUD builds the five handlers and wires the routes:
//
//	POST   {prefix}/            create
//	GET    {prefix}/_search     search   (+ POST, without the MCP tool)
//	GET    {prefix}/:id         get      (param name: cfg.IDParam, default "id")
//	PUT    {prefix}/:id         update
//	DELETE {prefix}/:id         delete
//
// Each route is gated by cfg.Permission(action) when the func is set, and
// cfg.ExtraOptions(action) appends additional options (login, CORS,
// sensitive-field masking...) after the permission and MCP options. The
// MCP tool is registered on the GET _search variant only so POST _search
// does not duplicate the tool name (coco convention).
func RegisterCRUD[T any, P PT[T]](cfg Config[T]) {
	validateConfig(cfg)

	h := NewHandlers[T, P](cfg)
	idParam := cfg.IDParam
	if idParam == "" {
		idParam = "id"
	}
	perm := func(action string, withMCP bool) []api.Option {
		return buildOptions(cfg, action, withMCP)
	}

	skip := map[string]bool{}
	for _, a := range cfg.SkipActions {
		skip[a] = true
	}
	register := func(method api.Method, path string, handler HandlerFunc, action string, withMCP bool) {
		if skip[action] {
			return
		}
		api.HandleUIMethod(method, path, handler, perm(action, withMCP)...)
	}

	register(api.POST, cfg.Prefix+"/", h.Create, ActionCreate, true)
	register(api.GET, cfg.Prefix+"/_search", h.Search, ActionSearch, true)
	register(api.POST, cfg.Prefix+"/_search", h.Search, ActionSearch, false)
	register(api.GET, cfg.Prefix+"/:"+idParam, h.Get, ActionRead, true)
	register(api.PUT, cfg.Prefix+"/:"+idParam, h.Update, ActionUpdate, true)
	register(api.DELETE, cfg.Prefix+"/:"+idParam, h.Delete, ActionDelete, true)
}

func validateConfig[T any](cfg Config[T]) {
	if cfg.Prefix == "" || cfg.Prefix[0] != '/' {
		panic(fmt.Sprintf("crud: Prefix must start with '/', got %q", cfg.Prefix))
	}
	if len(cfg.Prefix) > 1 && cfg.Prefix[len(cfg.Prefix)-1] == '/' {
		panic(fmt.Sprintf("crud: Prefix must not end with '/', got %q", cfg.Prefix))
	}
	if cfg.Resource == "" {
		panic("crud: Resource is required")
	}
}

// buildOptions assembles the route options for one action: the permission
// gate first, then the MCP tool (only on the registration that carries it,
// i.e. GET _search), then the caller's extra options. Exposed for testing.
func buildOptions[T any](cfg Config[T], action string, withMCP bool) []api.Option {
	var opts []api.Option
	if cfg.Permission != nil {
		if key := cfg.Permission(action); key != "" {
			opts = append(opts, api.RequirePermission(key))
		}
	}
	if cfg.MCP && withMCP {
		opts = append(opts, api.MCPTool(cfg.mcpToolName(action), cfg.mcpToolDesc(action)))
	}
	if cfg.ExtraOptions != nil {
		opts = append(opts, cfg.ExtraOptions(action)...)
	}
	if len(opts) == 0 {
		return nil
	}
	return opts
}
