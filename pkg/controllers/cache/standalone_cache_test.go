// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package cache_test

import (
	"testing"
	"time"

	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/log"
	"gotest.tools/v3/assert"
)

var (
	ip = "10.0.0.1"
	p1 = &cache.PodInfo{Name: "pod1", Namespace: "ns1"}
)

func TestCacheAddPod(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	c := cache.NewStandaloneCache()

	p2 := &cache.PodInfo{Name: "pod2", Namespace: "ns2"}
	p3 := &cache.PodInfo{Name: "pod1", Namespace: "ns1"}

	tests := []struct {
		name        string
		ip          string
		podInfo     *cache.PodInfo
		expectedPod string
		expectedNS  string
	}{
		{
			name:        "Add new pod",
			ip:          ip,
			podInfo:     p1,
			expectedPod: p1.Name,
			expectedNS:  p1.Namespace,
		},
		{
			name:        "Add identical pod",
			ip:          ip,
			podInfo:     p3,
			expectedPod: p1.Name,
			expectedNS:  p1.Namespace,
		},
		{
			name:        "Update pod info for same IP",
			ip:          ip,
			podInfo:     p2,
			expectedPod: p2.Name,
			expectedNS:  p2.Namespace,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.Update(tt.ip, tt.podInfo)

			got := c.GetPod(tt.ip)
			assert.Assert(t, got != nil, "Expected pod info, got nil")
			assert.Equal(t, got.Name, tt.expectedPod)
			assert.Equal(t, got.Namespace, tt.expectedNS)
		})
	}
}

func TestCacheDeletePod(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	c := cache.NewStandaloneCache()

	tests := []struct {
		name            string
		setup           func()
		ip              string
		expectedPodInfo *cache.PodInfo
	}{
		{
			name: "Delete existing pod",
			setup: func() {
				c.Update(ip, p1)
			},
			ip:              ip,
			expectedPodInfo: nil,
		},
		{
			name:            "Delete non-existing pod (no-op)",
			setup:           func() {},
			ip:              "10.0.0.2",
			expectedPodInfo: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			c.Update(tt.ip, nil) // Delete the pod

			got := c.GetPod(tt.ip)
			assert.Equal(t, got, tt.expectedPodInfo)
		})
	}
}

func TestCacheUpdate(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	c := cache.NewStandaloneCache()

	tests := []struct {
		name            string
		ip              string
		podInfo         *cache.PodInfo
		expectedPodInfo *cache.PodInfo
	}{
		{
			name:            "Add Pod",
			ip:              ip,
			podInfo:         p1,
			expectedPodInfo: p1,
		},
		{
			name:            "Delete Pod",
			ip:              ip,
			podInfo:         nil,
			expectedPodInfo: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.Update(tt.ip, tt.podInfo)

			got := c.GetPod(tt.ip)
			if tt.expectedPodInfo == nil {
				assert.Assert(t, got == nil)
			} else {
				assert.Assert(t, got != nil, "Expected pod info, got nil")
				assert.Equal(t, got.Name, tt.expectedPodInfo.Name)
				assert.Equal(t, got.Namespace, tt.expectedPodInfo.Namespace)
			}
		})
	}
}

func TestCacheTTL(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	c := cache.NewStandaloneCache()

	ttl := c.TTL()
	assert.Equal(t, ttl, 3*time.Minute)
}
