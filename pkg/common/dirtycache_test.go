// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package common

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDirtyCache(t *testing.T) {
	dc := NewDirtyCache()
	assert.NotNil(t, dc)

	tests := []struct {
		name            string
		toAdd           map[string]interface{}
		toDel           map[string]interface{}
		expectedAddList []interface{}
		expectedDelList []interface{}
	}{
		{
			name:            "empty",
			toAdd:           map[string]interface{}{},
			toDel:           map[string]interface{}{},
			expectedAddList: []interface{}{},
			expectedDelList: []interface{}{},
		},
		{
			name: "add",
			toAdd: map[string]interface{}{
				"key1": "value1",
			},
			toDel: map[string]interface{}{},
			expectedAddList: []interface{}{
				"value1",
			},
			expectedDelList: []interface{}{},
		},
		{
			name:  "delete",
			toAdd: map[string]interface{}{},
			toDel: map[string]interface{}{
				"key1": "value1",
			},
			expectedAddList: []interface{}{},
			expectedDelList: []interface{}{
				"value1",
			},
		},
		{
			name: "add and delete",
			toAdd: map[string]interface{}{
				"key1": "value1",
			},
			toDel: map[string]interface{}{
				"key2": "value2",
			},
			expectedAddList: []interface{}{
				"value1",
			},
			expectedDelList: []interface{}{
				"value2",
			},
		},
		{
			name: "add and delete same key",
			toAdd: map[string]interface{}{
				"key1": "value1",
			},
			toDel: map[string]interface{}{
				"key1": "value2",
			},
			expectedAddList: []interface{}{},
			expectedDelList: []interface{}{
				"value2",
			},
		},
	}

	for _, test := range tests {
		fmt.Printf("Running test %s\n", test.name)
		for k, v := range test.toAdd {
			dc.ToAdd(k, v)
		}
		for k, v := range test.toDel {
			dc.ToDelete(k, v)
		}
		assert.Equal(t, test.expectedAddList, dc.GetAddList())
		assert.Equal(t, test.expectedDelList, dc.GetDeleteList())
		dc.ClearAdd()
		assert.Equal(t, []interface{}{}, dc.GetAddList())

		dc.ClearDelete()
		assert.Equal(t, []interface{}{}, dc.GetDeleteList())
	}
}
