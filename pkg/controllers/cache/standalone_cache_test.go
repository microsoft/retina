// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package cache

import (
	"testing"
	"time"

	"github.com/microsoft/retina/pkg/log"
	"gotest.tools/v3/assert"
)

const (
	ip        = "10.0.0.1"
	name      = "test-pod"
	namespace = "test-namespace"
)

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
	if cache.eventChannel == nil {
		t.Fatalf("Expected non-nil eventChannel, got nil")
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

	cache.AddPod(ip, name, namespace)
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

	cache.DeletePod(ip)
	emptyPodInfo := cache.GetPod(ip)
	if emptyPodInfo != nil {
		t.Fatalf("Expected nil, got %v", emptyPodInfo)
	}

	cache.AddPod(ip, name, namespace)
	cache.DeletePod(ip)
	deletedPodInfo := cache.GetPod(ip)
	if deletedPodInfo != nil {
		t.Fatalf("Expected nil, got %v", deletedPodInfo)
	}
}

func TestPublishEvent(t *testing.T) {
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Fatalf("Failed to setup logger: %v", err)
	}

	MaxStandaloneCacheEventSize = 1
	cache := NewStandaloneCache()

	go func() {
		time.Sleep(100 * time.Millisecond)
		event := <-cache.WatchEvents()
		assert.Equal(t, ip, event.Ip)
		assert.Equal(t, EventAdd, event.Action)
	}()

	err := cache.PublishEvent(ip, EventAdd)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	err = cache.PublishEvent(ip, EventDelete)
	assert.Equal(t, err, ErrEventChannelFull)
}
