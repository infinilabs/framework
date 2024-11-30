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

package trie

import (
	"testing"
)

// test splitting /path/keys/ into parts (e.g. /path, /keys, /)
func TestPathSegmenter(t *testing.T) {
	cases := []struct {
		key     string
		parts   []string
		indices []int // indexes to use as next start, in order
	}{
		{"", []string{""}, []int{-1}},
		{"/", []string{"/"}, []int{-1}},
		{"static_file", []string{"static_file"}, []int{-1}},
		{"/users/scott", []string{"/users", "/scott"}, []int{6, -1}},
		{"users/scott", []string{"users", "/scott"}, []int{5, -1}},
		{"/users/ramona/", []string{"/users", "/ramona", "/"}, []int{6, 13, -1}},
		{"users/ramona/", []string{"users", "/ramona", "/"}, []int{5, 12, -1}},
		{"//", []string{"/", "/"}, []int{1, -1}},
		{"/a/b/c", []string{"/a", "/b", "/c"}, []int{2, 4, -1}},
	}

	for _, c := range cases {
		partNum := 0
		for prefix, i := PathSegmenter(c.key, 0); ; prefix, i = PathSegmenter(c.key, i) {
			if prefix != c.parts[partNum] {
				t.Errorf("expected part %d of key '%s' to be '%s', got '%s'", partNum, c.key, c.parts[partNum], prefix)
			}
			if i != c.indices[partNum] {
				t.Errorf("in iteration %d, expected next index of key '%s' to be '%d', got '%d'", partNum, c.key, c.indices[partNum], i)
			}
			partNum++
			if i == -1 {
				break
			}
		}
		if partNum != len(c.parts) {
			t.Errorf("expected '%s' to have %d parts, got %d", c.key, len(c.parts), partNum)
		}
	}
}

func TestPathSegmenterEdgeCases(t *testing.T) {
	cases := []struct {
		path      string
		start     int
		segment   string
		nextIndex int
	}{
		{"", 0, "", -1},
		{"", 10, "", -1},
		{"/", 0, "/", -1},
		{"/", 10, "", -1},
		{"/", -10, "", -1},
		{"/", 1, "", -1},
		{"//", 0, "/", 1},
		{"//", 1, "/", -1},
		{"//", 2, "", -1},
		{" /", 0, " ", 1},
		{" /", 1, "/", -1},
	}

	for _, c := range cases {
		segment, nextIndex := PathSegmenter(c.path, c.start)
		if segment != c.segment {
			t.Errorf("expected segment %s starting at %d in path %s, got %s", c.segment, c.start, c.path, segment)
		}
		if nextIndex != c.nextIndex {
			t.Errorf("expected nextIndex %d starting at %d in path %s, got %d", c.nextIndex, c.start, c.path, nextIndex)
		}
	}
}
