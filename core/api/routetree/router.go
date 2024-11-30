// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

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