// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package store

import (
	"errors"
	"strings"

	"github.com/autom8ter/dagger"
)

const (
	NodeXIDDelimiter = "."
	EdgeXIDDelimiter = "->"

	NodeXTypePod = "pod"
	NodeXTypeSvc = "service"
	// XXX: Not covered until we know how to represent this
	NodeXTypeExt = "ext"

	EdgeXTypeConnection = "connection"

	ReservedAttributeNodeType  = "node_type"
	ReservedAttributeEdgeType  = "edge_type"
	ReservedAttributeName      = "name"
	ReservedAttributeNamespace = "namespace"
	ReservedAttributeXID       = "xid"
)

type Dagger struct {
	// logger *log.ZapLogger
	graph *dagger.Graph
}

// func NewDaggerStore(logger *log.ZapLogger) *Dagger {
func NewDaggerStore() *Dagger {
	d := dagger.NewGraph()
	graph := &Dagger{
		// logger: logger,
		graph: d,
	}

	return graph
}

func (d *Dagger) AddNode(node Node) {
	xtype := string(NodeXTypeFromString(node.objType))

	if node.attributes == nil {
		node.attributes = make(map[string]interface{})
	}

	node.attributes[ReservedAttributeNodeType] = xtype
	node.attributes[ReservedAttributeName] = node.name
	node.attributes[ReservedAttributeNamespace] = node.namespace

	_ = d.graph.SetNode(dagger.Path{
		XID:   generateNodeXID(node.name, node.namespace),
		XType: xtype,
	}, node.attributes)
}

func (d *Dagger) DelNode(node Node) {
	d.graph.DelNode(dagger.Path{
		XID:   generateNodeXID(node.name, node.namespace),
		XType: NodeXTypeFromString(node.objType),
	})
}

func (d *Dagger) GetNodeAttributes(node Node) (map[string]interface{}, error) {
	n, ok := d.graph.GetNode(dagger.Path{
		XID:   generateNodeXID(node.name, node.namespace),
		XType: NodeXTypeFromString(node.objType),
	})
	if !ok {
		return nil, errors.New("node not found")
	}

	return n.Attributes, nil
}

func (d *Dagger) GetNodeAttribute(node Node, key string) (interface{}, error) {
	n, ok := d.graph.GetNode(dagger.Path{
		XID:   generateNodeXID(node.name, node.namespace),
		XType: NodeXTypeFromString(node.objType),
	})
	if !ok {
		return nil, errors.New("node not found")
	}

	v, ok := n.Attributes[key]
	if !ok {
		return nil, errors.New("attribute not found")
	}
	return v, nil
}

func (d *Dagger) SetNodeAttributes(node Node, attributes map[string]interface{}) (map[string]interface{}, error) {
	// TODO: Prevent overwriting reserved keys
	n, ok := d.graph.GetNode(dagger.Path{
		XID:   generateNodeXID(node.name, node.namespace),
		XType: NodeXTypeFromString(node.objType),
	})
	if !ok {
		return nil, errors.New("node not found")
	}

	for k, v := range attributes {
		n.Attributes[k] = v
	}

	_ = d.graph.SetNode(n.Path, n.Attributes)

	return n.Attributes, nil
}

func (d *Dagger) SetNodeAttribute(node Node, key string, value interface{}) (map[string]interface{}, error) {
	// TODO: Prevent overwriting reserved keys
	n, ok := d.graph.GetNode(dagger.Path{
		XID:   generateNodeXID(node.name, node.namespace),
		XType: NodeXTypeFromString(node.objType),
	})
	if !ok {
		return nil, errors.New("node not found")
	}

	if value == nil {
		return nil, errors.New("value cannot be nil")
	}

	n.Attributes[key] = value

	_ = d.graph.SetNode(n.Path, n.Attributes)

	return n.Attributes, nil
}

func (d *Dagger) DelNodeAttribute(node Node, key string) (map[string]interface{}, error) {
	n, ok := d.graph.GetNode(dagger.Path{
		XID:   generateNodeXID(node.name, node.namespace),
		XType: NodeXTypeFromString(node.objType),
	})
	if !ok {
		return nil, errors.New("node not found")
	}

	delete(n.Attributes, key)

	_ = d.graph.SetNode(n.Path, n.Attributes)

	return n.Attributes, nil
}

