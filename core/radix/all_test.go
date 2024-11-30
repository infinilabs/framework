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

package radix

import (
	"bufio"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPattern(t *testing.T) {
	patterns := []struct {
		s string
		i interface{}
	}{
		{"*", 0},
		{"*abcd*ef*", 1},
		{"*.google.com", 2},
		{"http://example.com/books/*", 3},
		{"*://example.com/movies", 4},
		{`http://example.com/\*`, 5},
		{`http://example.com/*`, 6},
		{"你好*世界*", 7},
		{`foo\`, 8},
		{`b\ar`, 9},
	}
	data := []struct {
		s string
		v interface{}
	}{
		{"abcdef", 1},
		{"abcdefef", 1},
		{"abcabcdefgef", 1},
		{"google.com", 0},
		{"www.google.com", 2},
		{"http://example.com/books/", 3},
		{"http://example.com/", 6},
		{"http://example.com/*", 5},
		{"你好世界", 7},
		{"你你好世界", 0},
		{"你好世界世界界界", 7},
		{"你好,世界", 7},
		{"你好,世界。", 7},
		{`foo\`, 0},
		{`foo`, 8},
		{`b\ar`, 0},
		{`bar`, 9},
	}

	tr := &PatternTrie{}
	for _, p := range patterns {
		tr.Add(p.s, p.i)
	}

	for _, data := range data {
		v, ok := tr.Lookup(data.s)
		assert.True(t, ok)
		assert.Equal(t, data.v, v)
	}

}

func TestMatch(t *testing.T) {
	patterns := []string{
		"hello*world",
		"Hello,*world",
		"*foo*bar",
	}
	pattern := Compile(patterns...)
	assert.True(t, pattern.Match("hello,world"))
	assert.True(t, pattern.Match("Hello,world"))
	assert.False(t, pattern.Match("Helloworld"))
	assert.True(t, pattern.Match("foobar"))
	assert.False(t, pattern.Match("foobar,"))

	assert.False(t, Match(`\*mark\*`, "mark"))
	assert.True(t, Match(`\*mark\*`, "*mark*"))
	assert.True(t, Match(`*abc*`, "aabccc"))
	assert.True(t, Match(`*abc*`, "abc"))
	assert.True(t, Match(`*abc*`, "abcabc"))
	assert.True(t, Match(`*abc*`, "abbabcc"))

	assert.True(t, Match(`*`, "foobar"))
	assert.True(t, Match(`*`, ""))
}

func BenchmarkInsert(b *testing.B) {
	var urls []string
	tr := NewTrie(false)
	f, err := os.Open("testdata/url.txt")
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		s := scanner.Text()
		urls = append(urls, s)
	}
	if err := scanner.Err(); err != nil {
		b.Fatal(err)
	}
	b.N = len(urls)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tr.Add(urls[i], i)
	}
}

func BenchmarkGoRadixInsert(b *testing.B) {
	var urls []string
	tr := NewTrie(false)
	f, err := os.Open("testdata/url.txt")
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		s := scanner.Text()
		urls = append(urls, s)
	}
	if err := scanner.Err(); err != nil {
		b.Fatal(err)
	}
	b.N = len(urls)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tr.Add(urls[i], i)
	}
}

func BenchmarkMapInsert(b *testing.B) {
	var urls []string
	m := map[string]int{}
	f, err := os.Open("testdata/url.txt")
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		s := scanner.Text()
		urls = append(urls, s)
	}
	if err := scanner.Err(); err != nil {
		b.Fatal(err)
	}
	b.N = len(urls)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m[urls[i]] = i
	}
}

func BenchmarkLookup(b *testing.B) {
	var urls []string
	tr := NewTrie(false)
	f, err := os.Open("testdata/url.txt")
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		s := scanner.Text()
		tr.Add(s, len(urls))
		urls = append(urls, s)
	}
	if err := scanner.Err(); err != nil {
		b.Fatal(err)
	}
	b.N = len(urls)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if v, _ := tr.Lookup(urls[i]); v != i {
			b.Errorf("expect %d, got %d\n", i, v)
		}
	}
}

func BenchmarkGoRadixLookup(b *testing.B) {
	var urls []string
	tr := NewTrie(false)
	f, err := os.Open("testdata/url.txt")
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		s := scanner.Text()
		tr.Add(s, len(urls))
		urls = append(urls, s)
	}
	if err := scanner.Err(); err != nil {
		b.Fatal(err)
	}
	b.N = len(urls)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if v, _ := tr.Lookup(urls[i]); v != i {
			b.Errorf("expect %d, got %d\n", i, v)
		}
	}
}

func BenchmarkMapLookup(b *testing.B) {
	var urls []string
	m := map[string]int{}
	f, err := os.Open("testdata/url.txt")
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		s := scanner.Text()
		m[s] = len(urls)
		urls = append(urls, s)
	}
	if err := scanner.Err(); err != nil {
		b.Fatal(err)
	}
	b.N = len(urls)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if v, ok := m[urls[i]]; !ok || v != i {
			b.Errorf("expect %d, got %d\n", i, v)
		}
	}
}
