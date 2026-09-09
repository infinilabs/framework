// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello#infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// PeekValue walks the map without error construction, so it shares the
// resolution semantics of GetValue (walkMap) only by convention — this
// matrix locks the two walks to the same answers.
func TestPeekValue_MatchesGetValue(t *testing.T) {
	m := MapStr{
		"flat":       "v1",
		"x.y":        "flat-literal", // dotted key stored flat, wins over nesting
		"a":          MapStr{"b": "nested", "list": []interface{}{"zero", "one"}},
		"generic":    map[string]interface{}{"deep": map[string]interface{}{"leaf": 42}},
		"arr":        []interface{}{map[string]interface{}{"id": 7}, "second"},
		"nums":       []int{10, 20, 30},
		"scalar":     "just-a-string",
		"presentNil": nil,
	}

	keys := []struct {
		key  string
		want interface{}
		ok   bool
	}{
		{"flat", "v1", true},
		{"x.y", "flat-literal", true},
		{"a.b", "nested", true},
		{"a.list.1", "one", true},
		{"generic.deep.leaf", 42, true},
		{"arr.0.id", 7, true},
		{"arr.1", "second", true},
		{"nums.2", 30, true},
		{"presentNil", nil, true},
		{"missing", nil, false},
		{"a.missing", nil, false},
		{"missing.leaf", nil, false},
		{"scalar.sub", nil, false},
		{"a.list.9", nil, false},
		{"a.list.abc", nil, false},
		{"nums.9", nil, false},
		{"a.b.c", nil, false}, // leaf is a string, cannot descend
	}

	for _, c := range keys {
		v, ok := m.PeekValue(c.key)
		assert.Equal(t, c.ok, ok, "key %q", c.key)
		assert.Equal(t, c.want, v, "key %q", c.key)

		got, err := m.GetValue(c.key)
		if c.ok {
			assert.NoError(t, err, "key %q", c.key)
			assert.Equal(t, c.want, got, "PeekValue disagrees with GetValue on key %q", c.key)
		} else {
			assert.Error(t, err, "key %q", c.key)
		}
	}
}

func BenchmarkPeekValue_Miss(b *testing.B) {
	m := MapStr{"record": "r1"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, ok := m.PeekValue("missing"); ok {
			b.Fatal("unexpected hit")
		}
	}
}

func BenchmarkPeekValue_Hit(b *testing.B) {
	m := MapStr{"record": "r1"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if v, ok := m.PeekValue("record"); !ok || v != "r1" {
			b.Fatal("unexpected miss")
		}
	}
}
