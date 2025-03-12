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

package api

import "infini.sh/framework/core/util"

// Define the Option type (function that modifies HandlerOptions)
type Option func(*HandlerOptions)

// Define HandlerOptions to hold the state of all options
type HandlerOptions struct {
	RequireLogin bool
	OptionLogin  bool
	Permission   string
	LogRequest   bool
	Labels       util.MapStr
	Tags         []string
	// Add other options as needed
}

// Registry to store options for each method or route
type OptionRegistry struct {
	options map[string]*HandlerOptions
}

func (o OptionRegistry) GetKey(method Method, pattern string) string {
	return string(method) + pattern
}

func (o OptionRegistry) Register(method Method, pattern string, options *HandlerOptions) {
	o.options[o.GetKey(method, pattern)] = options
}

func (o OptionRegistry) Get(method Method, pattern string, ) (options *HandlerOptions, ok bool) {
	options, ok = o.options[o.GetKey(method, pattern)]
	return options, ok
}

// NewOptionRegistry creates a new registry
func NewOptionRegistry() *OptionRegistry {
	return &OptionRegistry{
		options: make(map[string]*HandlerOptions),
	}
}

// Option to set RequireLogin
func RequireLogin() Option {
	return func(o *HandlerOptions) {
		o.RequireLogin = true
	}
}

func OptionLogin() Option {
	return func(o *HandlerOptions) {
		o.OptionLogin = true
	}
}

func AllowPublicAccess() Option {
	return func(o *HandlerOptions) {
		o.RequireLogin = false
	}
}

// Option to set Permission
func Permission(permission string) Option {
	return func(o *HandlerOptions) {
		o.Permission = permission
	}
}
