package api

import (
	"sort"

	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/util"
)

type RegisteredUIMethodRoute struct {
	Route   MCPAutoRoute
	Options *HandlerOptions
	Handler httprouter.Handle
}

type MCPAutoRoute struct {
	Method Method
	Path   string
}

func WalkRegisteredUIMethodRoutes(walk func(route RegisteredUIMethodRoute)) {
	if walk == nil {
		return
	}

	uiMutex.Lock()
	routes := make([]RegisteredUIMethodRoute, 0)
	for method, handlers := range registeredUIMethodHandler {
		for path, handler := range handlers {
			routes = append(routes, RegisteredUIMethodRoute{
				Route:   MCPAutoRoute{Method: method, Path: path},
				Options: cloneHandlerOptions(handler.Options),
				Handler: handler.Handler,
			})
		}
	}
	uiMutex.Unlock()

	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Route.Method == routes[j].Route.Method {
			return routes[i].Route.Path < routes[j].Route.Path
		}
		return routes[i].Route.Method < routes[j].Route.Method
	})

	for _, route := range routes {
		walk(route)
	}
}

func WalkMCPAutoUIMethodRoutes(walk func(route RegisteredUIMethodRoute)) {
	if walk == nil {
		return
	}

	WalkRegisteredUIMethodRoutes(func(route RegisteredUIMethodRoute) {
		if route.Options == nil || !route.Options.Feature(FeatureMCPAuto) {
			return
		}
		walk(route)
	})
}

func cloneHandlerOptions(options *HandlerOptions) *HandlerOptions {
	if options == nil {
		return nil
	}

	cloned := *options
	if options.RequirePermission != nil {
		cloned.RequirePermission = append([]PermissionKey(nil), options.RequirePermission...)
	}
	if options.Tags != nil {
		cloned.Tags = append([]string(nil), options.Tags...)
	}
	if options.Features != nil {
		cloned.Features = map[string]bool{}
		for key, value := range options.Features {
			cloned.Features[key] = value
		}
	}
	if options.Labels != nil {
		cloned.Labels = util.MapStr{}
		for key, value := range options.Labels {
			cloned.Labels[key] = value
		}
	}

	return &cloned
}
