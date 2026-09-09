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

package param

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"infini.sh/framework/core/util"
)

// Peek must resolve keys exactly like GetValue — only the miss reporting
// differs (no error construction) — otherwise a probe can disagree with
// the Get that follows it.
func TestPeek_MatchesGetSemantics(t *testing.T) {
	p := &Parameters{}

	p.Set("record", "r1")                      // flat
	p.Set("a.b", "nested")                     // dot-notation, stored nested by Set
	p.Set("arr", []interface{}{"zero", "one"}) // slice segments by index
	p.Set("s", "plain-string")                 // non-map intermediate
	p.Set("@metadata.tag", "m1")               // metadata branch
	p.Data["x.y"] = "flat-literal"             // dotted key stored flat (bracket form)
	p.Data["nilkey"] = nil                     // present but nil

	cases := []struct {
		key  string
		want interface{}
		ok   bool
	}{
		{"record", "r1", true},
		{"a.b", "nested", true},
		{"x.y", "flat-literal", true},
		{"arr.0", "zero", true},
		{"arr.1", "one", true},
		{"nilkey", nil, true},
		{"missing", nil, false},
		{"a.z", nil, false},    // leaf missing
		{"nope.x", nil, false}, // root missing
		{"s.sub", nil, false},  // non-map intermediate
		{"arr.9", nil, false},  // index out of range
		{"arr.abc", nil, false},
	}
	for _, c := range cases {
		v, ok := p.Peek(ParaKey(c.key))
		assert.Equal(t, c.ok, ok, "key %q", c.key)
		assert.Equal(t, c.want, v, "key %q", c.key)

		got, err := p.GetValue(c.key)
		if c.ok {
			assert.NoError(t, err, "key %q", c.key)
			assert.Equal(t, c.want, got, "Peek disagrees with Get on key %q", c.key)
		} else {
			assert.Error(t, err, "key %q", c.key)
		}
	}
}

func TestPeek_TimestampAndMetadata(t *testing.T) {
	p := &Parameters{}
	now := time.Now()
	p.Set("@timestamp", now)

	v, ok := p.Peek("@timestamp")
	assert.True(t, ok)
	assert.Equal(t, now, v)

	// whole-collection metadata read mirrors Get: init() materializes an
	// empty Meta map, so the key resolves (to the empty map) even before
	// anything is stored under @metadata.*
	v, ok = p.Peek("@metadata")
	assert.True(t, ok)
	assert.Equal(t, util.MapStr{}, v)
	gotMeta, err := p.GetValue("@metadata")
	assert.NoError(t, err)
	assert.Equal(t, gotMeta, v)

	p.Set("@metadata.env", "prod")
	v, ok = p.Peek("@metadata.env")
	assert.True(t, ok)
	assert.Equal(t, "prod", v)

	v, ok = p.Peek("@metadata")
	assert.True(t, ok)
	assert.Equal(t, util.MapStr{"env": "prod"}, v)
}

// The probe must stay allocation-free on the common dot-free miss — that
// is the whole point of Peek over GetValue.
func BenchmarkPeek_Miss(b *testing.B) {
	p := &Parameters{}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if v, ok := p.Peek("record"); ok {
			b.Fatal("unexpected hit", v)
		}
	}
}

func BenchmarkPeek_Hit(b *testing.B) {
	p := &Parameters{}
	p.Set("record", "r1")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if v, ok := p.Peek("record"); !ok || v != "r1" {
			b.Fatal("unexpected miss", v)
		}
	}
}
