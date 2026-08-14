/* Copyright © INFINI LTD. All rights reserved. */

package crud

import (
	"fmt"

	"infini.sh/framework/core/api"
)

// RegisterCRUD builds the five handlers and wires the routes:
//
//	POST   {prefix}/            create
//	GET    {prefix}/_search     search   (+ POST)
//	GET    {prefix}/:id         get
//	PUT    {prefix}/:id         update
//	DELETE {prefix}/:id         delete
//
// Each route is gated by cfg.Permission(action) when the func is set.
func RegisterCRUD[T any, P PT[T]](cfg Config[T]) {
	validateConfig(cfg)

	h := NewHandlers[T, P](cfg)
	perm := func(action string) []api.Option {
		var opts []api.Option
		if cfg.Permission != nil {
			if key := cfg.Permission(action); key != "" {
				opts = append(opts, api.RequirePermission(key))
			}
		}
		if cfg.MCP {
			opts = append(opts, api.MCPTool(cfg.mcpToolName(action), cfg.mcpToolDesc(action)))
		}
		if len(opts) == 0 {
			return nil
		}
		return opts
	}

	api.HandleUIMethod(api.POST, cfg.Prefix+"/", h.Create, perm(ActionCreate)...)
	api.HandleUIMethod(api.GET, cfg.Prefix+"/_search", h.Search, perm(ActionSearch)...)
	api.HandleUIMethod(api.POST, cfg.Prefix+"/_search", h.Search, perm(ActionSearch)...)
	api.HandleUIMethod(api.GET, cfg.Prefix+"/:id", h.Get, perm(ActionRead)...)
	api.HandleUIMethod(api.PUT, cfg.Prefix+"/:id", h.Update, perm(ActionUpdate)...)
	api.HandleUIMethod(api.DELETE, cfg.Prefix+"/:id", h.Delete, perm(ActionDelete)...)
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
