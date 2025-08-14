// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package cache

import (
	"testing"
	"time"

	"github.com/microsoft/retina/pkg/log"
	"github.com/stretchr/testify/require"
)

var (
	ip  = "10.0.0.1"
	p1  = &PodInfo{Name: "pod1", Namespace: "ns1"}
	ttl = 3 * time.Minute
)

func TestCacheAddPod(t *testing.T) {
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Errorf("Error setting up logger: %s", err)
	}
	c := NewStandaloneCache(ttl)

	p2 := &PodInfo{Name: "pod2", Namespace: "ns2"}
	p3 := &PodInfo{Name: "pod1", Namespace: "ns1"}

	tests := []struct {
		name        string
		ip          string
		pod         string
		namespace   string
		expectedPod string
		expectedNS  string
	}{
		{
			name:        "Add new pod",
			ip:          ip,
			pod:         p1.Name,
			namespace:   p1.Namespace,
			expectedPod: p1.Name,
			expectedNS:  p1.Namespace,
		},
		{
			name:        "Add identical pod",
			ip:          ip,
			pod:         p3.Name,
			namespace:   p3.Namespace,
			expectedPod: p1.Name,
			expectedNS:  p1.Namespace,
		},
		{
			name:        "Update pod info for same IP",
			ip:          ip,
			pod:         p2.Name,
			namespace:   p2.Namespace,
			expectedPod: p2.Name,
			expectedNS:  p2.Namespace,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.addPod(tt.ip, tt.pod, tt.namespace)

			got := c.GetPod(tt.ip)
			require.NotNil(t, got, "Expected pod info, got nil")
			require.Equal(t, tt.expectedPod, got.Name)
			require.Equal(t, tt.expectedNS, got.Namespace)
		})
	}
}

func TestCacheDeletePod(t *testing.T) {
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Errorf("Error setting up logger: %s", err)
	}
	c := NewStandaloneCache(ttl)

	tests := []struct {
		name            string
		setup           func()
		ip              string
		expectedPodInfo *PodInfo
	}{
		{
			name: "Delete existing pod",
			setup: func() {
				c.addPod(ip, p1.Name, p1.Namespace)
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
			c.deletePod(tt.ip)

			got := c.GetPod(tt.ip)
			require.Equal(t, tt.expectedPodInfo, got)
		})
	}
}

func TestCacheUpdate(t *testing.T) {
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Errorf("Error setting up logger: %s", err)
	}
	c := NewStandaloneCache(ttl)

	tests := []struct {
		name            string
		ip              string
		podInfo         *PodInfo
		expectedPodInfo *PodInfo
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
				require.Nil(t, got, "Expected nil pod info, got %v", got)
			} else {
				require.NotNil(t, got != nil, "Expected pod info, got nil")
				require.Equal(t, tt.expectedPodInfo.Name, got.Name)
				require.Equal(t, tt.expectedPodInfo.Namespace, got.Namespace)
			}
		})
	}
}

func TestCacheTTL(t *testing.T) {
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Errorf("Error setting up logger: %s", err)
	}
	c := NewStandaloneCache(ttl)

	cacheTime := c.TTL()
	require.Equal(t, cacheTime, ttl)
}
