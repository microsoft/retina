// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package cache_test

import (
	"testing"

	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/log"
	"gotest.tools/v3/assert"
)

const (
	ip        = "10.0.0.1"
	name      = "test-pod"
	namespace = "test-ns"
)

var defaultInfo = &cache.PodInfo{
	Name:      name,
	Namespace: namespace,
}

func TestStandaloneCache(t *testing.T) {
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Fatalf("Failed to setup logger: %v", err)
	}
	testCache := cache.NewStandaloneCache()

	tests := []struct {
		name   string
		setup  func()
		expect func(t *testing.T)
	}{
		{
			name: "Add Pod - New pod info added to cache",
			setup: func() {
				testCache.Update(ip, defaultInfo)
			},
			expect: func(t *testing.T) {
				podInfo := testCache.GetPod(ip)
				if podInfo == nil {
					t.Fatalf("Expected pod info, got nil")
				}
				assert.Equal(t, podInfo.Name, name)
				assert.Equal(t, podInfo.Namespace, namespace)
				assert.Equal(t, podInfo.Active, true)
			},
		},
		{
			name: "Add pod - Pod info updated if not identical",
			setup: func() {
				testCache.Update(ip, &cache.PodInfo{Name: "updated-pod", Namespace: "updated-ns"})
			},
			expect: func(t *testing.T) {
				podInfo := testCache.GetPod(ip)
				if podInfo == nil {
					t.Fatalf("Expected pod info, got nil")
				}
				assert.Equal(t, podInfo.Name, "updated-pod")
				assert.Equal(t, podInfo.Namespace, "updated-ns")
				assert.Equal(t, podInfo.Active, true)
			},
		},
		{
			name: "Delete pod - Pod info deleted from cache",
			setup: func() {
				testCache.Update(ip, nil)
			},
			expect: func(t *testing.T) {
				podInfo := testCache.GetPod(ip)
				if podInfo != nil {
					t.Fatalf("Expected nil, got %v", podInfo)
				}
			},
		},
		{
			name: "Reset IP Statuses - Pods added before should be marked inactive",
			setup: func() {
				testCache.Update(ip, defaultInfo)
				testCache.ResetIPStatuses()
				testCache.Update("ip-2", &cache.PodInfo{Name: "pod-2", Namespace: "ns-2"})
			},
			expect: func(t *testing.T) {
				podInfo1 := testCache.GetPod(ip)
				if podInfo1 == nil {
					t.Fatalf("Expected pod info, got nil")
				}
				assert.Equal(t, podInfo1.Active, false)

				podInfo2 := testCache.GetPod("ip-2")
				if podInfo2 == nil {
					t.Fatalf("Expected pod info, got nil")
				}
				assert.Equal(t, podInfo2.Active, true)
			},
		},
		{
			name: "Remove Stale Entries - Stale pods removed from cache",
			setup: func() {
				testCache.Update(ip, defaultInfo)
				testCache.Update("ip-2", &cache.PodInfo{Name: "pod-2", Namespace: "ns-2"})
				testCache.ResetIPStatuses()
				testCache.Update(ip, defaultInfo)
				testCache.RemoveStaleEntries()
			},
			expect: func(t *testing.T) {
				podInfo := testCache.GetPod(ip)
				if podInfo == nil {
					t.Fatalf("Expected pod info, got nil")
				}
				assert.Equal(t, podInfo.Name, defaultInfo.Name)
				assert.Equal(t, podInfo.Namespace, defaultInfo.Namespace)
				assert.Equal(t, podInfo.Active, true)

				removedPod := testCache.GetPod("ip-2")
				if removedPod != nil {
					t.Fatalf("Expected nil, got %v", removedPod)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCache = cache.NewStandaloneCache()
			tt.setup()
			tt.expect(t)
		})
	}
}

// Split up these test into their own units - later
