package api

import httprouter "infini.sh/framework/core/api/router"

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
