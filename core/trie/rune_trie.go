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

// RuneTrie is a trie of runes with string keys and interface{} values.
// Note that internal nodes have nil values so a stored nil value will not
// be distinguishable and will not be included in Walks.
type RuneTrie struct {
	value    interface{}
	children map[rune]*RuneTrie
}

// NewRuneTrie allocates and returns a new *RuneTrie.
func NewRuneTrie() *RuneTrie {
	return &RuneTrie{
		children: make(map[rune]*RuneTrie),
	}
}

// Children returns the immediate children at the current trie node
func (trie *RuneTrie) Children() []Trier {
	kids := make([]Trier, 0, len(trie.children))
	for _, v := range trie.children {
		kids = append(kids, v)
	}

	return kids
}

// Value returns the value at the current trie node
func (trie *RuneTrie) Value() interface{} {
	return trie.value
}

// Get returns the value stored at the given key. Returns nil for internal
// nodes or for nodes with a value of nil.
func (trie *RuneTrie) Get(key string) interface{} {
	node := trie
	for _, r := range key {
		node = node.children[r]
		if node == nil {
			return nil
		}
	}
	return node.value
}

// GetPath returns all values stored at each node in the path from the root to
// the given key. Does not return values for internal nodes or for nodes with a
// nil value.
func (trie *RuneTrie) GetPath(key string) []interface{} {
	var values []interface{}
	node := trie
	for _, r := range key {
		node = node.children[r]
		if node == nil {
			return nil
		}
		if node.value != nil {
			values = append(values, node.value)
		}
	}
	return values
}

// Put inserts the value into the trie at the given key, replacing any
// existing items. It returns true if the put adds a new value, false
// if it replaces an existing value.
// Note that internal nodes have nil values so a stored nil value will not
// be distinguishable and will not be included in Walks.
func (trie *RuneTrie) Put(key string, value interface{}) bool {
	node := trie
	for _, r := range key {
		child, _ := node.children[r]
		if child == nil {
			child = NewRuneTrie()
			node.children[r] = child
		}
		node = child
	}
	// does node have an existing value?
	isNewVal := node.value == nil
	node.value = value
	return isNewVal
}

// Delete removes the value associated with the given key. Returns true if a
// node was found for the given key. If the node or any of its ancestors
// becomes childless as a result, it is removed from the trie.
func (trie *RuneTrie) Delete(key string) bool {
	path := make([]nodeRune, len(key)) // record ancestors to check later
	node := trie
	for i, r := range key {
		path[i] = nodeRune{r: r, node: node}
		node = node.children[r]
		if node == nil {
			// node does not exist
			return false
		}
	}
	// delete the node value
	node.value = nil
	// if leaf, remove it from its parent's children map. Repeat for ancestor path.
	if node.isLeaf() {
		// iterate backwards over path
		for i := len(key) - 1; i >= 0; i-- {
			parent := path[i].node
			r := path[i].r
			delete(parent.children, r)
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
func (trie *RuneTrie) Node(key string) Trier {
	node := trie
	for _, r := range key {
		node = node.children[r]
		if node == nil {
			return nil
		}
	}
	return node
}

// Walk iterates over each key/value stored in the trie and calls the given
// walker function with the key and value. If the walker function returns
// an error, the walk is aborted.
// The traversal is depth first with no guaranteed order.
func (trie *RuneTrie) Walk(walker WalkFunc) error {
	return trie.walk("", walker)
}

// RuneTrie node and the rune key of the child the path descends into.
type nodeRune struct {
	node *RuneTrie
	r    rune
}

func (trie *RuneTrie) walk(key string, walker WalkFunc) error {
	if trie.value != nil {
		if err := walker(key, trie.value); err != nil {
			return err
		}
	}
	for r, child := range trie.children {
		if err := child.walk(key+string(r), walker); err != nil {
			return err
		}
	}
	return nil
}

func (trie *RuneTrie) isLeaf() bool {
	return len(trie.children) == 0
}
