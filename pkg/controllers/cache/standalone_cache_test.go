// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package cache

import (
	"testing"

	"github.com/microsoft/retina/pkg/log"
	"gotest.tools/v3/assert"
)

func TestStandaloneCache(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	cache := NewStandaloneCache()

	ip := "10.0.0.1"
	name := "standalone-pod"
	namespace := "standalone-namespace"

	// Add pod information
	cache.AddPod(ip, name, namespace)
	podInfo := cache.GetPod(ip)

	if podInfo == nil {
		t.Fatalf("Expected pod info, got nil")
	}
	assert.Equal(t, podInfo.Name, name)
	assert.Equal(t, podInfo.Namespace, namespace)

	// Delete pod information
	cache.DeletePod(ip)
	deletedPodInfo := cache.GetPod(ip)
	if deletedPodInfo != nil {
		t.Fatalf("Expected nil, got %v", deletedPodInfo)
	}
}
