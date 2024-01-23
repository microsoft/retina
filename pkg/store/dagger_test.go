// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
//go:build unit
// +build unit

package store

import (
	"reflect"
	"testing"
)

func arrayEqualNode(x []Node, y []Node) bool {
	for xItem := range x {
		found := false
		for yItem := range y {
			if reflect.DeepEqual(xItem, yItem) {
				found = true
				break
			}
		}

		if !found {
			return false
		}
	}

	return true
}

func arrayEqualEdge(x []Edge, y []Edge) bool {
	for xItem := range x {
		found := false
		for yItem := range y {
			if reflect.DeepEqual(xItem, yItem) {
				found = true
				break
			}
		}

		if !found {
			return false
		}
	}

	return true
}

func TestDagger_GetNodeAttributes(t *testing.T) {
	type fields struct {
		d *Dagger
	}
	type args struct {
		node Node
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name: "additional attributes",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					d.AddNode(Node{
						name:      "test-pod",
						namespace: "test-namespace",
						objType:   "pod",
						attributes: map[string]interface{}{
							"test-key": "test-value",
						},
					})
					return d
				}(),
			},
			args: args{
				node: Node{
					objType:   "pod",
					name:      "test-pod",
					namespace: "test-namespace",
				},
			},
			want: map[string]interface{}{
				ReservedAttributeNodeType:  "pod",
				ReservedAttributeName:      "test-pod",
				ReservedAttributeNamespace: "test-namespace",
				"test-key":                 "test-value",
			},
			wantErr: false,
		},
		{
			name: "node not found",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					return d
				}(),
			},
			args: args{
				node: Node{
					objType:   "pod",
					name:      "test-pod",
					namespace: "test-namespace",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "nil attributes",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					d.AddNode(Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					})
					return d
				}(),
			},
			args: args{
				node: Node{
					objType:   "pod",
					name:      "test-pod",
					namespace: "test-namespace",
				},
			},
			want: map[string]interface{}{
				ReservedAttributeNodeType:  "pod",
				ReservedAttributeName:      "test-pod",
				ReservedAttributeNamespace: "test-namespace",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fields.d.GetNodeAttributes(tt.args.node)
			if (err != nil) != tt.wantErr {
				t.Errorf("Dagger.GetNodeAttributes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Dagger.GetNodeAttributes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDagger_GetNodeAttribute(t *testing.T) {
	type fields struct {
		d *Dagger
	}
	type args struct {
		node Node
		key  string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "attribute exists",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					d.AddNode(Node{
						name:      "test-pod",
						namespace: "test-namespace",
						objType:   "pod",
						attributes: map[string]interface{}{
							"test-key": "test-value",
						},
					})
					return d
				}(),
			},
			args: args{
				node: Node{
					objType:   "pod",
					name:      "test-pod",
					namespace: "test-namespace",
				},
				key: "test-key",
			},
			want:    "test-value",
			wantErr: false,
		},
		{
			name: "attribute does not exist",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					d.AddNode(Node{
						name:      "test-pod",
						namespace: "test-namespace",
						objType:   "pod",
					})
					return d
				}(),
			},
			args: args{
				node: Node{
					objType:   "pod",
					name:      "test-pod",
					namespace: "test-namespace",
				},
				key: "test-key",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "node  not found",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					return d
				}(),
			},
			args: args{
				node: Node{
					objType:   "pod",
					name:      "test-pod",
					namespace: "test-namespace",
				},
				key: "test-key",
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fields.d.GetNodeAttribute(tt.args.node, tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("Dagger.GetNodeAttribute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Dagger.GetNodeAttribute() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDagger_SetNodeAttributes(t *testing.T) {
	type fields struct {
		d *Dagger
	}
	type args struct {
		node       Node
		attributes map[string]interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name: "add multiple attributes",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					d.AddNode(Node{
						name:      "test-pod",
						namespace: "test-namespace",
						objType:   "pod",
					})
					return d
				}(),
			},
			args: args{
				node: Node{
					objType:   "pod",
					name:      "test-pod",
					namespace: "test-namespace",
				},
				attributes: map[string]interface{}{
					"test-key":  "test-value",
					"test-key2": "test-value2",
				},
			},
			want: map[string]interface{}{
				ReservedAttributeNodeType:  "pod",
				ReservedAttributeName:      "test-pod",
				ReservedAttributeNamespace: "test-namespace",
				"test-key":                 "test-value",
				"test-key2":                "test-value2",
			},
			wantErr: false,
		},
		{
			name: "node not found",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					return d
				}(),
			},
			args: args{
				node: Node{
					objType:   "pod",
					name:      "test-pod",
					namespace: "test-namespace",
				},
				attributes: map[string]interface{}{},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "nil attributes",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					d.AddNode(Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					})
					return d
				}(),
			},
			args: args{
				node: Node{
					objType:   "pod",
					name:      "test-pod",
					namespace: "test-namespace",
				},
				attributes: nil,
			},
			want: map[string]interface{}{
				ReservedAttributeNodeType:  "pod",
				ReservedAttributeName:      "test-pod",
				ReservedAttributeNamespace: "test-namespace",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fields.d.SetNodeAttributes(tt.args.node, tt.args.attributes)
			if (err != nil) != tt.wantErr {
				t.Errorf("Dagger.SetNodeAttributes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Dagger.SetNodeAttributes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDagger_SetNodeAttribute(t *testing.T) {
	type fields struct {
		d *Dagger
	}
	type args struct {
		node  Node
		key   string
		value interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name: "add attribute",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					d.AddNode(Node{
						name:      "test-pod",
						namespace: "test-namespace",
						objType:   "pod",
					})
					return d
				}(),
			},
			args: args{
				node: Node{
					objType:   "pod",
					name:      "test-pod",
					namespace: "test-namespace",
				},
				key:   "test-key",
				value: "test-value",
			},
			want: map[string]interface{}{
				ReservedAttributeNodeType:  "pod",
				ReservedAttributeName:      "test-pod",
				ReservedAttributeNamespace: "test-namespace",
				"test-key":                 "test-value",
			},
			wantErr: false,
		},
		{
			name: "node not found",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					return d
				}(),
			},
			args: args{
				node: Node{
					objType:   "pod",
					name:      "test-pod",
					namespace: "test-namespace",
				},
				key:   "test-key",
				value: "test-value",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "nil attribute value",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					d.AddNode(Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					})
					return d
				}(),
			},
			args: args{
				node: Node{
					objType:   "pod",
					name:      "test-pod",
					namespace: "test-namespace",
				},
				key:   "test-key",
				value: nil,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fields.d.SetNodeAttribute(tt.args.node, tt.args.key, tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Dagger.SetNodeAttributes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Dagger.SetNodeAttributes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDagger_DelNodeAttribute(t *testing.T) {
	type fields struct {
		d *Dagger
	}
	type args struct {
		node Node
		key  string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name: "attribute exists",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					d.AddNode(Node{
						name:      "test-pod",
						namespace: "test-namespace",
						objType:   "pod",
						attributes: map[string]interface{}{
							"test-key": "test-value",
						},
					})
					return d
				}(),
			},
			args: args{
				node: Node{
					objType:   "pod",
					name:      "test-pod",
					namespace: "test-namespace",
				},
				key: "test-key",
			},
			want: map[string]interface{}{
				ReservedAttributeNodeType:  "pod",
				ReservedAttributeName:      "test-pod",
				ReservedAttributeNamespace: "test-namespace",
			},
			wantErr: false,
		},
		{
			name: "node not found",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					return d
				}(),
			},
			args: args{
				node: Node{
					objType:   "pod",
					name:      "test-pod",
					namespace: "test-namespace",
				},
				key: "test-key",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "attribute does not exist",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					d.AddNode(Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					})
					return d
				}(),
			},
			args: args{
				node: Node{
					objType:   "pod",
					name:      "test-pod",
					namespace: "test-namespace",
				},
				key: "test-key",
			},
			want: map[string]interface{}{
				ReservedAttributeNodeType:  "pod",
				ReservedAttributeName:      "test-pod",
				ReservedAttributeNamespace: "test-namespace",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fields.d.DelNodeAttribute(tt.args.node, tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("Dagger.SetNodeAttributes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Dagger.SetNodeAttributes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDagger_AddEdge(t *testing.T) {
	type fields struct {
		d *Dagger
	}
	type args struct {
		edge Edge
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "adds an edge to the graph",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					d.AddNode(Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					})
					d.AddNode(Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					})
					return d
				}(),
			},
			args: args{
				edge: Edge{
					from: Node{
						name:      "test-pod",
						namespace: "test-namespace",
						objType:   "pod",
					},
					to: Node{
						name:      "test-service",
						namespace: "test-namespace",
						objType:   "service",
					},
					attributes: nil,
				},
			},
			wantErr: false,
		},
		{
			name: "to node not found",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					d.AddNode(Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					})
					return d
				}(),
			},
			args: args{
				edge: Edge{
					from: Node{
						name:      "test-pod",
						namespace: "test-namespace",
						objType:   "pod",
					},
					to: Node{
						name:      "test-service",
						namespace: "test-namespace",
						objType:   "service",
					},
					attributes: nil,
				},
			},
			wantErr: true,
		},
		{
			name: "from node not found",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					d.AddNode(Node{
						name:       "test-svx",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					})
					return d
				}(),
			},
			args: args{
				edge: Edge{
					from: Node{
						name:      "test-pod",
						namespace: "test-namespace",
						objType:   "pod",
					},
					to: Node{
						name:      "test-service",
						namespace: "test-namespace",
						objType:   "service",
					},
					attributes: nil,
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.fields.d.AddEdge(tt.args.edge); (err != nil) != tt.wantErr {
				t.Errorf("Dagger.AddEdge() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDagger_GetEdgeNodes(t *testing.T) {
	type fields struct {
		d *Dagger
	}
	type args struct {
		edge Edge
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    Node
		want1   Node
		wantErr bool
	}{
		{
			name: "gets the edge nodes",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					from := Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					}
					to := Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					}
					d.AddNode(from)
					d.AddNode(to)
					_ = d.AddEdge(Edge{
						from: from,
						to:   to,
						attributes: map[string]interface{}{
							"test-key": "test-value",
						},
					})
					return d
				}(),
			},
			args: args{
				edge: Edge{
					from: Node{
						name:      "test-pod",
						namespace: "test-namespace",
						objType:   "pod",
					},
					to: Node{
						name:      "test-service",
						namespace: "test-namespace",
						objType:   "service",
					},
					attributes: map[string]interface{}{
						"test-key": "test-value",
					},
				},
			},
			want: Node{
				name:      "test-pod",
				namespace: "test-namespace",
				objType:   "pod",
				attributes: map[string]interface{}{
					ReservedAttributeNodeType:  "pod",
					ReservedAttributeName:      "test-pod",
					ReservedAttributeNamespace: "test-namespace",
				},
			},
			want1: Node{
				name:      "test-service",
				namespace: "test-namespace",
				objType:   "service",
				attributes: map[string]interface{}{
					ReservedAttributeNodeType:  "service",
					ReservedAttributeName:      "test-service",
					ReservedAttributeNamespace: "test-namespace",
				},
			},
			wantErr: false,
		},
		{
			name: "edge does not exist",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					from := Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					}
					to := Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					}
					d.AddNode(from)
					d.AddNode(to)
					return d
				}(),
			},
			args: args{
				edge: Edge{
					from: Node{
						name:      "test-pod",
						namespace: "test-namespace",
						objType:   "pod",
					},
					to: Node{
						name:      "test-service",
						namespace: "test-namespace",
						objType:   "service",
					},
					attributes: map[string]interface{}{
						"test-key": "test-value",
					},
				},
			},
			want:    Node{},
			want1:   Node{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := tt.fields.d.GetEdgeNodes(tt.args.edge)
			if (err != nil) != tt.wantErr {
				t.Errorf("Dagger.GetEdgeNodes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Dagger.GetEdgeNodes() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("Dagger.GetEdgeNodes() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestDagger_GetEdgeAttribute(t *testing.T) {
	type fields struct {
		d *Dagger
	}
	type args struct {
		edge Edge
		key  string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "attribute exists",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					from := Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					}
					to := Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					}
					d.AddNode(from)
					d.AddNode(to)
					d.AddEdge(Edge{
						from: from,
						to:   to,
						attributes: map[string]interface{}{
							"test-key": "test-value",
						},
					})
					return d
				}(),
			},
			args: args{
				edge: Edge{
					from: Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					},
					to: Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					},
				},
				key: "test-key",
			},
			want:    "test-value",
			wantErr: false,
		},
		{
			name: "attribute does not exist",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					from := Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					}
					to := Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					}
					d.AddNode(from)
					d.AddNode(to)
					d.AddEdge(Edge{
						from:       from,
						to:         to,
						attributes: nil,
					})
					return d
				}(),
			},
			args: args{
				edge: Edge{
					from: Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					},
					to: Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					},
				},
				key: "test-key",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "edge does not exist",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					from := Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					}
					to := Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					}
					d.AddNode(from)
					d.AddNode(to)
					return d
				}(),
			},
			args: args{
				edge: Edge{
					from: Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					},
					to: Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					},
				},
				key: "test-key",
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fields.d.GetEdgeAttribute(tt.args.edge, tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("Dagger.GetEdgeAttribute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Dagger.GetEdgeAttribute() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDagger_GetEdgeAttributes(t *testing.T) {
	type fields struct {
		d *Dagger
	}
	type args struct {
		edge Edge
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name: "with additional attributes",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					from := Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					}
					to := Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					}
					d.AddNode(from)
					d.AddNode(to)
					d.AddEdge(Edge{
						from: from,
						to:   to,
						attributes: map[string]interface{}{
							"test-key": "test-value",
						},
					})
					return d
				}(),
			},
			args: args{
				edge: Edge{
					from: Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					},
					to: Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					},
				},
			},
			want: map[string]interface{}{
				ReservedAttributeEdgeType:            EdgeXTypeConnection,
				"from_" + ReservedAttributeNodeType:  NodeXTypeFromString("pod"),
				"to_" + ReservedAttributeNodeType:    NodeXTypeFromString("service"),
				"from_" + ReservedAttributeName:      "test-pod",
				"from_" + ReservedAttributeNamespace: "test-namespace",
				"from_" + ReservedAttributeXID:       generateNodeXID("test-pod", "test-namespace"),
				"to_" + ReservedAttributeName:        "test-service",
				"to_" + ReservedAttributeNamespace:   "test-namespace",
				"to_" + ReservedAttributeXID:         generateNodeXID("test-service", "test-namespace"),
				"test-key":                           "test-value",
			},
			wantErr: false,
		},
		{
			name: "with nil attributes",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					from := Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					}
					to := Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					}
					d.AddNode(from)
					d.AddNode(to)
					d.AddEdge(Edge{
						from:       from,
						to:         to,
						attributes: nil,
					})
					return d
				}(),
			},
			args: args{
				edge: Edge{
					from: Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					},
					to: Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					},
				},
			},
			want: map[string]interface{}{
				ReservedAttributeEdgeType:            EdgeXTypeConnection,
				"from_" + ReservedAttributeNodeType:  NodeXTypeFromString("pod"),
				"to_" + ReservedAttributeNodeType:    NodeXTypeFromString("service"),
				"from_" + ReservedAttributeName:      "test-pod",
				"from_" + ReservedAttributeNamespace: "test-namespace",
				"from_" + ReservedAttributeXID:       generateNodeXID("test-pod", "test-namespace"),
				"to_" + ReservedAttributeName:        "test-service",
				"to_" + ReservedAttributeNamespace:   "test-namespace",
				"to_" + ReservedAttributeXID:         generateNodeXID("test-service", "test-namespace"),
			},
			wantErr: false,
		},
		{
			name: "edge does not exist",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					from := Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					}
					to := Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					}
					d.AddNode(from)
					d.AddNode(to)
					return d
				}(),
			},
			args: args{
				edge: Edge{
					from: Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					},
					to: Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fields.d.GetEdgeAttributes(tt.args.edge)
			if (err != nil) != tt.wantErr {
				t.Errorf("Dagger.GetEdgeAttributes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Dagger.GetEdgeAttributes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDagger_SetEdgeAttributes(t *testing.T) {
	type fields struct {
		d *Dagger
	}
	type args struct {
		edge       Edge
		attributes map[string]interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name: "add multiple attributes",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					from := Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					}
					to := Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					}
					d.AddNode(from)
					d.AddNode(to)
					d.AddEdge(Edge{
						from:       from,
						to:         to,
						attributes: nil,
					})
					return d
				}(),
			},
			args: args{
				edge: Edge{
					from: Node{
						name:      "test-pod",
						namespace: "test-namespace",
						objType:   "pod",
					},
					to: Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					},
				},
				attributes: map[string]interface{}{
					"test-key":  "test-value",
					"test-key2": "test-value2",
				},
			},
			want: map[string]interface{}{
				ReservedAttributeEdgeType:            EdgeXTypeConnection,
				"from_" + ReservedAttributeNodeType:  NodeXTypeFromString("pod"),
				"to_" + ReservedAttributeNodeType:    NodeXTypeFromString("service"),
				"from_" + ReservedAttributeName:      "test-pod",
				"from_" + ReservedAttributeNamespace: "test-namespace",
				"from_" + ReservedAttributeXID:       generateNodeXID("test-pod", "test-namespace"),
				"to_" + ReservedAttributeName:        "test-service",
				"to_" + ReservedAttributeNamespace:   "test-namespace",
				"to_" + ReservedAttributeXID:         generateNodeXID("test-service", "test-namespace"),
				"test-key":                           "test-value",
				"test-key2":                          "test-value2",
			},
			wantErr: false,
		},
		{
			name: "edge does not exist",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					from := Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					}
					to := Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					}
					d.AddNode(from)
					d.AddNode(to)
					return d
				}(),
			},
			args: args{
				edge: Edge{
					from: Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					},
					to: Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "add nil attributes",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					from := Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					}
					to := Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					}
					d.AddNode(from)
					d.AddNode(to)
					d.AddEdge(Edge{
						from:       from,
						to:         to,
						attributes: nil,
					})
					return d
				}(),
			},
			args: args{
				edge: Edge{
					from: Node{
						name:      "test-pod",
						namespace: "test-namespace",
						objType:   "pod",
					},
					to: Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					},
				},
				attributes: nil,
			},
			want: map[string]interface{}{
				ReservedAttributeEdgeType:            EdgeXTypeConnection,
				"from_" + ReservedAttributeNodeType:  NodeXTypeFromString("pod"),
				"to_" + ReservedAttributeNodeType:    NodeXTypeFromString("service"),
				"from_" + ReservedAttributeName:      "test-pod",
				"from_" + ReservedAttributeNamespace: "test-namespace",
				"from_" + ReservedAttributeXID:       generateNodeXID("test-pod", "test-namespace"),
				"to_" + ReservedAttributeName:        "test-service",
				"to_" + ReservedAttributeNamespace:   "test-namespace",
				"to_" + ReservedAttributeXID:         generateNodeXID("test-service", "test-namespace"),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fields.d.SetEdgeAttributes(tt.args.edge, tt.args.attributes)
			if (err != nil) != tt.wantErr {
				t.Errorf("Dagger.SetEdgeAttributes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Dagger.SetEdgeAttributes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDagger_SetEdgeAttribute(t *testing.T) {
	type fields struct {
		d *Dagger
	}
	type args struct {
		edge  Edge
		key   string
		value interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name: "add new attributes",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					from := Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					}
					to := Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					}
					d.AddNode(from)
					d.AddNode(to)
					d.AddEdge(Edge{
						from:       from,
						to:         to,
						attributes: nil,
					})
					return d
				}(),
			},
			args: args{
				edge: Edge{
					from: Node{
						name:      "test-pod",
						namespace: "test-namespace",
						objType:   "pod",
					},
					to: Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					},
				},
				key:   "test-key",
				value: "test-value",
			},
			want: map[string]interface{}{
				ReservedAttributeEdgeType:            EdgeXTypeConnection,
				"from_" + ReservedAttributeNodeType:  NodeXTypeFromString("pod"),
				"to_" + ReservedAttributeNodeType:    NodeXTypeFromString("service"),
				"from_" + ReservedAttributeName:      "test-pod",
				"from_" + ReservedAttributeNamespace: "test-namespace",
				"from_" + ReservedAttributeXID:       generateNodeXID("test-pod", "test-namespace"),
				"to_" + ReservedAttributeName:        "test-service",
				"to_" + ReservedAttributeNamespace:   "test-namespace",
				"to_" + ReservedAttributeXID:         generateNodeXID("test-service", "test-namespace"),
				"test-key":                           "test-value",
			},
			wantErr: false,
		},
		{
			name: "edge does not exist",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					from := Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					}
					to := Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					}
					d.AddNode(from)
					d.AddNode(to)
					return d
				}(),
			},
			args: args{
				edge: Edge{
					from: Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					},
					to: Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					},
				},
				key:   "test-key",
				value: "test-value",
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fields.d.SetEdgeAttribute(tt.args.edge, tt.args.key, tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Dagger.SetEdgeAttribute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Dagger.SetEdgeAttribute() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDagger_DelEdgeAttribute(t *testing.T) {
	type fields struct {
		d *Dagger
	}
	type args struct {
		edge Edge
		key  string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name: "delete existing attributes",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					from := Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					}
					to := Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					}
					d.AddNode(from)
					d.AddNode(to)
					d.AddEdge(Edge{
						from: from,
						to:   to,
						attributes: map[string]interface{}{
							"test-key":  "test-value",
							"test-key2": "test-value2",
						},
					})
					return d
				}(),
			},
			args: args{
				edge: Edge{
					from: Node{
						name:      "test-pod",
						namespace: "test-namespace",
						objType:   "pod",
					},
					to: Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					},
				},
				key: "test-key",
			},
			want: map[string]interface{}{
				ReservedAttributeEdgeType:            EdgeXTypeConnection,
				"from_" + ReservedAttributeNodeType:  NodeXTypeFromString("pod"),
				"to_" + ReservedAttributeNodeType:    NodeXTypeFromString("service"),
				"from_" + ReservedAttributeName:      "test-pod",
				"from_" + ReservedAttributeNamespace: "test-namespace",
				"from_" + ReservedAttributeXID:       generateNodeXID("test-pod", "test-namespace"),
				"to_" + ReservedAttributeName:        "test-service",
				"to_" + ReservedAttributeNamespace:   "test-namespace",
				"to_" + ReservedAttributeXID:         generateNodeXID("test-service", "test-namespace"),
				"test-key2":                          "test-value2",
			},
			wantErr: false,
		},
		{
			name: "delete missing attributes",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					from := Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					}
					to := Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					}
					d.AddNode(from)
					d.AddNode(to)
					d.AddEdge(Edge{
						from: from,
						to:   to,
						attributes: map[string]interface{}{
							"test-key2": "test-value2",
						},
					})
					return d
				}(),
			},
			args: args{
				edge: Edge{
					from: Node{
						name:      "test-pod",
						namespace: "test-namespace",
						objType:   "pod",
					},
					to: Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					},
				},
				key: "test-key",
			},
			want: map[string]interface{}{
				ReservedAttributeEdgeType:            EdgeXTypeConnection,
				"from_" + ReservedAttributeNodeType:  NodeXTypeFromString("pod"),
				"to_" + ReservedAttributeNodeType:    NodeXTypeFromString("service"),
				"from_" + ReservedAttributeName:      "test-pod",
				"from_" + ReservedAttributeNamespace: "test-namespace",
				"from_" + ReservedAttributeXID:       generateNodeXID("test-pod", "test-namespace"),
				"to_" + ReservedAttributeName:        "test-service",
				"to_" + ReservedAttributeNamespace:   "test-namespace",
				"to_" + ReservedAttributeXID:         generateNodeXID("test-service", "test-namespace"),
				"test-key2":                          "test-value2",
			},
			wantErr: false,
		},
		{
			name: "edge does not exist",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					from := Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					}
					to := Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					}
					d.AddNode(from)
					d.AddNode(to)
					return d
				}(),
			},
			args: args{
				edge: Edge{
					from: Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					},
					to: Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					},
				},
				key: "test-key",
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fields.d.DelEdgeAttribute(tt.args.edge, tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("Dagger.DelEdgeAttribute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Dagger.DelEdgeAttribute() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDagger_ListOutboundEdges(t *testing.T) {
	type fields struct {
		d *Dagger
	}
	type args struct {
		node Node
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []Edge
		wantErr bool
	}{
		{
			name: "list two outbound edges",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					from := Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					}
					toSvc := Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					}
					toPod := Node{
						name:       "test-pod2",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					}
					d.AddNode(from)
					d.AddNode(toSvc)
					d.AddNode(toPod)

					d.AddEdge(Edge{
						from: from,
						to:   toSvc,
						attributes: map[string]interface{}{
							"test-key": "test-value",
						},
					})
					d.AddEdge(Edge{
						from: from,
						to:   toPod,
						attributes: map[string]interface{}{
							"test-key": "test-value",
						},
					})
					return d
				}(),
			},
			args: args{
				node: Node{
					name:      "test-pod",
					namespace: "test-namespace",
					objType:   "pod",
				},
			},
			want: []Edge{
				{
					from: Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					},
					to: Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					},
					attributes: map[string]interface{}{
						ReservedAttributeEdgeType:            EdgeXTypeConnection,
						"from_" + ReservedAttributeNodeType:  NodeXTypeFromString("pod"),
						"to_" + ReservedAttributeNodeType:    NodeXTypeFromString("service"),
						"from_" + ReservedAttributeName:      "test-pod",
						"from_" + ReservedAttributeNamespace: "test-namespace",
						"from_" + ReservedAttributeXID:       generateNodeXID("test-pod", "test-namespace"),
						"to_" + ReservedAttributeName:        "test-service",
						"to_" + ReservedAttributeNamespace:   "test-namespace",
						"to_" + ReservedAttributeXID:         generateNodeXID("test-service", "test-namespace"),
						"test-key":                           "test-value",
					},
				},
				{
					from: Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					},
					to: Node{
						name:       "test-pod2",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					},
					attributes: map[string]interface{}{
						ReservedAttributeEdgeType:            EdgeXTypeConnection,
						"from_" + ReservedAttributeNodeType:  NodeXTypeFromString("pod"),
						"to_" + ReservedAttributeNodeType:    NodeXTypeFromString("pod"),
						"from_" + ReservedAttributeName:      "test-pod",
						"from_" + ReservedAttributeNamespace: "test-namespace",
						"from_" + ReservedAttributeXID:       generateNodeXID("test-pod", "test-namespace"),
						"to_" + ReservedAttributeName:        "test-pod2",
						"to_" + ReservedAttributeNamespace:   "test-namespace",
						"to_" + ReservedAttributeXID:         generateNodeXID("test-pod2", "test-namespace"),
						"test-key":                           "test-value",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fields.d.ListOutboundEdges(tt.args.node)
			if (err != nil) != tt.wantErr {
				t.Errorf("Dagger.ListOutboundEdges() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !arrayEqualEdge(got, tt.want) {
				t.Errorf("Dagger.ListOutboundEdges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDagger_ListInboundEdges(t *testing.T) {
	type fields struct {
		d *Dagger
	}
	type args struct {
		node Node
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []Edge
		wantErr bool
	}{
		{
			name: "list two inbound edges",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					to := Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					}
					fromSvc := Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					}
					fromPod := Node{
						name:       "test-pod2",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					}
					d.AddNode(to)
					d.AddNode(fromSvc)
					d.AddNode(fromPod)

					d.AddEdge(Edge{
						from: fromSvc,
						to:   to,
						attributes: map[string]interface{}{
							"test-key": "test-value",
						},
					})
					d.AddEdge(Edge{
						from: fromPod,
						to:   to,
						attributes: map[string]interface{}{
							"test-key": "test-value",
						},
					})
					return d
				}(),
			},
			args: args{
				node: Node{
					name:      "test-pod",
					namespace: "test-namespace",
					objType:   "pod",
				},
			},
			want: []Edge{
				{
					to: Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					},
					from: Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					},
					attributes: map[string]interface{}{
						ReservedAttributeEdgeType:            EdgeXTypeConnection,
						"to_" + ReservedAttributeNodeType:    NodeXTypeFromString("pod"),
						"from_" + ReservedAttributeNodeType:  NodeXTypeFromString("service"),
						"to_" + ReservedAttributeName:        "test-pod",
						"to_" + ReservedAttributeNamespace:   "test-namespace",
						"to_" + ReservedAttributeXID:         generateNodeXID("test-pod", "test-namespace"),
						"from_" + ReservedAttributeName:      "test-service",
						"from_" + ReservedAttributeNamespace: "test-namespace",
						"from_" + ReservedAttributeXID:       generateNodeXID("test-service", "test-namespace"),
						"test-key":                           "test-value",
					},
				},
				{
					to: Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					},
					from: Node{
						name:       "test-pod2",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					},
					attributes: map[string]interface{}{
						ReservedAttributeEdgeType:            EdgeXTypeConnection,
						"to_" + ReservedAttributeNodeType:    NodeXTypeFromString("pod"),
						"from_" + ReservedAttributeNodeType:  NodeXTypeFromString("pod"),
						"to_" + ReservedAttributeName:        "test-pod",
						"to_" + ReservedAttributeNamespace:   "test-namespace",
						"to_" + ReservedAttributeXID:         generateNodeXID("test-pod", "test-namespace"),
						"from_" + ReservedAttributeName:      "test-pod2",
						"from_" + ReservedAttributeNamespace: "test-namespace",
						"from_" + ReservedAttributeXID:       generateNodeXID("test-pod2", "test-namespace"),
						"test-key":                           "test-value",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fields.d.ListInboundEdges(tt.args.node)
			if (err != nil) != tt.wantErr {
				t.Errorf("Dagger.ListInboundEdges() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !arrayEqualEdge(got, tt.want) {
				t.Errorf("Dagger.ListInboundEdges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDagger_ListNodes(t *testing.T) {
	type fields struct {
		d *Dagger
	}
	tests := []struct {
		name    string
		fields  fields
		want    []Node
		wantErr bool
	}{
		{
			name: "list all graph nodes",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					to := Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					}
					fromSvc := Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					}
					fromPod := Node{
						name:       "test-pod2",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					}
					d.AddNode(to)
					d.AddNode(fromSvc)
					d.AddNode(fromPod)

					d.AddEdge(Edge{
						from: fromSvc,
						to:   to,
						attributes: map[string]interface{}{
							"test-key": "test-value",
						},
					})
					d.AddEdge(Edge{
						from: fromPod,
						to:   to,
						attributes: map[string]interface{}{
							"test-key": "test-value",
						},
					})
					return d
				}(),
			},
			want: []Node{
				{
					name:       "test-pod",
					namespace:  "test-namespace",
					objType:    "pod",
					attributes: nil,
				},
				{
					name:       "test-pod2",
					namespace:  "test-namespace",
					objType:    "pod",
					attributes: nil,
				},
				{
					name:       "test-service",
					namespace:  "test-namespace",
					objType:    "service",
					attributes: nil,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fields.d.ListNodes()
			if (err != nil) != tt.wantErr {
				t.Errorf("Dagger.ListNodes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !arrayEqualNode(got, tt.want) {
				t.Errorf("Dagger.ListNodes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDagger_ListNeighbors(t *testing.T) {
	type fields struct {
		d *Dagger
	}
	type args struct {
		node Node
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []Node
		wantErr bool
	}{
		{
			name: "list two neighbor nodes",
			fields: fields{
				d: func() *Dagger {
					d := NewDaggerStore()
					from := Node{
						name:       "test-pod",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					}
					toSvc := Node{
						name:       "test-service",
						namespace:  "test-namespace",
						objType:    "service",
						attributes: nil,
					}
					toPod := Node{
						name:       "test-pod2",
						namespace:  "test-namespace",
						objType:    "pod",
						attributes: nil,
					}
					d.AddNode(from)
					d.AddNode(toSvc)
					d.AddNode(toPod)

					d.AddEdge(Edge{
						from: from,
						to:   toSvc,
						attributes: map[string]interface{}{
							"test-key": "test-value",
						},
					})
					d.AddEdge(Edge{
						from: from,
						to:   toPod,
						attributes: map[string]interface{}{
							"test-key": "test-value",
						},
					})
					return d
				}(),
			},
			args: args{
				node: Node{
					name:      "test-pod",
					namespace: "test-namespace",
					objType:   "pod",
				},
			},
			want: []Node{
				{
					name:       "test-service",
					namespace:  "test-namespace",
					objType:    "service",
					attributes: nil,
				},
				{
					name:       "test-pod2",
					namespace:  "test-namespace",
					objType:    "pod",
					attributes: nil,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fields.d.ListNeighbors(tt.args.node)
			if (err != nil) != tt.wantErr {
				t.Errorf("Dagger.ListNeighbors() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !arrayEqualNode(got, tt.want) {
				t.Errorf("Dagger.ListNeighbors() = %v, want %v", got, tt.want)
			}
		})
	}
}
