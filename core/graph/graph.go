/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package graph

type NestedNode struct {
	Category string `json:"category,omitempty"`
	Name string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Value float64 `json:"value,omitempty"`
	Children []*NestedNode `json:"children,omitempty"`

}

func (n *NestedNode) Lookup(name string)*NestedNode {
	for _,v:=range n.Children{
		if v.Name==name{
			return v
		}
	}
	return nil
}

func (n *NestedNode) Add(name , desc string, value float64) {
	node:=n.Lookup(name)
	if node==nil{
		n.Children=append(n.Children,&NestedNode{Name: name,Description: desc,Value: value})
	}
}
