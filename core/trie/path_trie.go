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

// PathTrie is a trie of paths with string keys and interface{} values.

// PathTrie is a trie of string keys and interface{} values. Internal nodes
// have nil values so stored nil values cannot be distinguished and are
// excluded from walks. By default, PathTrie will segment keys by forward
// slashes with PathSegmenter (e.g. "/a/b/c" -> "/a", "/b", "/c"). A custom
// StringSegmenter may be used to customize how strings are segmented into
// nodes. A classic trie might segment keys by rune (i.e. unicode points).
type PathTrie struct {
	segmenter StringSegmenter // key segmenter, must not cause heap allocs
	value     interface{}
	children  map[string]*PathTrie
}

// NewPathTrie allocates and returns a new *PathTrie.
func NewPathTrie() *PathTrie {
	return &PathTrie{
		segmenter: PathSegmenter,
		children:  make(map[string]*PathTrie),
	}
}

// Value returns the value at the current trie node
func (trie *PathTrie) Value() interface{} {
	return trie.value
}

// Children returns the immediate children at the current trie node
func (trie *PathTrie) Children() []Trier {
	kids := make([]Trier, 0, len(trie.children))
	for _, v := range trie.children {
		kids = append(kids, v)
	}

	return kids
}

// Get returns the value stored at the given key. Returns nil for internal
// nodes or for nodes with a value of nil.
func (trie *PathTrie) Get(key string) interface{} {
	node := trie
	for part, i := trie.segmenter(key, 0); ; part, i = trie.segmenter(key, i) {
		node = node.children[part]
		if node == nil {
			return nil
		}
		if i == -1 {
			break
		}
	}
	return node.value
}

// GetPath returns all values stored at each node in the path from the root to
// the given key. Does not return values for internal nodes or for nodes with a
// nil value.
func (trie *PathTrie) GetPath(key string) []interface{} {
	var values []interface{}
	node := trie
	for part, i := trie.segmenter(key, 0); ; part, i = trie.segmenter(key, i) {
		node = node.children[part]
		if node == nil {
			return nil
		}
		if node.value != nil {
			values = append(values, node.value)
		}
		if i == -1 {
			break
		}
	}
	return values
}

// Put inserts the value into the trie at the given key, replacing any
// existing items. It returns true if the put adds a new value, false
// if it replaces an existing value.
// Note that internal nodes have nil values so a stored nil value will not
// be distinguishable and will not be included in Walks.
func (trie *PathTrie) Put(key string, value interface{}) bool {
	node := trie
	for part, i := trie.segmenter(key, 0); ; part, i = trie.segmenter(key, i) {
		child, _ := node.children[part]
		if child == nil {
			child = NewPathTrie()
			node.children[part] = child
		}
		node = child
		if i == -1 {
			break
		}
	}
	// does node have an existing value?
	isNewVal := node.value == nil
	node.value = value
	return isNewVal
}

// Delete removes the value associated with the given key. Returns true if a
// node was found for the given key. If the node or any of its ancestors
// becomes childless as a result, it is removed from the trie.
func (trie *PathTrie) Delete(key string) bool {
	var path []nodeStr // record ancestors to check later
	node := trie
	for part, i := trie.segmenter(key, 0); ; part, i = trie.segmenter(key, i) {
		path = append(path, nodeStr{part: part, node: node})
		node = node.children[part]
		if node == nil {
			// node does not exist
			return false
		}
		if i == -1 {
			break
		}
	}
	// delete the node value
	node.value = nil
	// if leaf, remove it from its parent's children map. Repeat for ancestor path.
	if node.isLeaf() {
		// iterate backwards over path
		for i := len(path) - 1; i >= 0; i-- {
			parent := path[i].node
			part := path[i].part
			delete(parent.children, part)
			if parent.value != nil || !parent.isLeaf() {
				// parent has a value or has other children, stop
				break
			}
		}
	}
	return true // node (internal or not) existed and its value was nil'd
}

// Node returns the trie node with the given key.
// Returns nil if the node with the given key is not found
func (trie *PathTrie) Node(key string) Trier {
	node := trie
	for part, i := trie.segmenter(key, 0); ; part, i = trie.segmenter(key, i) {
		node = node.children[part]
		if node == nil {
			return nil
		}
		if i == -1 {
			break
		}
	}
	return node
}

// Walk iterates over each key/value stored in the trie and calls the given
// walker function with the key and value. If the walker function returns
// an error, the walk is aborted.
// The traversal is depth first with no guaranteed order.
func (trie *PathTrie) Walk(walker WalkFunc) error {
	return trie.walk("", walker)
}

// PathTrie node and the part string key of the child the path descends into.
type nodeStr struct {
	node *PathTrie
	part string
}

func (trie *PathTrie) walk(key string, walker WalkFunc) error {
	if trie.value != nil {
		if err := walker(key, trie.value); err != nil {
			return err
		}
	}
	for part, child := range trie.children {
		if err := child.walk(key+part, walker); err != nil {
			return err
		}
	}
	return nil
}

func (trie *PathTrie) isLeaf() bool {
	return len(trie.children) == 0
}
