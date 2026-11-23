// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package filtermanager

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitReturnsSharedFilterManager(t *testing.T) {
	first, err := Init(1, 1)
	require.NoError(t, err)
	require.NotNil(t, first)

	second, err := Init(2, 2)
	require.NoError(t, err)
	assert.Same(t, first, second)
}

func TestFilterManagerRetainsRequests(t *testing.T) {
	manager := &FilterManager{
		c: &filterCache{data: make(map[string]requests)},
	}
	ip := net.ParseIP("10.0.0.1")
	firstRequestor := Requestor("first")
	secondRequestor := Requestor("second")
	metadata := RequestMetadata{RuleID: "rule"}

	require.NoError(t, manager.AddIPs([]net.IP{ip}, firstRequestor, metadata))
	require.NoError(t, manager.AddIPs([]net.IP{ip}, secondRequestor, metadata))
	assert.True(t, manager.HasIP(ip))

	require.NoError(t, manager.DeleteIPs([]net.IP{ip}, firstRequestor, metadata))
	assert.True(t, manager.HasIP(ip))

	require.NoError(t, manager.DeleteIPs([]net.IP{ip}, secondRequestor, metadata))
	assert.False(t, manager.HasIP(ip))
}

func TestFilterManagerResetAndStop(t *testing.T) {
	tests := []struct {
		name  string
		reset func(*FilterManager) error
	}{
		{
			name:  "reset",
			reset: (*FilterManager).Reset,
		},
		{
			name:  "stop",
			reset: (*FilterManager).Stop,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &FilterManager{
				c: &filterCache{data: make(map[string]requests)},
			}
			ips := []net.IP{
				net.ParseIP("10.0.0.1"),
				net.ParseIP("10.0.0.2"),
			}

			require.NoError(t, manager.AddIPs(ips, "requestor", RequestMetadata{RuleID: "rule"}))
			require.NoError(t, tt.reset(manager))

			assert.False(t, manager.HasIP(ips[0]))
			assert.False(t, manager.HasIP(ips[1]))
		})
	}
}