func (d *Dagger) AddEdge(edge Edge) error {
	if edge.attributes == nil {
		edge.attributes = make(map[string]interface{})
	}

	edge.attributes[ReservedAttributeEdgeType] = EdgeXTypeConnection

	fromXID := generateNodeXID(edge.from.name, edge.from.namespace)
	edge.attributes["from_"+ReservedAttributeNodeType] = NodeXTypeFromString(edge.from.objType)
	edge.attributes["from_"+ReservedAttributeName] = edge.from.name
	edge.attributes["from_"+ReservedAttributeNamespace] = edge.from.namespace
	edge.attributes["from_"+ReservedAttributeXID] = fromXID

	toXID := generateNodeXID(edge.to.name, edge.to.namespace)
	edge.attributes["to_"+ReservedAttributeNodeType] = NodeXTypeFromString(edge.to.objType)
	edge.attributes["to_"+ReservedAttributeName] = edge.to.name
	edge.attributes["to_"+ReservedAttributeNamespace] = edge.to.namespace
	edge.attributes["to_"+ReservedAttributeXID] = toXID

	if _, err := d.graph.SetEdge(dagger.Path{
		XID:   fromXID,
		XType: NodeXTypeFromString(edge.from.objType),
	}, dagger.Path{
		XID:   toXID,
		XType: NodeXTypeFromString(edge.to.objType),
	}, dagger.Node{
		Path: dagger.Path{
			XID:   generateEdgeXID(fromXID, toXID),
			XType: string(EdgeXTypeConnection),
		},
		Attributes: edge.attributes,
	}); err != nil {
		return err
	}
	return nil
}

func (d *Dagger) DelEdge(edge Edge) {
	fromXID := generateNodeXID(edge.from.name, edge.from.namespace)
	toXID := generateNodeXID(edge.to.name, edge.to.namespace)

	d.graph.DelEdge(dagger.Path{
		XID:   generateEdgeXID(fromXID, toXID),
		XType: string(EdgeXTypeConnection),
	})
}

func (d *Dagger) GetEdgeNodes(edge Edge) (Node, Node, error) {
	toXID := generateNodeXID(edge.to.name, edge.to.namespace)
	fromXID := generateNodeXID(edge.from.name, edge.from.namespace)

	e, ok := d.graph.GetEdge(dagger.Path{
		XID:   generateEdgeXID(fromXID, toXID),
		XType: string(EdgeXTypeConnection),
	})
	if !ok {
		return Node{}, Node{}, errors.New("edge not found")
	}

	from, ok := d.graph.GetNode(e.From)
	if !ok {
		return Node{}, Node{}, errors.New("from node not found")
	}

	to, ok := d.graph.GetNode(e.To)
	if !ok {
		return Node{}, Node{}, errors.New("to node not found")
	}

	fromNode := Node{
		objType:    edge.from.objType,
		name:       edge.from.name,
		namespace:  edge.from.namespace,
		attributes: from.Attributes,
	}
	toNode := Node{
		objType:    edge.to.objType,
		name:       edge.to.name,
		namespace:  edge.to.namespace,
		attributes: to.Attributes,
	}
	return fromNode, toNode, nil
}

func (d *Dagger) GetEdgeAttributes(edge Edge) (map[string]interface{}, error) {
	toXID := generateNodeXID(edge.to.name, edge.to.namespace)
	fromXID := generateNodeXID(edge.from.name, edge.from.namespace)

	e, ok := d.graph.GetEdge(dagger.Path{
		XID:   generateEdgeXID(fromXID, toXID),
		XType: string(EdgeXTypeConnection),
	})
	if !ok {
		return nil, errors.New("edge not found")
	}
	return e.Attributes, nil
}

