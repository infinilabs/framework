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

package yaml

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"infini.sh/framework/lib/go-ucfg"
	"infini.sh/framework/lib/go-ucfg/cfgtest"
)

func TestPrimitives(t *testing.T) {
	input := `
    b: true
    i: 42
    u: 23
    f: 3.14
    s: string
  `
	c := mustNewConfig(t, input)
	verify := struct {
		B bool
		I int
		U uint
		F float64
		S string
	}{}
	mustUnpack(t, c, &verify)

	assert.True(t, verify.B)
	assert.Equal(t, 42, verify.I)
	assert.Equal(t, uint(23), verify.U)
	assert.Equal(t, 3.14, verify.F)
	assert.Equal(t, "string", verify.S)
}

func TestNested(t *testing.T) {
	input := `
    c:
      b: true
  `
	c := mustNewConfig(t, input)
	var verify struct {
		C struct{ B bool }
	}
	mustUnpack(t, c, &verify)
	assert.True(t, verify.C.B)
}

func TestNestedPath(t *testing.T) {
	input := `
    c.b: true
  `
	c := mustNewConfig(t, input, ucfg.PathSep("."))
	var verify struct {
		C struct{ B bool }
	}
	mustUnpack(t, c, &verify)
	assert.True(t, verify.C.B)
}

func TestArray(t *testing.T) {
	input := `
- b: 2
  c: 3
- c: 4
`
	c := mustNewConfig(t, input)
	var verify []map[string]int
	mustUnpack(t, c, &verify)

	require.Len(t, verify, 2)
	assert.Equal(t, verify[0]["b"], 2)
	assert.Equal(t, verify[0]["c"], 3)
	assert.Equal(t, verify[1]["c"], 4)
}

func TestEmptyCollections(t *testing.T) {
	tests := map[string]struct {
		input string
		to    interface{}
		want  interface{}
	}{
		"struct with empty array into struct with array of interfaces": {
			input: "a: []",
			to: &struct {
				A []interface{}
			}{},
			want: &struct {
				A []interface{}
			}{
				A: []interface{}{},
			},
		},
		"empty array into array of interfaces": {
			input: `[]`,
			to:    &[]interface{}{},
			want:  &[]interface{}{},
		},
		"empty array into array of string": {
			input: `a: []`,
			to: &struct {
				A []string
			}{},
			want: &struct{ A []string }{A: []string{}},
		},
		"struct with empty map into struct with interface": {
			input: "a: {}",
			to: &struct {
				A interface{}
			}{},
			want: &struct {
				A interface{}
			}{},
		},
		"struct with empty map into struct with map of interfaces": {
			input: "a: {}",
			to: &struct {
				A map[string]interface{}
			}{},
			want: &struct {
				A map[string]interface{}
			}{
				A: map[string]interface{}{},
			},
		},
		"empty map into map of interfaces": {
			input: "{}",
			to:    &map[string]interface{}{},
			want:  &map[string]interface{}{},
		},
		"empty map in array into array of maps of interfaces": {
			input: "[{}]",
			to:    &[]map[string]interface{}{},
			want: &[]map[string]interface{}{
				{},
			},
		},
		"struct with empty array into map of interfaces": {
			input: `{"a": []}`,
			to:    &map[string]interface{}{},
			want: &map[string]interface{}{
				"a": []interface{}{},
			},
		},
		"struct with empty array into map of interfaces with existing fields": {
			input: `{"a": []}`,
			to: &map[string]interface{}{
				"a": nil,
				"b": 3,
			},
			want: &map[string]interface{}{
				"a": []interface{}{},
				"b": 3,
			},
		},
		"struct with empty array into map of arrays of interfaces": {
			input: `{"a": []}`,
			to:    &map[string][]interface{}{},
			want: &map[string][]interface{}{
				"a": {},
			},
		},
		"struct with empty array into map of arrays of interfaces with existing fields": {
			input: `{"a": []}`,
			to: &map[string][]interface{}{
				"a": nil,
				"b": {3},
			},
			want: &map[string][]interface{}{
				"a": {},
				"b": {3},
			},
		},
		"empty array in array into array of interfaces": {
			input: `[[]]`,
			to:    &[]interface{}{},
			want: &[]interface{}{
				[]interface{}{},
			},
		},
		"empty array in array into array of array of interfaces": {
			input: `[[]]`,
			to:    &[][]interface{}{},
			want: &[][]interface{}{
				{},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			c := mustNewConfig(t, test.input)
			cfgtest.MustUnpack(t, c, test.to)
			assert.Equal(t, test.want, test.to)
		})
	}
}

func mustNewConfig(t *testing.T, input string, opts ...ucfg.Option) *ucfg.Config {
	c, err := NewConfig([]byte(input), opts...)
	require.NoError(t, err, "failed to parse input")
	return c
}

func mustUnpack(t *testing.T, c *ucfg.Config, v interface{}) {
	err := c.Unpack(v)
	require.NoError(t, err, "failed to unpack config")
}

// Defines struct to read config from
type Config struct {
	RoleMapping map[string][]string `yaml:"role_mapping"`
	Count       int32               `yaml:"count" validate:"min=0, max=9"`
}

var config = &Config{
	RoleMapping: make(map[string][]string),
}

func TestPatchLoadDefaultCfg(t *testing.T) {
	path := "patch_test.yml"
	in, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	if err := decode(in, &config); err != nil {
		t.Fatalf("Failed to load config file: %v", err)
	}

	// 验证反序列化结果
	assert.Equal(t, int32(3), config.Count)
	assert.Len(t, config.RoleMapping, 1) // 验证映射有 1 个键值对
	assert.Contains(t, config.RoleMapping, "\"liugq.infinilabs.com\"")
	assert.Equal(t, []string{"ReadonlyUI"}, config.RoleMapping["\"liugq.infinilabs.com\""])
}

func TestPatchDecoderDefaultCfg(t *testing.T) {
	path := "patch_test.yml"
	in, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}
	// 使用 PatchedDecoder 解码
	decoder := NewCustomDecoder(bytes.NewBuffer(in))
	err = decoder.Decode(&config)
	if err != nil {
		t.Fatalf("Error decoding YAML: %v", err)
	}

	// 验证反序列化结果
	assert.Equal(t, int32(3), config.Count)
	assert.Len(t, config.RoleMapping, 1) // 验证映射有 1 个键值对
	assert.Contains(t, config.RoleMapping, "\"liugq.infinilabs.com\"")
	assert.Equal(t, []string{"ReadonlyUI"}, config.RoleMapping["\"liugq.infinilabs.com\""])
}
