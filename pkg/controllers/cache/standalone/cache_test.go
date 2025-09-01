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

	tests := []struct {
		name        string
		endpoint    *common.RetinaEndpoint
		expectedPod string
		expectedNs  string
	}{
		{
			name:        "Add new endpoint",
			endpoint:    ep1,
			expectedPod: ep1.Name(),
			expectedNs:  ep1.Namespace(),
		},
		{
			name:        "Add identical endpoint",
			endpoint:    ep3,
			expectedPod: ep1.Name(),
			expectedNs:  ep1.Namespace(),
		},
		{
			name:        "Update endpoint info for same IP",
			endpoint:    ep2,
			expectedPod: ep2.Name(),
			expectedNs:  ep2.Namespace(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New()

			err := c.UpdateRetinaEndpoint(tt.endpoint)
			require.NoError(t, err)

			ip, err := tt.endpoint.PrimaryIP()
			require.NoError(t, err)

			got := c.GetPodByIP(ip)
			require.NotNil(t, got, "Expected retina endpoint, got nil")
			require.Equal(t, tt.expectedPod, got.Name())
			require.Equal(t, tt.expectedNs, got.Namespace())
		})
	}
}

func TestCacheDeleteEndpoint(t *testing.T) {
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Errorf("Error setting up logger: %s", err)
	}

	ip1, err := ep1.PrimaryIP()
	require.NoError(t, err)

	tests := []struct {
		name             string
		add              []*common.RetinaEndpoint
		deleteIP         string
		expectedEndpoint *common.RetinaEndpoint
	}{
		{
			name:             "Delete existing endpoint",
			add:              []*common.RetinaEndpoint{ep1},
			deleteIP:         ip1,
			expectedEndpoint: nil,
		},
		{
			name:             "Delete non-existing pod (no-op)",
			add:              []*common.RetinaEndpoint{},
			deleteIP:         "10.0.0.2",
			expectedEndpoint: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New()

			for _, ep := range tt.add {
				require.NoError(t, c.UpdateRetinaEndpoint(ep))
			}
			require.NoError(t, c.DeleteRetinaEndpoint(tt.deleteIP))

			got := c.GetPodByIP(tt.deleteIP)
			require.Equal(t, tt.expectedEndpoint, got)
		})
	}
}

func TestCacheGetAllIPs(t *testing.T) {
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Errorf("Error setting up logger: %s", err)
	}
	ep4 := common.NewRetinaEndpoint("pod4", "ns4", &common.IPAddresses{IPv4: net.ParseIP("10.0.0.4")})

	tests := []struct {
		name    string
		add     []*common.RetinaEndpoint
		delete  []string
		wantIPs []string
	}{
		{
			name:    "Add two IPs",
			add:     []*common.RetinaEndpoint{ep1, ep2},
			wantIPs: []string{"10.0.0.1"},
		},
		{
			name:    "Add two unique IPs",
			add:     []*common.RetinaEndpoint{ep1, ep4},
			wantIPs: []string{"10.0.0.1", "10.0.0.4"},
		},
		{
			name:    "Add two unique IPs and delete one IP",
			add:     []*common.RetinaEndpoint{ep1, ep4},
			delete:  []string{"10.0.0.1"},
			wantIPs: []string{"10.0.0.4"},
		},
		{
			name:    "Add two unique IPs and delete two IPs",
			add:     []*common.RetinaEndpoint{ep1, ep4},
			delete:  []string{"10.0.0.1", "10.0.0.4"},
			wantIPs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New()

			for _, ep := range tt.add {
				require.NoError(t, c.UpdateRetinaEndpoint(ep))
			}
			for _, ip := range tt.delete {
				require.NoError(t, c.DeleteRetinaEndpoint(ip))
			}

			gotIPs := c.GetAllIPs()
			require.ElementsMatch(t, tt.wantIPs, gotIPs, "IPs mismatch for test: %s", tt.name)
		})
	}
}
