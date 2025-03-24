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

import (
	"infini.sh/framework/core/util"
)

// Define the Option type (function that modifies HandlerOptions)
type Option func(*HandlerOptions)

// Define HandlerOptions to hold the state of all options
type HandlerOptions struct {
	RequireLogin      bool
	RequirePermission []string
	OptionLogin       bool
	Resource          string
	Action            string
	Name              string
	LogRequest        bool
	Labels            util.MapStr
	Features          map[string]bool
	Tags              []string
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

func (o OptionRegistry) Get(method Method, pattern string) (options *HandlerOptions, ok bool) {
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

func RequirePermission(permissions ...string) Option {
	return func(o *HandlerOptions) {
		o.RequireLogin = true
		o.OptionLogin = false
		if o.RequirePermission==nil{
			o.RequirePermission=[]string{}
		}

		for _,v:=range permissions {
			o.RequirePermission = append(o.RequirePermission, v)
		}
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

func Resource(source string) Option {
	return func(o *HandlerOptions) {
		o.Resource = source
	}
}

func Action(action string) Option {
	return func(o *HandlerOptions) {
		o.Action = action
	}
}

func Name(name string) Option {
	return func(o *HandlerOptions) {
		o.Name = name
	}
}

func Label(label, v string) Option {
	return func(o *HandlerOptions) {
		if o.Labels == nil {
			o.Labels = util.MapStr{}
		}
		o.Labels[label] = v
	}
}

func Feature(feature string) Option {
	return func(o *HandlerOptions) {
		if o.Features == nil {
			o.Features = map[string]bool{}
		}
		o.Features[feature] = true
	}
}
