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

//go:build !integration
// +build !integration

//copied from github.com/elastic/beats
//https://github.com/elastic/beats/blob/master/LICENSE
//Licensed under the Apache License, Version 2.0 (the "License");

package util

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapStrUpdate(t *testing.T) {
	assert := assert.New(t)

	a := MapStr{
		"a": 1,
		"b": 2,
	}
	b := MapStr{
		"b": 3,
		"c": 4,
	}

	a.Update(b)

	assert.Equal(a, MapStr{"a": 1, "b": 3, "c": 4})
}

func TestMapStrUnion(t *testing.T) {
	assert := assert.New(t)

	a := MapStr{
		"a": 1,
		"b": 2,
	}
	b := MapStr{
		"b": 3,
		"c": 4,
	}

	c := MapStrUnion(a, b)

	assert.Equal(c, MapStr{"a": 1, "b": 3, "c": 4})
}

func TestMapStrCopyFieldsTo(t *testing.T) {
	assert := assert.New(t)

	m := MapStr{
		"a": MapStr{
			"a1": 2,
			"a2": 3,
		},
		"b": 2,
		"c": MapStr{
			"c1": 1,
			"c2": 2,
			"c3": MapStr{
				"c31": 1,
				"c32": 2,
			},
		},
	}
	c := MapStr{}

	err := m.CopyFieldsTo(c, "dd")
	assert.Error(err)
	assert.Equal(MapStr{}, c)

	err = m.CopyFieldsTo(c, "a")
	assert.Equal(nil, err)
	assert.Equal(MapStr{"a": MapStr{"a1": 2, "a2": 3}}, c)

	err = m.CopyFieldsTo(c, "c.c1")
	assert.Equal(nil, err)
	assert.Equal(MapStr{"a": MapStr{"a1": 2, "a2": 3}, "c": MapStr{"c1": 1}}, c)

	err = m.CopyFieldsTo(c, "b")
	assert.Equal(nil, err)
	assert.Equal(MapStr{"a": MapStr{"a1": 2, "a2": 3}, "c": MapStr{"c1": 1}, "b": 2}, c)

	err = m.CopyFieldsTo(c, "c.c3.c32")
	assert.Equal(nil, err)
	assert.Equal(MapStr{"a": MapStr{"a1": 2, "a2": 3}, "c": MapStr{"c1": 1, "c3": MapStr{"c32": 2}}, "b": 2}, c)

	m = MapStr{
		"c": []MapStr{
			MapStr{
				"c1": 1,
				"c2": 2,
			},
		},
	}
	c = MapStr{}

	err = m.CopyFieldsTo(c, "c.0.c2")
	assert.Nil(err)
	assert.Equal(MapStr{"c": MapStr{"0": MapStr{"c2": 2}}}, c)

	m = MapStr{
		"c": MapStr{
			"0": MapStr{
				"c1": 1,
				"c2": 2,
			},
		},
	}
	c = MapStr{}

	err = m.CopyFieldsTo(c, "c.0.c2")
	assert.Nil(err)
	assert.Equal(MapStr{"c": MapStr{"0": MapStr{"c2": 2}}}, c)
}

func TestMapStrDelete(t *testing.T) {
	assert := assert.New(t)

	m := MapStr{
		"c": MapStr{
			"c1": 1,
			"c2": 2,
			"c3": MapStr{
				"c31": 1,
				"c32": 2,
			},
		},
	}

	err := m.Delete("c.c2")
	assert.Equal(nil, err)
	assert.Equal(MapStr{"c": MapStr{"c1": 1, "c3": MapStr{"c31": 1, "c32": 2}}}, m)

	err = m.Delete("c.c2.c21")
	assert.NotEqual(nil, err)
	assert.Equal(MapStr{"c": MapStr{"c1": 1, "c3": MapStr{"c31": 1, "c32": 2}}}, m)

	err = m.Delete("c.c3.c31")
	assert.Equal(nil, err)
	assert.Equal(MapStr{"c": MapStr{"c1": 1, "c3": MapStr{"c32": 2}}}, m)

	err = m.Delete("c")
	assert.Equal(nil, err)
	assert.Equal(MapStr{}, m)

	m = MapStr{
		"c": []MapStr{
			MapStr{
				"c1": 1,
				"c2": 2,
			},
		},
	}

	err = m.Delete("c.0.c2")
	assert.Nil(err)
	assert.Equal(MapStr{"c": []MapStr{MapStr{"c1": 1}}}, m)

	m = MapStr{
		"c": MapStr{
			"0": MapStr{
				"c1": 1,
				"c2": 2,
			},
		},
	}

	err = m.Delete("c.0.c2")
	assert.Nil(err)
	assert.Equal(MapStr{"c": MapStr{"0": MapStr{"c1": 1}}}, m)
}

