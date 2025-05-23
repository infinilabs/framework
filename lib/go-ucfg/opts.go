// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package ucfg

import (
	"fmt"
	"os"
	"strings"

	"infini.sh/framework/lib/go-ucfg/parse"
)

// Option type implementing additional options to be passed
// to go-ucfg library functions.
type Option func(*options)

type options struct {
	tag          string
	validatorTag string
	pathSep      string
	meta         *Meta
	env          []*Config
	resolvers    []func(name string) (string, parse.Config, error)
	varexp       bool
	noParse      bool

	configValueHandling configHandling
	fieldHandlingTree   *fieldHandlingTree

	// temporary cache of parsed splice values for lifetime of call to
	// Unpack/Pack/Get/...
	parsed valueCache

	activeFields       *fieldSet
	resolveRef         bool
	defaultParseConfig *parse.Config
	noResolve          bool
}

// NoResolve option sets do not to resolve variables.
var NoResolve Option = doNoResolve

func doNoResolve(o *options) { o.noResolve = true }

// DefaultParseConfig option sets the default parse config used to parse dyn value
// if it is not empty
func DefaultParseConfig(config parse.Config) Option {
	return func(o *options) {
		o.defaultParseConfig = &config
	}
}

type valueCache map[string]spliceValue

// specific API on top of Config to handle adjusting merging behavior per fields
type fieldHandlingTree Config

// id used to store intermediate parse results in current execution context.
// As parsing results might differ between multiple calls due to:
// splice being shared between multiple configurations, or environment
// changing between calls + lazy nature of cfgSplice, parsing results cannot
// be stored in cfgSplice itself.
type cacheID string

type spliceValue struct {
	err   error
	value value
}

// StructTag option sets the struct tag name to use for looking up
// field names and options in `Unpack` and `Merge`.
// The default struct tag in `config`.
func StructTag(tag string) Option {
	return func(o *options) {
		o.tag = tag
	}
}

// ValidatorTag option sets the struct tag name used to set validators
// on struct fields in `Unpack`.
// The default struct tag in `validate`.
func ValidatorTag(tag string) Option {
	return func(o *options) {
		o.validatorTag = tag
	}
}

// PathSep sets the path separator used to split up names into a tree like hierarchy.
// If PathSep is not set, field names will not be split.
func PathSep(sep string) Option {
	return func(o *options) {
		o.pathSep = sep
	}
}

// MetaData option passes additional metadata (currently only source of the
// configuration) to be stored internally (e.g. for error reporting).
func MetaData(meta Meta) Option {
	return func(o *options) {
		o.meta = &meta
	}
}

// Env option adds another configuration for variable expansion to be used, if
// the path to look up does not exist in the actual configuration. Env can be used
// multiple times in order to add more lookup environments.
func Env(e *Config) Option {
	return func(o *options) {
		o.env = append(o.env, e)
	}
}

// Resolve option adds a callback used by variable name expansion. The callback
// will be called if a variable can not be resolved from within the actual configuration
// or any of its environments.
func Resolve(fn func(name string) (string, parse.Config, error)) Option {
	return func(o *options) {
		o.resolvers = append(o.resolvers, fn)
	}
}

// ResolveEnv option adds a look up callback looking up values in the available
// OS environment variables.
var ResolveEnv Option = doResolveEnv

func doResolveEnv(o *options) {
	o.resolvers = append(o.resolvers, func(name string) (string, parse.Config, error) {
		value := os.Getenv(name)
		if value == "" {
			return "", parse.EnvConfig, ErrMissing
		}
		return value, parse.EnvConfig, nil
	})
}

// ResolveNOOP option add a resolver that will not search the value but instead will return the
// provided key wrap with the field reference syntax. This is useful if you don't to expose values
// from envionment variable or other resolvers.
//
// Example: "mysecret" => ${mysecret}"
var ResolveNOOP Option = doResolveNOOP

func doResolveNOOP(o *options) {
	o.resolvers = append(o.resolvers, func(name string) (string, parse.Config, error) {
		return "$[[" + name + "]]", parse.NoopConfig, nil
	})
}

var (
	// ReplaceValues option configures all merging and unpacking operations to
	// replace old dictionaries and arrays while merging. Value merging can be
	// overwritten in unpack by using struct tags.
	ReplaceValues = makeOptValueHandling(cfgReplaceValue)

	// AppendValues option configures all merging and unpacking operations to
	// merge dictionaries and append arrays to existing arrays while merging.
	// Value merging can be overwritten in unpack by using struct tags.
	AppendValues = makeOptValueHandling(cfgArrAppend)

	// PrependValues option configures all merging and unpacking operations to
	// merge dictionaries and prepend arrays to existing arrays while merging.
	// Value merging can be overwritten in unpack by using struct tags.
	PrependValues = makeOptValueHandling(cfgArrPrepend)
)

func makeOptValueHandling(h configHandling) Option {
	return func(o *options) {
		o.configValueHandling = h
	}
}

