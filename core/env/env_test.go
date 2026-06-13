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

package env

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"infini.sh/framework/core/config"
)

func TestParseConfigSection_NilConfig(t *testing.T) {
	var out struct{ Foo string }
	exist, err := ParseConfigSection(nil, "anykey", &out)

	assert.False(t, exist)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cfg is nil")
}

func TestParseConfigSection_MissingKey_ReturnsFalseNil(t *testing.T) {
	cfg, err := config.NewConfigFrom(map[string]interface{}{
		"other_section": map[string]interface{}{"a": 1},
	})
	require.NoError(t, err)

	var out struct{ Foo string }
	exist, err := ParseConfigSection(cfg, "nonexistent_key", &out)

	assert.False(t, exist)
	assert.NoError(t, err, "missing key should return nil error, not ErrMissing")
}

func TestParseConfigSection_MissingKey_EmptyConfig(t *testing.T) {
	// Empty config: any key is missing.
	cfg, err := config.NewConfigFrom(map[string]interface{}{})
	require.NoError(t, err)

	var out struct{ Foo string }
	exist, err := ParseConfigSection(cfg, "anykey", &out)

	assert.False(t, exist)
	assert.NoError(t, err)
}

func TestParseConfigSection_ExistingKey_UnpackOK(t *testing.T) {
	cfg, err := config.NewConfigFrom(map[string]interface{}{
		"mysection": map[string]interface{}{
			"foo": "bar",
			"num": 42,
		},
	})
	require.NoError(t, err)

	var out struct {
		Foo string `config:"foo"`
		Num int    `config:"num"`
	}
	exist, err := ParseConfigSection(cfg, "mysection", &out)

	assert.True(t, exist)
	assert.NoError(t, err)
	assert.Equal(t, "bar", out.Foo)
	assert.Equal(t, 42, out.Num)
}

func TestParseConfigSection_ExistingKey_UnpackFails(t *testing.T) {
	cfg, err := config.NewConfigFrom(map[string]interface{}{
		"mysection": map[string]interface{}{
			"foo": "not_an_int",
		},
	})
	require.NoError(t, err)

	var out struct {
		Foo int `config:"foo"`
	}
	exist, err := ParseConfigSection(cfg, "mysection", &out)

	assert.True(t, exist)
	require.Error(t, err)
}

func TestParseConfigSection_KeyExistsButPrimitive_ReturnsError(t *testing.T) {
	// Key exists but value is primitive (string), not an object. Child returns type error.
	cfg, err := config.NewConfigFrom(map[string]interface{}{
		"mysection": "a_string_value",
	})
	require.NoError(t, err)

	var out struct{ Foo string }
	exist, err := ParseConfigSection(cfg, "mysection", &out)

	assert.False(t, exist)
	require.Error(t, err)
}