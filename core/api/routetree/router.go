/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package routetree

type Router struct {
	root *node
	redirectTrailingSlash bool
}

func New(options ...RouterOption) *Router{
	r := &Router{
		root:  &node{path: "/"},
		redirectTrailingSlash: true,
	}
	for _, option := range options {
		option(r)
	}
	return r
}

func (r *Router) Handle(method string, path string, handlerFunc HandlerFunc) {
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}
	n := r.root.addPath(path, nil, false)
	n.setPermission(method, handlerFunc, false)
}

func (r *Router) Search(method string, path string) (handler HandlerFunc, params map[string]string, matched bool){
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}
	pathLen := len(path)
	trailingSlash := path[pathLen-1] == '/' && pathLen > 1
	if trailingSlash && r.redirectTrailingSlash {
		path = path[:pathLen-1]
	}
	var (
		matchNode *node
		paramValues []string
	)
	matchNode, handler, paramValues = r.root.search(method, path)
	params = map[string]string{}
	if matchNode != nil {
		length := len(paramValues)
		if length > 0 && length == len(matchNode.leafWildcardNames){
			for i, paramName := range matchNode.leafWildcardNames {
				params[paramName] = paramValues[length-i-1]
			}
		}
		matched = true
		return
	}
	matched = false
	return
}


type RouterOption func(r *Router)

func RedirectTrailingSlashOption( trailingSlash bool) RouterOption{
	return func(r *Router) {
		r.redirectTrailingSlash = trailingSlash
	}
}