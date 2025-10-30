/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package util

import (
	"github.com/emirpasic/gods/sets/hashset"
	"testing"
)

func newSet(values ...interface{}) *hashset.Set {
	s := hashset.New()
	s.Add(values...)
	return s
}

func TestIsSuperset(t *testing.T) {
	tests := []struct {
		name     string
		a        *hashset.Set
		b        *hashset.Set
		expected bool
	}{
		{
			name:     "both empty",
			a:        newSet(),
			b:        newSet(),
			expected: true, // empty set is superset of empty
		},
		{
			name:     "a empty, b non-empty",
			a:        newSet(),
			b:        newSet("x"),
			expected: false,
		},
		{
			name:     "a non-empty, b empty",
			a:        newSet("x", "y"),
			b:        newSet(),
			expected: true,
		},
		{
			name:     "a == b",
			a:        newSet(1, 2, 3),
			b:        newSet(1, 2, 3),
			expected: true,
		},
		{
			name:     "a larger than b (true superset)",
			a:        newSet(1, 2, 3, 4),
			b:        newSet(2, 3),
			expected: true,
		},
		{
			name:     "a missing one element from b",
			a:        newSet(1, 2),
			b:        newSet(1, 2, 3),
			expected: false,
		},
		{
			name:     "different types - string elements",
			a:        newSet("a", "b", "c"),
			b:        newSet("b", "c"),
			expected: true,
		},
		{
			name:     "b contains element not in a",
			a:        newSet("a", "b"),
			b:        newSet("a", "b", "c"),
			expected: false,
		},
		{
			name:     "subset partially overlapping",
			a:        newSet("x", "y"),
			b:        newSet("y", "z"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSuperset(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("IsSuperset(%v, %v) = %v; want %v", tt.a.Values(), tt.b.Values(), result, tt.expected)
			}
		})
	}
}