var (
	// FieldMergeValues option configures all merging and unpacking operations to use
	// the default merging behavior for the specified field. This overrides the any struct
	// tags during unpack for the field. Nested field names can be defined using dot
	// notation.
	FieldMergeValues = makeFieldOptValueHandling(cfgMergeValues)

	// FieldReplaceValues option configures all merging and unpacking operations to
	// replace old dictionaries and arrays while merging for the specified field. This
	// overrides the any struct tags during unpack for the field. Nested field names
	// can be defined using dot notation.
	FieldReplaceValues = makeFieldOptValueHandling(cfgReplaceValue)

	// FieldAppendValues option configures all merging and unpacking operations to
	// merge dictionaries and append arrays to existing arrays while merging for the
	// specified field. This overrides the any struct tags during unpack for the field.
	// Nested field names can be defined using dot notation.
	FieldAppendValues = makeFieldOptValueHandling(cfgArrAppend)

	// FieldPrependValues option configures all merging and unpacking operations to
	// merge dictionaries and prepend arrays to existing arrays while merging for the
	// specified field. This overrides the any struct tags during unpack for the field.
	// Nested field names can be defined using dot notation.
	FieldPrependValues = makeFieldOptValueHandling(cfgArrPrepend)
)

func makeFieldOptValueHandling(h configHandling) func(...string) Option {
	return func(fieldName ...string) Option {
		if len(fieldName) == 0 {
			return func(_ *options) {}
		}

		table := make(map[string]configHandling)
		for _, name := range fieldName {
			// field value config options are rendered into a Config; the '*' represents the handling method
			// for everything nested under this field.
			if !strings.HasSuffix(name, ".*") {
				name = fmt.Sprintf("%s.*", name)
			}
			table[name] = h
		}

		return func(o *options) {
			if o.fieldHandlingTree == nil {
				o.fieldHandlingTree = newFieldHandlingTree()
			}
			o.fieldHandlingTree.merge(table, PathSep(o.pathSep))
		}
	}
}

// VarExp option enables support for variable expansion. Resolve and Env options will only be effective if  VarExp is set.
var VarExp Option = doVarExp

func doVarExp(o *options) { o.varexp = true }

// ResolveRef option enables support for variable resolve reference from parent config.
var ResolveRef Option = doResolveRef

func doResolveRef(o *options) { o.resolveRef = true }

func makeOptions(opts []Option) *options {
	o := options{
		tag:          "config",
		validatorTag: "validate",
		pathSep:      "", // no separator by default
		parsed:       map[string]spliceValue{},
		activeFields: newFieldSet(nil),
	}
	for _, opt := range opts {
		opt(&o)
	}
	return &o
}

func (cache valueCache) cachedValue(
	id cacheID,
	f func() (value, error),
) (value, error) {
	if v, ok := cache[string(id)]; ok {
		if v.err != nil {
			return nil, v.err
		}
		return v.value, nil
	}

	v, err := f()

	// Only primitives can be cached, allowing us to get out of infinite loop
	if v != nil && v.canCache() {
		cache[string(id)] = spliceValue{err, v}
	}
	return v, err
}

func newFieldHandlingTree() *fieldHandlingTree {
	return (*fieldHandlingTree)(New())
}

func (t *fieldHandlingTree) merge(other interface{}, opts ...Option) error {
	cfg := (*Config)(t)
	return cfg.Merge(other, opts...)
}

func (t *fieldHandlingTree) child(fieldName string, idx int) (*fieldHandlingTree, error) {
	cfg := (*Config)(t)
	child, err := cfg.Child(fieldName, idx)
	if err != nil {
		return nil, err
	}
	return (*fieldHandlingTree)(child), nil
}

func (t *fieldHandlingTree) configHandling(fieldName string, idx int) (configHandling, error) {
	cfg := (*Config)(t)
	handling, err := cfg.Uint(fieldName, idx)
	if err != nil {
		return cfgDefaultHandling, err
	}
	return configHandling(handling), nil
}

func (t *fieldHandlingTree) wildcard() (*fieldHandlingTree, error) {
	return t.child("**", -1)
}

func (t *fieldHandlingTree) setWildcard(wildcard *fieldHandlingTree) error {
	cfg := (*Config)(t)
	return cfg.SetChild("**", -1, (*Config)(wildcard))
}

func (t *fieldHandlingTree) fieldHandling(fieldName string, idx int) (configHandling, *fieldHandlingTree, bool) {
	child, err := t.child(fieldName, idx)
	if err == nil {
		cfgHandling, err := child.configHandling("*", -1)
		if err == nil {
			return cfgHandling, child, true
		}
	}
	// try wildcard match
	wildcard, err := t.wildcard()
	if err != nil {
		return cfgDefaultHandling, child, false
	}
	cfgHandling, cfg, ok := wildcard.fieldHandling(fieldName, idx)
	if ok {
		return cfgHandling, cfg, ok
	}
	return cfgDefaultHandling, child, ok
}
