package api

import (
	"sort"

	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/util"
)

type ProtectedAPIRoute struct {
	Method Method
	Path   string
}

var DefaultProtectedAPIRoutes = []ProtectedAPIRoute{
	{Method: GET, Path: "/stats"},
	{Method: GET, Path: "/queue/stats"},
	{Method: GET, Path: "/queue/:id/stats"},
	{Method: GET, Path: "/queue/:id/_scroll"},
	{Method: DELETE, Path: "/queue/:id"},
	{Method: DELETE, Path: "/queue/_search"},
	{Method: PUT, Path: "/queue/:id/consumer/:consumer_id/offset"},
	{Method: GET, Path: "/queue/:id/consumer/:consumer_id/offset"},
	{Method: DELETE, Path: "/queue/:id/consumer/:consumer_id"},
	{Method: DELETE, Path: "/queue/consumer/_search"},
	{Method: GET, Path: "/pipeline/tasks/"},
	{Method: POST, Path: "/pipeline/tasks/_search"},
	{Method: POST, Path: "/pipeline/task/:id/_start"},
	{Method: POST, Path: "/pipeline/task/:id/_stop"},
	{Method: GET, Path: "/pipeline/task/:id"},
	{Method: DELETE, Path: "/pipeline/task/:id"},
	{Method: GET, Path: "/config/"},
	{Method: PUT, Path: "/config/"},
	{Method: GET, Path: "/config/runtime"},
	{Method: GET, Path: "/setting/logger"},
	{Method: PUT, Path: "/setting/logger"},
	{Method: POST, Path: "/setting/logger"},
}

func RegisterProtectedUIRoutes(routes []ProtectedAPIRoute, handle httprouter.Handle, options ...Option) {
	for _, route := range routes {
		HandleUIMethod(route.Method, route.Path, handle, options...)
	}
}

func RegisterProtectedRouterRoutes(router *httprouter.Router, routes []ProtectedAPIRoute, handle httprouter.Handle) {
	if router == nil {
		return
	}
	for _, route := range routes {
		router.Handle(string(route.Method), route.Path, handle)
	}
}

type MissingAPIMethodUIRoute struct {
	Route   ProtectedAPIRoute
	Options *HandlerOptions
}

func WalkMissingAPIMethodUIRoutes(walk func(route MissingAPIMethodUIRoute)) {
	if walk == nil {
		return
	}

	l.Lock()
	routes := make([]MissingAPIMethodUIRoute, 0)
	for method, handlers := range registeredAPIMethodHandler {
		for path := range handlers {
			if shouldSkipEmbeddedAPIRoute(method, path) {
				continue
			}
			var options *HandlerOptions
			if registeredOptions, ok := apiOptions.Get(Method(method), path); ok {
				options = cloneHandlerOptions(registeredOptions)
			}
			routes = append(routes, MissingAPIMethodUIRoute{
				Route: ProtectedAPIRoute{
					Method: Method(method),
					Path:   path,
				},
				Options: options,
			})
		}
	}
	l.Unlock()

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

// RegisterMissingAPIMethodUIRoutes mirrors registered API method routes onto the
// web router only when no UI route already owns the same method/path.
func RegisterMissingAPIMethodUIRoutes(handle httprouter.Handle, options ...Option) {
	if handle == nil {
		return
	}

	WalkMissingAPIMethodUIRoutes(func(route MissingAPIMethodUIRoute) {
		HandleUIMethod(route.Route.Method, route.Route.Path, handle, options...)
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
