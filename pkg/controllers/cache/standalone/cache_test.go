// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package standalone

import (
	"net"
	"testing"

	"github.com/microsoft/retina/pkg/common"
	"github.com/microsoft/retina/pkg/log"
	"github.com/stretchr/testify/require"
)

var (
	ep1 = common.NewRetinaEndpoint("pod1", "ns1", &common.IPAddresses{IPv4: net.ParseIP("10.0.0.1")})
	ep2 = common.NewRetinaEndpoint("pod2", "ns2", &common.IPAddresses{IPv4: net.ParseIP("10.0.0.1")})
	ep3 = common.NewRetinaEndpoint("pod1", "ns1", &common.IPAddresses{IPv4: net.ParseIP("10.0.0.1")})
)

func TestCacheAddEndpoint(t *testing.T) {
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Errorf("Error setting up logger: %s", err)
	}
	c := NewCache()

	tests := []struct {
		name        string
		endpoint    *common.RetinaEndpoint
		expectedPod string
		expectedNS  string
	}{
		{
			name:        "Add new endpoint",
			endpoint:    ep1,
			expectedPod: ep1.Name(),
			expectedNS:  ep1.Namespace(),
		},
		{
			name:        "Add identical endpoint",
			endpoint:    ep3,
			expectedPod: ep1.Name(),
			expectedNS:  ep1.Namespace(),
		},
		{
			name:        "Update endpoint info for same IP",
			endpoint:    ep2,
			expectedPod: ep2.Name(),
			expectedNS:  ep2.Namespace(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.UpdateRetinaEndpoint(tt.endpoint)

			ip, err := tt.endpoint.PrimaryIP()
			require.NoError(t, err)

			got := c.GetPodByIP(ip)
			require.NotNil(t, got, "Expected retina endpoint, got nil")
			require.Equal(t, tt.expectedPod, got.Name())
			require.Equal(t, tt.expectedNS, got.Namespace())
		})
	}
}

func TestCacheDeleteEndpoint(t *testing.T) {
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Errorf("Error setting up logger: %s", err)
	}
	c := NewCache()
	ip, err := ep1.PrimaryIP()
	if err != nil {
		t.Fatalf("failed to get IP for endpoint: %v", err)
	}

	tests := []struct {
		name             string
		setup            func()
		ip               string
		expectedEndpoint *common.RetinaEndpoint
	}{
		{
			name: "Delete existing endpoint",
			setup: func() {
				c.UpdateRetinaEndpoint(ep1)
			},
			ip:               ip,
			expectedEndpoint: nil,
		},
		{
			name:             "Delete non-existing pod (no-op)",
			setup:            func() {},
			ip:               "10.0.0.2",
			expectedEndpoint: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			c.DeleteRetinaEndpoint(tt.ip)

			got := c.GetPodByIP(tt.ip)
			require.Equal(t, tt.expectedEndpoint, got)
		})
	}
}

func TestCacheGetAllIPs(t *testing.T) {
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Errorf("Error setting up logger: %s", err)
	}
	c := NewCache()
	ep4 := common.NewRetinaEndpoint("pod4", "ns4", &common.IPAddresses{IPv4: net.ParseIP("10.0.0.4")})

	tests := []struct {
		name    string
		actions func()
		wantIPs []string
	}{
		{
			name: "Add ep1 and ep2",
			actions: func() {
				_ = c.UpdateRetinaEndpoint(ep1)
				_ = c.UpdateRetinaEndpoint(ep2)
			},
			wantIPs: []string{"10.0.0.1"},
		},
		{
			name: "Add ep4",
			actions: func() {
				_ = c.UpdateRetinaEndpoint(ep4)
			},
			wantIPs: []string{"10.0.0.1", "10.0.0.4"},
		},
		{
			name: "Delete ep1",
			actions: func() {
				ip1, _ := ep1.PrimaryIP()
				_ = c.DeleteRetinaEndpoint(ip1)
			},
			wantIPs: []string{"10.0.0.4"},
		},
		{
			name: "Delete ep4",
			actions: func() {
				ip4, _ := ep4.PrimaryIP()
				_ = c.DeleteRetinaEndpoint(ip4)
			},
			wantIPs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.actions()
			ips := c.GetAllIPs()
			require.ElementsMatch(t, tt.wantIPs, ips, "IPs mismatch for test: %s", tt.name)
		})
	}
}
