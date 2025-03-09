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
	name      = "standalone-pod"
	namespace = "standalone-namespace"
)

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

	select {
	case event := <-cache.WatchEvents():
		assert.Equal(t, event.Ip, ip)
		assert.Equal(t, event.PodInfo.Name, name)
		assert.Equal(t, event.PodInfo.Namespace, namespace)
		assert.Equal(t, event.Action, EventAdd)
	case <-time.After(time.Second):
		t.Fatalf("Expected an AddPod event, but none received")
	}
}

func TestDeletePod(t *testing.T) {
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Fatalf("Failed to setup logger: %v", err)
	}
	cache := NewStandaloneCache()

	cache.DeletePod(ip)
	select {
	case event := <-cache.WatchEvents():
		t.Fatalf("Expected no events, but got %+v", event)
	case <-time.After(100 * time.Millisecond):
	}

	cache.AddPod(ip, name, namespace)
	cache.DeletePod(ip)
	deletedPodInfo := cache.GetPod(ip)
	if deletedPodInfo != nil {
		t.Fatalf("Expected nil, got %v", deletedPodInfo)
	}

	for {
		select {
		case event := <-cache.WatchEvents():
			if event.Action == EventDelete {
				assert.Equal(t, event.Ip, ip)
				assert.Equal(t, event.Action, EventDelete)
				return
			}
		case <-time.After(time.Second):
			t.Fatalf("Expected a DeletePod event, but none received")
		}
	}
}
