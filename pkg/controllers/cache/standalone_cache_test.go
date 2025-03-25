// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package cache

import (
	"testing"

	"github.com/microsoft/retina/pkg/log"
	"gotest.tools/v3/assert"
)

const (
	ip        = "10.0.0.1"
	name      = "test-pod"
	namespace = "test-namespace"
)

var defaultInfo = &PodInfo{
	Name:      name,
	Namespace: namespace,
}

func TestNewStandaloneCache(t *testing.T) {
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Fatalf("Failed to setup logger: %v", err)
	}
	cache := NewStandaloneCache()
	if cache == nil {
		t.Fatalf("Expected non-nil cache, got nil")
	}
	if cache.ipToPod == nil {
		t.Fatalf("Expected non-nil ipToPod map, got nil")
	}
}

func TestAddPod(t *testing.T) {
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Fatalf("Failed to setup logger: %v", err)
	}
	cache := NewStandaloneCache()

	emptyPodInfo := cache.GetPod(ip)
	if emptyPodInfo != nil {
		t.Fatalf("Expected nil, got %v", emptyPodInfo)
	}

	cache.ProcessPodInfo(ip, defaultInfo)
	podInfo := cache.GetPod(ip)

	if podInfo == nil {
		t.Fatalf("Expected pod info, got nil")
	}
	assert.Equal(t, podInfo.Name, name)
	assert.Equal(t, podInfo.Namespace, namespace)
}

func TestDeletePod(t *testing.T) {
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Fatalf("Failed to setup logger: %v", err)
	}
	cache := NewStandaloneCache()

	// Add pod
	cache.ProcessPodInfo(ip, defaultInfo)
	podInfo := cache.GetPod(ip)
	if podInfo == nil {
		t.Fatalf("Expected pod info, got nil")
	}

	// Attempt to delete pod not in cache
	cache.ProcessPodInfo("9.9.9.9", nil)
	podInfo1 := cache.GetPod(ip)
	if podInfo1 == nil {
		t.Fatalf("Expected pod info, got nil")
	}

	// Delete pod
	cache.ProcessPodInfo(ip, nil)
	deletedPodInfo := cache.GetPod(ip)
	if deletedPodInfo != nil {
		t.Fatalf("Expected nil, got %v", deletedPodInfo)
	}
}
