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

package orm

import (
	"context"
	"infini.sh/framework/core/param"
	"infini.sh/framework/core/util"
	"reflect"
)

type Context struct {
	context.Context
	Refresh string
	param.Parameters
}

func (ctx *Context) SetValue(key param.ParaKey, value interface{}) {
	ctx.Set(key, value)
}

func NewContext() *Context {
	c := Context{}
	c.Context = context.Background()
	c.Parameters = param.Parameters{}
	return &c
}

func (c *Context) DirectAccess() *Context {
	c.Set(DirectReadWithoutPermissionCheck, true)
	c.Set(DirectWriteWithoutPermissionCheck, true)
	return c
}

func (c *Context) DirectReadAccess() *Context {
	c.Set(DirectReadWithoutPermissionCheck, true)
	return c
}

func (c *Context) DirectWriteAccess() *Context {
	c.Set(DirectWriteWithoutPermissionCheck, true)
	return c
}

// TODO
func (c *Context) RunAs(tenantID, userID string) *Context {
	c.Set(DirectReadWithoutPermissionCheck, true)
	return c
}

func NewContextWithParent(parent context.Context) *Context {
	return &Context{
		Context:    parent,
		Parameters: param.Parameters{},
	}
}

func NewModelContext(m interface{}) *Context {
	c := NewContext()
	WithModel(c, m)
	return c
}

const (
	ctxKeyModel        param.ParaKey = "model"
	ctxWildcardIndex   param.ParaKey = "wildcard_index"
	ctxKeyIndices      param.ParaKey = "indices"       // []string
	ctxKeyIndexPattern param.ParaKey = "index_pattern" // string, mutually exclusive with indices
	ctxKeyRoutingKey   param.ParaKey = "routing_key"   // string

	ctxCollapseFieldKey  param.ParaKey = "collapse_field"
	ctxQueryArgsKey      param.ParaKey = "query_args"
	ctxTemplatedQueryKey param.ParaKey = "templated_query"
)

func SetWildcardIndex(ctx *Context, wildcard bool) *Context {
	ctx.Set(ctxWildcardIndex, wildcard)
	return ctx
}

func IsWildcardIndex(ctx *Context) bool {
	if isNil(ctx) {
		return false
	}

	if val, ok := ctx.Get(ctxWildcardIndex).(bool); ok {
		return val
	}
	return false
}

// Set multiple explicit indices
func WithIndices(ctx *Context, indices ...string) *Context {
	ctx.Set(ctxKeyIndices, indices)
	return ctx
}

// Set an index pattern (like "logs-*")
func WithIndexPattern(ctx *Context, pattern string) *Context {
	ctx.Set(ctxKeyIndexPattern, pattern)
	return ctx
}

// Set a routing key (to target specific shard / tenant partition)
func WithRoutingKey(ctx *Context, routing string) *Context {
	ctx.Set(ctxKeyRoutingKey, routing)
	return ctx
}

func GetIndices(ctx *Context) []string {
	if isNil(ctx) {
		return []string{}
	}
	val, _ := ctx.Get(ctxKeyIndices).([]string)
	return val
}

func GetIndexPattern(ctx *Context) string {
	if isNil(ctx) {
		return ""
	}
	val, _ := ctx.Get(ctxKeyIndexPattern).(string)
	return val
}

func GetRoutingKey(ctx *Context) string {
	if isNil(ctx) {
		return ""
	}
	val, _ := ctx.Get(ctxKeyRoutingKey).(string)
	return val
}

// WithModel sets the model type (typically a pointer to a struct) in the context.
func WithModel(ctx *Context, model interface{}) *Context {
	ctx.SetValue(ctxKeyModel, model)
	return ctx
}

// GetModel retrieves the model stored in context, returns nil if not set.
func GetModel(ctx *Context) interface{} {
	return ctx.Get(ctxKeyModel)
}

// WithCollapseField stores the collapse field in the context.
func WithCollapseField(ctx *Context, field string) *Context {
	ctx.Set(ctxCollapseFieldKey, field)
	return ctx
}

// GetCollapseField retrieves the collapse field from the context.
func GetCollapseField(ctx *Context) string {

	if isNil(ctx) {
		return ""
	}
	if x := ctx.Get(ctxCollapseFieldKey); x != nil {
		if v, ok := x.(string); ok {
			return v
		}
	}

	return ""
}

func WithQueryArgs(ctx *Context, args *[]util.KV) *Context {
	ctx.Set(ctxQueryArgsKey, args)
	return ctx
}

func WithTemplatedQuery(ctx *Context, templateID string, param map[string]interface{}) *Context {
	templatedQuery := &TemplatedQuery{
		TemplateID: templateID,
		Parameters: param,
	}
	ctx.Set(ctxTemplatedQueryKey, templatedQuery)
	return ctx
}

func GetTemplatedQuery(ctx *Context) *TemplatedQuery {
	if isNil(ctx) {
		return nil
	}
	if val, ok := ctx.Get(ctxTemplatedQueryKey).(*TemplatedQuery); ok {
		return val
	}
	return nil
}

func GetQueryArgs(ctx *Context) *[]util.KV {
	if isNil(ctx) {
		return nil
	}

	if v, ok := ctx.Get(ctxQueryArgsKey).(*[]util.KV); ok {
		return v
	}
	return nil
}

func (ctx *Context) Get(key param.ParaKey) interface{} {
	if ctx.Context != nil {
		if val := ctx.Context.Value(key); val != nil {
			return val
		}
	}
	return ctx.Parameters.Get(key)
}

func (ctx *Context) GetBool(key param.ParaKey, defaultV bool) bool {
	v := ctx.Get(key)
	if v != nil {
		s, ok := v.(bool)
		if ok {
			return s
		}
	}
	return defaultV
}

func (ctx *Context) GetString(key param.ParaKey) (string, bool) {
	v := ctx.Get(key)
	if v == nil {
		return "", false
	}

	s, ok := v.(string)
	if ok {
		return s, ok
	}
	return s, ok
}

func isNil(i interface{}) bool {
	if i == nil {
		return true
	}
	v := reflect.ValueOf(i)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return v.IsNil()
	}
	return false
}
