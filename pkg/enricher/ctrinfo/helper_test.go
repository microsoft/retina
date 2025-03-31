// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package ctrinfo

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJSONUnmarshalling(t *testing.T) {
	data, err := os.ReadFile("mock_podSpec.json")
	if err != nil {
		t.Fatalf("Failed to open test file: %v", err)
	}

	var spec PodSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		fmt.Println("Error decoding file:", err)
		assert.Error(t, err, "expected error when using invalid file path")
	}

	assert.Equal(t, "retina-pod", spec.Status.MetaData.Name)
	assert.Equal(t, "retina-namespace", spec.Status.MetaData.Namespace)
	assert.Equal(t, "10.0.0.4", spec.Status.Network.IP)
}