func (d *Dagger) GetEdgeAttribute(edge Edge, key string) (interface{}, error) {
	toXID := generateNodeXID(edge.to.name, edge.to.namespace)
	fromXID := generateNodeXID(edge.from.name, edge.from.namespace)

	e, ok := d.graph.GetEdge(dagger.Path{
		XID:   generateEdgeXID(fromXID, toXID),
		XType: string(EdgeXTypeConnection),
	})
	if !ok {
		return nil, errors.New("edge not found")
	}

	v, ok := e.Attributes[key]
	if ok {
		return v, nil
	}

	return nil, errors.New("attribute not found")
}

func (d *Dagger) SetEdgeAttributes(edge Edge, attributes map[string]interface{}) (map[string]interface{}, error) {
	toXID := generateNodeXID(edge.to.name, edge.to.namespace)
	fromXID := generateNodeXID(edge.from.name, edge.from.namespace)

	e, ok := d.graph.GetEdge(dagger.Path{
		XID:   generateEdgeXID(fromXID, toXID),
		XType: string(EdgeXTypeConnection),
	})
	if !ok {
		return nil, errors.New("edge not found")
	}

	for k, v := range attributes {
		e.Attributes[k] = v
	}

	if _, err := d.graph.SetEdge(
		dagger.Path{
			XID:   fromXID,
			XType: NodeXTypeFromString(edge.from.objType),
		},
		dagger.Path{
			XID:   toXID,
			XType: NodeXTypeFromString(edge.to.objType),
		},
		dagger.Node{
			Path: dagger.Path{
				XID:   generateEdgeXID(fromXID, toXID),
				XType: string(EdgeXTypeConnection),
			},
			Attributes: e.Attributes,
		}); err != nil {
		return nil, err
	}

	return e.Attributes, nil
}

func (d *Dagger) SetEdgeAttribute(edge Edge, key string, value interface{}) (map[string]interface{}, error) {
	toXID := generateNodeXID(edge.to.name, edge.to.namespace)
	fromXID := generateNodeXID(edge.from.name, edge.from.namespace)

	e, ok := d.graph.GetEdge(dagger.Path{
		XID:   generateEdgeXID(fromXID, toXID),
		XType: string(EdgeXTypeConnection),
	})
	if !ok {
		return nil, errors.New("edge not found")
	}

	e.Attributes[key] = value

	if _, err := d.graph.SetEdge(
		dagger.Path{
			XID:   fromXID,
			XType: NodeXTypeFromString(edge.from.objType),
		},
		dagger.Path{
			XID:   toXID,
			XType: NodeXTypeFromString(edge.to.objType),
		},
		dagger.Node{
			Path: dagger.Path{
				XID:   generateEdgeXID(fromXID, toXID),
				XType: string(EdgeXTypeConnection),
			},
			Attributes: e.Attributes,
		}); err != nil {
		return nil, err
	}

	return e.Attributes, nil
}

func (d *Dagger) DelEdgeAttribute(edge Edge, key string) (map[string]interface{}, error) {
	toXID := generateNodeXID(edge.to.name, edge.to.namespace)
	fromXID := generateNodeXID(edge.from.name, edge.from.namespace)

	e, ok := d.graph.GetEdge(dagger.Path{
		XID:   generateEdgeXID(fromXID, toXID),
		XType: string(EdgeXTypeConnection),
	})
	if !ok {
		return nil, errors.New("edge not found")
	}

	delete(e.Attributes, key)

	if _, err := d.graph.SetEdge(
		dagger.Path{
			XID:   fromXID,
			XType: NodeXTypeFromString(edge.from.objType),
		},
		dagger.Path{
			XID:   toXID,
			XType: NodeXTypeFromString(edge.to.objType),
		},
		dagger.Node{
			Path: dagger.Path{
				XID:   generateEdgeXID(fromXID, toXID),
				XType: string(EdgeXTypeConnection),
			},
			Attributes: e.Attributes,
		}); err != nil {
		return nil, err
	}

	return e.Attributes, nil
}

