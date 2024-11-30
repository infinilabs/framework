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
	"crypto/rand"
	"testing"
)

var stringKeys [1000]string // random string keys
const bytesPerKey = 30

var pathKeys [1000]string // random /paths/of/parts keys
const partsPerKey = 3     // (e.g. /a/b/c has parts /a, /b, /c)
const bytesPerPart = 10

func init() {
	// string keys
	for i := 0; i < len(stringKeys); i++ {
		key := make([]byte, bytesPerKey)
		if _, err := rand.Read(key); err != nil {
			panic("error generating random byte slice")
		}
		stringKeys[i] = string(key)
	}

	// path keys
	for i := 0; i < len(pathKeys); i++ {
		var key string
		for j := 0; j < partsPerKey; j++ {
			key += "/"
			part := make([]byte, bytesPerPart)
			if _, err := rand.Read(part); err != nil {
				panic("error generating random byte slice")
			}
			key += string(part)
		}
		pathKeys[i] = key
	}
}

// RuneTrie
///////////////////////////////////////////////////////////////////////////////

// string keys

func BenchmarkRuneTriePutStringKey(b *testing.B) {
	trie := NewRuneTrie()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		trie.Put(stringKeys[i%len(stringKeys)], i)
	}
}

func BenchmarkRuneTrieGetStringKey(b *testing.B) {
	trie := NewRuneTrie()
	for i := 0; i < b.N; i++ {
		trie.Put(stringKeys[i%len(stringKeys)], i)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		trie.Get(stringKeys[i%len(stringKeys)])
	}
}

// path keys

func BenchmarkRuneTriePutPathKey(b *testing.B) {
	trie := NewRuneTrie()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		trie.Put(pathKeys[i%len(pathKeys)], i)
	}
}

func BenchmarkRuneTrieGetPathKey(b *testing.B) {
	trie := NewRuneTrie()
	for i := 0; i < b.N; i++ {
		trie.Put(pathKeys[i%len(pathKeys)], i)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		trie.Get(pathKeys[i%len(pathKeys)])
	}
}

// PathTrie
///////////////////////////////////////////////////////////////////////////////

// string keys

func BenchmarkPathTriePutStringKey(b *testing.B) {
	trie := NewPathTrie()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		trie.Put(stringKeys[i%len(stringKeys)], i)
	}
}

func BenchmarkPathTrieGetStringKey(b *testing.B) {
	trie := NewPathTrie()
	for i := 0; i < b.N; i++ {
		trie.Put(stringKeys[i%len(stringKeys)], i)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		trie.Get(stringKeys[i%len(stringKeys)])
	}
}

// path keys

func BenchmarkPathTriePutPathKey(b *testing.B) {
	trie := NewPathTrie()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		trie.Put(pathKeys[i%len(pathKeys)], i)
	}
}

func BenchmarkPathTrieGetPathKey(b *testing.B) {
	trie := NewPathTrie()
	for i := 0; i < b.N; i++ {
		trie.Put(pathKeys[i%len(pathKeys)], i)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		trie.Get(pathKeys[i%len(pathKeys)])
	}
}

// benchmark PathSegmenter

func BenchmarkPathSegmenter(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for j := 0; j < b.N; j++ {
		for part, i := PathSegmenter(pathKeys[j%len(pathKeys)], 0); ; part, i = PathSegmenter(pathKeys[j%len(pathKeys)], i) {
			var _ = part // NoOp 'use' the key part
			if i == -1 {
				break
			}
		}
	}
}
