// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package store

type NodeOps interface {
	AddNode(node Node)
	DelNode(node Node)
	GetNodeAttributes(node Node) (map[string]interface{}, error)
	GetNodeAttribute(node Node, key string) (interface{}, error)
	SetNodeAttributes(node Node, attributes map[string]interface{}) (map[string]interface{}, error)
	SetNodeAttribute(node Node, key string, value interface{}) (map[string]interface{}, error)
	DelNodeAttribute(node Node, key string) (map[string]interface{}, error)
}

type EdgeOps interface {
	AddEdge(edge Edge) error
	DelEdge(edge Edge)
	GetEdgeNodes(edge Edge) (Node, Node, error)
	GetEdgeAttributes(edge Edge) (map[string]interface{}, error)
	GetEdgeAttribute(edge Edge, key string) (interface{}, error)
	SetEdgeAttributes(edge Edge, attributes map[string]interface{}) (map[string]interface{}, error)
	SetEdgeAttribute(edge Edge, key string, value interface{}) (map[string]interface{}, error)
	DelEdgeAttribute(edge Edge, key string) (map[string]interface{}, error)
}

type GraphOps interface {
	ListNodes() ([]Node, error)
	ListNeighbors(node Node) ([]Node, error)
	ListOutboundEdges(node Node) ([]Edge, error)
	ListInboundEdges(node Node) ([]Edge, error)
}

type GraphStorage interface {
	NodeOps
	EdgeOps
	GraphOps
}

type Node struct {
	objType         string
	name, namespace string
	attributes      map[string]interface{}
}

type Edge struct {
	from, to   Node
	attributes map[string]interface{}
}
