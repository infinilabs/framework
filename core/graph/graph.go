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

/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package graph

type NestedNode struct {
	Category    string        `json:"category,omitempty"`
	Name        string        `json:"name,omitempty"`
	Description string        `json:"description,omitempty"`
	Value       float64       `json:"value,omitempty"`
	Children    []*NestedNode `json:"children,omitempty"`
}

func (n *NestedNode) Lookup(name string) *NestedNode {
	for _, v := range n.Children {
		if v.Name == name {
			return v
		}
	}
	return nil
}

func (n *NestedNode) Add(name, desc string, value float64) {
	node := n.Lookup(name)
	if node == nil {
		n.Children = append(n.Children, &NestedNode{Name: name, Description: desc, Value: value})
	}
}
