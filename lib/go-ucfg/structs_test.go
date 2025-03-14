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
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"infini.sh/framework/lib/go-ucfg/parse"
)

type testEnv map[string]string

func TestStructMergeUnpackTyped(t *testing.T) {
	type test struct {
		t   interface{}
		cfg interface{}
		env testEnv
	}

	tests := []test{
		{
			t: &struct {
				Strings []string
			}{
				Strings: []string{"string1", "abc"},
			},
			cfg: map[string]interface{}{
				"strings": "$[[env_strings]]",
			},
			env: testEnv{"env_strings": "string1,abc"},
		},
		{
			t: &struct {
				Strings string
			}{
				Strings: "string1,abc",
			},
			cfg: map[string]interface{}{
				"strings": "$[[env_strings:string1,abc]]",
			},
		},
		{
			t: &struct {
				Strings []string
			}{
				Strings: []string{"one string"},
			},
			cfg: map[string]interface{}{
				"strings": "$[[env_strings]] string",
			},
			env: testEnv{"env_strings": "one"},
		},
		{
			t: &struct {
				Hosts []string
			}{
				Hosts: []string{"host1:1234", "host2:4567"},
			},
			cfg: map[string]interface{}{
				"hosts": "$[[hosts_from_env]]",
			},
			env: testEnv{"hosts_from_env": "host1:1234,host2:4567"},
		},
		{
			t: &struct {
				Hosts string
			}{
				Hosts: "host1:1234,host2:4567",
			},
			cfg: map[string]interface{}{
				"hosts": "$[[missing_env:host1:1234,host2:4567]]",
			},
		},
	}

	for i, test := range tests {
		t.Logf("run test (%v): %v, %v", i, test.t, test.cfg)

		opts := []Option{
			VarExp,
			resolveTestEnv(test.env),
		}

		// unpack input
		c, err := NewFrom(test.t, opts...)
		if err != nil {
			t.Fatal(err)
		}

		// compute expected outcome
		var expected map[string]interface{}
		if err := c.Unpack(&expected, opts...); err != nil {
			t.Fatal(err)
		}

		// reset test.t to zero value
		v := chaseValue(reflect.ValueOf(test.t))
		v.Set(reflect.Zero(v.Type()))

		// create new config from test
		t.Logf("new from: %v", test.cfg)
		if c, err = NewFrom(test.cfg, opts...); err != nil {
			t.Fatal(err)
		}

		// unpack config into zeroed out config
		if err := c.Unpack(test.t, opts...); err != nil {
			t.Fatal(i, err)
		}

		// parse restored input config
		if c, err = NewFrom(test.t, opts...); err != nil {
			t.Fatal(err)
		}

		// validate
		var actual map[string]interface{}
		if err := c.Unpack(&actual, opts...); err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, expected, actual)
	}
}

func TestIgnoreStructFields(t *testing.T) {
	tests := []struct {
		name   string
		config interface{}
	}{
		{
			"ignore private field",
			&struct {
				OK     string `config:"ok"`
				ignore string
			}{
				ignore: "should not be present",
			},
		},
		{
			"honor ignore option",
			&struct {
				OK     string `config:"ok"`
				Ignore string `config:",ignore"`
			}{
				Ignore: "should not be present",
			},
		},
	}

	for i, test := range tests {
		t.Logf("run test (%v): %v", i, test.name)

		testConfig, err := NewFrom(map[string]interface{}{
			"ignored": "test",
			"ok":      "ok",
		})
		if err != nil {
			t.Fatal(err)
		}

		err = testConfig.Unpack(test.config)
		if err != nil {
			t.Error(err)
			continue
		}

		t.Logf("after unpack: %#v", test.config)

		tmp, err := NewFrom(test.config)
		if err != nil {
			t.Error(err)
			continue
		}

		actual := map[string]interface{}{}
		if err := tmp.Unpack(&actual); err != nil {
			t.Error(err)
			continue
		}

		t.Logf("reconstructed: %v", actual)

		assert.Equal(t, "ok", actual["ok"])
		result, exists := actual["ignore"]
		assert.Equal(t, nil, result)
		assert.False(t, exists)
	}
}

func resolveTestEnv(e testEnv) Option {
	fail := func(name string) (string, parse.Config, error) {
		return "", parse.EnvConfig, fmt.Errorf("empty environment variable %v", name)
	}

	if e == nil {
		return Resolve(fail)
	}
	return Resolve(func(name string) (string, parse.Config, error) {
		if v := e[name]; v != "" {
			return v, parse.EnvConfig, nil
		}
		return fail(name)
	})
}