func TestHasKey(t *testing.T) {
	assert := assert.New(t)

	m := MapStr{
		"c": MapStr{
			"c1": 1,
			"c2": 2,
			"c3": MapStr{
				"c31": 1,
				"c32": 2,
			},
		},
	}

	hasKey, err := m.HasKey("c.c2")
	assert.Equal(nil, err)
	assert.Equal(true, hasKey)

	hasKey, err = m.HasKey("c.c4")
	assert.Equal(nil, err)
	assert.Equal(false, hasKey)

	hasKey, err = m.HasKey("c.c3.c32")
	assert.Equal(nil, err)
	assert.Equal(true, hasKey)

	hasKey, err = m.HasKey("dd")
	assert.Equal(nil, err)
	assert.Equal(false, hasKey)

	m = MapStr{
		"c": []MapStr{
			MapStr{
				"c1": 1,
				"c2": 2,
			},
		},
	}

	hasKey, err = m.HasKey("c.0.c2")
	assert.Nil(err)
	assert.Equal(true, hasKey)

	hasKey, err = m.HasKey("c.1.c2")
	assert.NotNil(err)
	assert.Equal(false, hasKey)

	m = MapStr{
		"c": MapStr{
			"0": MapStr{
				"c1": 1,
				"c2": 2,
			},
		},
	}

	hasKey, err = m.HasKey("c.0.c2")
	assert.Nil(err)
	assert.Equal(true, hasKey)

	hasKey, err = m.HasKey("c.1.c2")
	assert.NotNil(err)
	assert.Equal(false, hasKey)
}

func TestMapStrPut(t *testing.T) {
	m := MapStr{
		"subMap": MapStr{
			"a": 1,
		},
	}

	// Add new value to the top-level.
	v, err := m.Put("a", "ok")
	assert.NoError(t, err)
	assert.Nil(t, v)
	assert.Equal(t, MapStr{"a": "ok", "subMap": MapStr{"a": 1}}, m)

	// Add new value to subMap.
	v, err = m.Put("subMap.b", 2)
	assert.NoError(t, err)
	assert.Nil(t, v)
	assert.Equal(t, MapStr{"a": "ok", "subMap": MapStr{"a": 1, "b": 2}}, m)

	// Overwrite a value in subMap.
	v, err = m.Put("subMap.a", 2)
	assert.NoError(t, err)
	assert.Equal(t, 1, v)
	assert.Equal(t, MapStr{"a": "ok", "subMap": MapStr{"a": 2, "b": 2}}, m)

	// Add value to map that does not exist.
	m = MapStr{}
	v, err = m.Put("subMap.newMap.a", 1)
	assert.NoError(t, err)
	assert.Nil(t, v)
	assert.Equal(t, MapStr{"subMap": MapStr{"newMap": MapStr{"a": 1}}}, m)

	m = MapStr{
		"c": []MapStr{
			MapStr{
				"c1": 1,
			},
		},
	}

	v, err = m.Put("c.0.c2", 2)
	assert.Nil(t, err)
	assert.Nil(t, v)
	assert.Equal(t, MapStr{"c": []MapStr{MapStr{"c1": 1, "c2": 2}}}, m)

	m = MapStr{
		"c": MapStr{
			"0": MapStr{
				"c1": 1,
			},
		},
	}

	v, err = m.Put("c.0.c2", 2)
	assert.Nil(t, err)
	assert.Nil(t, v)
	assert.Equal(t, MapStr{"c": MapStr{"0": MapStr{"c1": 1, "c2": 2}}}, m)
}

func TestClone(t *testing.T) {
	assert := assert.New(t)

	m := MapStr{
		"c1": 1,
		"c2": 2,
		"c3": MapStr{
			"c31": 1,
			"c32": 2,
		},
	}

	c := m.Clone()
	assert.Equal(MapStr{"c31": 1, "c32": 2}, c["c3"])
}

func TestString(t *testing.T) {
	type io struct {
		Input  MapStr
		Output string
	}
	tests := []io{
		{
			Input: MapStr{
				"a": "b",
			},
			Output: `{"a":"b"}`,
		},
		{
			Input: MapStr{
				"a": []int{1, 2, 3},
			},
			Output: `{"a":[1,2,3]}`,
		},
	}
	for _, test := range tests {
		assert.Equal(t, test.Output, test.Input.String())
	}
}