func (d *Dagger) ListNodes() ([]Node, error) {
	nodes := []Node{}
	d.graph.RangeNodes(NodeXTypePod, func(n dagger.Node) bool {
		name, namespace := nameNamespaceFromNodeXID(n.Path.XID)
		nodes = append(nodes, Node{
			name:      name,
			namespace: namespace,
			objType:   NodeXTypeToString(n.Path.XType),
		})
		return true
	})

	d.graph.RangeNodes(NodeXTypeSvc, func(n dagger.Node) bool {
		name, namespace := nameNamespaceFromNodeXID(n.Path.XID)
		nodes = append(nodes, Node{
			name:      name,
			namespace: namespace,
			objType:   NodeXTypeToString(n.Path.XType),
		})
		return true
	})

	// XXX: Not covered until we know how to represent this

	// d.graph.RangeNodes(NodeXTypeExt, func(n dagger.Node) bool {
	// 	name, namespace := nameNamespaceFromNodeXID(n.Path.XID)
	// 	nodes = append(nodes, Node{
	// 		name:      name,
	// 		namespace: namespace,
	// 		objType:   NodeXTypeToString(n.Path.XType),
	// 	})
	// 	return true
	// })

	return nodes, nil
}

func (d *Dagger) ListNeighbors(node Node) ([]Node, error) {
	fromXID := generateNodeXID(node.name, node.namespace)

	nodes := []Node{}

	d.graph.RangeEdgesFrom(EdgeXTypeConnection, dagger.Path{
		XID:   fromXID,
		XType: NodeXTypeFromString(node.objType),
	}, func(e dagger.Edge) bool {
		node, _ := d.graph.GetNode(e.To)
		name, namespace := nameNamespaceFromNodeXID(node.Path.XID)
		nodes = append(nodes, Node{
			name:      name,
			namespace: namespace,
			objType:   NodeXTypeToString(node.Path.XType),
		})
		return true
	})

	return nodes, nil
}

func (d *Dagger) ListOutboundEdges(node Node) ([]Edge, error) {
	fromXID := generateNodeXID(node.name, node.namespace)

	edges := []Edge{}

	d.graph.RangeEdgesFrom(EdgeXTypeConnection, dagger.Path{
		XID:   fromXID,
		XType: NodeXTypeFromString(node.objType),
	}, func(e dagger.Edge) bool {
		toName, toNamespace := nameNamespaceFromNodeXID(e.To.XID)
		edges = append(edges, Edge{
			from: node,
			to: Node{
				name:      toName,
				namespace: toNamespace,
				objType:   NodeXTypeToString(e.To.XType),
			},
			attributes: e.Attributes,
		})
		return true
	})

	return edges, nil
}

func (d *Dagger) ListInboundEdges(node Node) ([]Edge, error) {
	toXID := generateNodeXID(node.name, node.namespace)

	edges := []Edge{}

	d.graph.RangeEdgesTo(EdgeXTypeConnection, dagger.Path{
		XID:   toXID,
		XType: NodeXTypeFromString(node.objType),
	}, func(e dagger.Edge) bool {
		fromName, fromNamespace := nameNamespaceFromNodeXID(e.From.XID)
		edges = append(edges, Edge{
			to: node,
			from: Node{
				name:      fromName,
				namespace: fromNamespace,
				objType:   NodeXTypeToString(e.From.XType),
			},
			attributes: e.Attributes,
		})
		return true
	})

	return edges, nil
}

func generateNodeXID(name, namespace string) string {
	return namespace + NodeXIDDelimiter + name
}

func nameNamespaceFromNodeXID(nodeXID string) (string, string) {
	parts := strings.Split(nodeXID, NodeXIDDelimiter)
	return parts[1], parts[0]
}

func NodeXTypeFromString(objType string) string {
	switch strings.ToLower(objType) {
	case "pod":
		return NodeXTypePod
	case "service":
		return NodeXTypeSvc
	default:
		return NodeXTypeExt
	}
}

func NodeXTypeToString(xtype string) string {
	switch strings.ToLower(xtype) {
	case "pod":
		return "pod"
	case "service":
		return "service"
	default:
		// XXX: Not covered until we know how to represent this
		return NodeXTypeExt
	}
}

func generateEdgeXID(fromNodeXID, toNodeXID string) string {
	return fromNodeXID + EdgeXIDDelimiter + toNodeXID
}