// Smoke test. The method has no observable outputs so this
// is only verifying there are no panics.
func TestStringToPrint(t *testing.T) {
	m := MapStr{}

	assert.Equal(t, "{}", m.StringToPrint())
	assert.Equal(t, true, len(m.StringToPrint()) > 0)
}

func TestMergeFields(t *testing.T) {
	type io struct {
		UnderRoot bool
		Event     MapStr
		Fields    MapStr
		Output    MapStr
		Err       string
	}
	tests := []io{
		// underRoot = true, merges
		{
			UnderRoot: true,
			Event: MapStr{
				"a": "1",
			},
			Fields: MapStr{
				"b": 2,
			},
			Output: MapStr{
				"a": "1",
				"b": 2,
			},
		},

		// underRoot = true, overwrites existing
		{
			UnderRoot: true,
			Event: MapStr{
				"a": "1",
			},
			Fields: MapStr{
				"a": 2,
			},
			Output: MapStr{
				"a": 2,
			},
		},

		// underRoot = false, adds new 'fields' when it doesn't exist
		{
			UnderRoot: false,
			Event: MapStr{
				"a": "1",
			},
			Fields: MapStr{
				"a": 2,
			},
			Output: MapStr{
				"a": "1",
				"fields": MapStr{
					"a": 2,
				},
			},
		},

		// underRoot = false, merge with existing 'fields' and overwrites existing keys
		{
			UnderRoot: false,
			Event: MapStr{
				"fields": MapStr{
					"a": "1",
					"b": 2,
				},
			},
			Fields: MapStr{
				"a": 3,
				"c": 4,
			},
			Output: MapStr{
				"fields": MapStr{
					"a": 3,
					"b": 2,
					"c": 4,
				},
			},
		},

		// underRoot = false, error when 'fields' is wrong type
		{
			UnderRoot: false,
			Event: MapStr{
				"fields": "not a MapStr",
			},
			Fields: MapStr{
				"a": 3,
			},
			Output: MapStr{
				"fields": "not a MapStr",
			},
			Err: "expected map",
		},
	}

	for _, test := range tests {
		err := MergeFields(test.Event, test.Fields, test.UnderRoot)
		assert.Equal(t, test.Output, test.Event)
		if test.Err != "" {
			assert.Contains(t, err.Error(), test.Err)
		} else {
			assert.NoError(t, err)
		}
	}
}

func TestAddTag(t *testing.T) {
	type io struct {
		Event  MapStr
		Tags   []string
		Output MapStr
		Err    string
	}
	tests := []io{
		// No existing tags, creates new tag array
		{
			Event: MapStr{},
			Tags:  []string{"json"},
			Output: MapStr{
				"tags": []string{"json"},
			},
		},
		// Existing tags, appends
		{
			Event: MapStr{
				"tags": []string{"json"},
			},
			Tags: []string{"docker"},
			Output: MapStr{
				"tags": []string{"json", "docker"},
			},
		},
		// Existing tags is not a []string
		{
			Event: MapStr{
				"tags": "not a slice",
			},
			Tags: []string{"docker"},
			Output: MapStr{
				"tags": "not a slice",
			},
			Err: "expected string array",
		},
	}

	for _, test := range tests {
		err := AddTags(test.Event, test.Tags)
		assert.Equal(t, test.Output, test.Event)
		if test.Err != "" {
			assert.Contains(t, err.Error(), test.Err)
		} else {
			assert.NoError(t, err)
		}
	}
}

func TestMapstrFlatten(t *testing.T) {
	type data struct {
		Event    MapStr
		Expected MapStr
	}
	tests := []data{
		{
			Event: MapStr{
				"hello": MapStr{
					"world": 15,
				},
			},
			Expected: MapStr{
				"hello.world": 15,
			},
		},
		{
			Event: MapStr{
				"test": 15,
			},
			Expected: MapStr{
				"test": 15,
			},
		},
		{
			Event: MapStr{
				"test": 15,
				"hello": MapStr{
					"world": MapStr{
						"ok": "test",
					},
				},
				"elastic": MapStr{
					"for": "search",
				},
			},
			Expected: MapStr{
				"test":           15,
				"hello.world.ok": "test",
				"elastic.for":    "search",
			},
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.Expected, test.Event.Flatten())
	}
}

func TestGetValueByKeys(t *testing.T) {
	obj := MapStr{
		"obj1": MapStr{
			"obj2": MapStr{
				"obj3": "good",
			},
		},
	}
	va, ok := GetValueByKeys([]string{"obj1", "obj2", "obj3"}, obj)
	fmt.Println(va, ok)
	assert.Equal(t, "good", va)

	va, ok = GetValueByKeys([]string{"obj1", "obj2"}, obj)
	fmt.Println(va, ok)

}
