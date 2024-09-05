// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// nolint //Ignore lint
package utils

import (
	"fmt"
	"net"
	"testing"

	"github.com/cilium/cilium/api/v1/flow"
	"github.com/stretchr/testify/assert"
	"github.com/vishvananda/netlink"

	"github.com/microsoft/retina/pkg/log"
)

func TestToFlow(t *testing.T) {
	l, _ := log.SetupZapLogger(log.GetDefaultLogOpts())

	ts := int64(1649748687588860)
	f := ToFlow(l, ts, net.ParseIP("1.1.1.1").To4(),
		net.ParseIP("2.2.2.2").To4(),
		443, 80, 6, uint8(1), flow.Verdict_FORWARDED)
	/*
		expected  ---> flow.Flow{
			IP: &flow.IP{
				Source:      "1.1.1.1",
				Destination: "2.2.2.2",
				IpVersion:   1,
			},
			L4: &flow.Layer4{
				Protocol: &flow.Layer4_TCP{
					TCP: &flow.TCP{
						SourcePort:      443,
						DestinationPort: 80,
					},
				},
			},
			TraceObservationPoint: flow.TraceObservationPoint_TO_ENDPOINT,
		}
	*/
	assert.Equal(t, f.IP.Source, "1.1.1.1")
	assert.Equal(t, f.IP.Destination, "2.2.2.2")
	assert.Equal(t, f.IP.IpVersion, flow.IPVersion_IPv4)
	assert.EqualValues(t, f.GetL4().Protocol.(*flow.Layer4_TCP).TCP.SourcePort, uint32(443))
	assert.EqualValues(t, f.GetL4().Protocol.(*flow.Layer4_TCP).TCP.DestinationPort, uint32(80))
	assert.NotNil(t, f.Time)
	assert.NotNil(t, f.Extensions)
	assert.Equal(t, f.Type, flow.FlowType_L3_L4)

	if !f.GetTime().IsValid() {
		t.Errorf("Time is not valid")
	}
	assert.EqualValues(t, f.GetTime().AsTime().Format("Oct 1 15:04:05.000"), "Oct 1 02:15:48.687")

	expectedObsPoint := []flow.TraceObservationPoint{
		flow.TraceObservationPoint_TO_STACK,
		flow.TraceObservationPoint_TO_ENDPOINT,
		flow.TraceObservationPoint_FROM_NETWORK,
		flow.TraceObservationPoint_TO_NETWORK,
		flow.TraceObservationPoint_UNKNOWN_POINT,
	}
	expectedSubtype := []int32{3, 0, 10, 11, 0}
	for idx, val := range []uint32{0, 1, 2, 3, 4} {
		f = ToFlow(l, ts, net.ParseIP("1.1.1.1").To4(),
			net.ParseIP("2.2.2.2").To4(),
			443, 80, 6, uint8(val), flow.Verdict_FORWARDED)
		assert.EqualValues(t, f.TraceObservationPoint, expectedObsPoint[idx])
		assert.EqualValues(t, f.GetEventType().GetSubType(), expectedSubtype[idx])
	}
}

func TestAddPacketSize(t *testing.T) {
	l, _ := log.SetupZapLogger(log.GetDefaultLogOpts())

	ts := int64(1649748687588864)
	fl := ToFlow(
		l,
		ts,
		net.ParseIP("1.1.1.1").To4(),
		net.ParseIP("2.2.2.2").To4(),
		443,
		80,
		6,
		uint8(1),
		flow.Verdict_FORWARDED,
	)
	meta := &RetinaMetadata{}
	AddPacketSize(meta, uint32(100))
	AddRetinaMetadata(fl, meta)

	res := PacketSize(fl)
	assert.EqualValues(t, res, uint32(100))
}

func TestTcpID(t *testing.T) {
	l, _ := log.SetupZapLogger(log.GetDefaultLogOpts())

	ts := int64(1649748687588864)
	fl := ToFlow(
		l,
		ts,
		net.ParseIP("1.1.1.1").To4(),
		net.ParseIP("2.2.2.2").To4(),
		443,
		80,
		6,
		uint8(1),
		flow.Verdict_FORWARDED,
	)

	meta := &RetinaMetadata{}
	AddTCPID(meta, uint64(1234))
	AddRetinaMetadata(fl, meta)
	assert.EqualValues(t, GetTCPID(fl), uint64(1234))
}

func TestAddDropReason(t *testing.T) {
	testCases := []struct {
		name                 string
		dropReason           uint16
		expectedDesc         flow.DropReason
		expectedReason       uint32
		expectedRetinaReason string
	}{
		{
			name:           "IPTable Drop",
			dropReason:     0,
			expectedDesc:   flow.DropReason_POLICY_DENIED,
			expectedReason: 133,
		},
		{
			name:           "SNAT Drop",
			dropReason:     1,
			expectedDesc:   flow.DropReason_SNAT_NO_MAP_FOUND,
			expectedReason: 134,
		},
		{
			name:           "Unknown Connection Tracking State",
			dropReason:     5,
			expectedDesc:   flow.DropReason_UNKNOWN_CONNECTION_TRACKING_STATE,
			expectedReason: 138,
		},
		{
			name:           "Unknown Drop Reason",
			dropReason:     6,
			expectedDesc:   flow.DropReason_DROP_REASON_UNKNOWN,
			expectedReason: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f := &flow.Flow{}
			meta := &RetinaMetadata{}
			AddDropReason(f, meta, tc.dropReason)
			AddRetinaMetadata(f, meta)
			assert.Equal(t, f.DropReasonDesc, tc.expectedDesc)
			assert.Equal(t, f.Verdict, flow.Verdict_DROPPED)
			assert.NotNil(t, f.EventType.Type, 1)
			assert.EqualValues(t, f.EventType.GetSubType(), int32(tc.expectedDesc))
			assert.NotNil(t, DropReasonDescription(f), DropReason_name[int32(tc.dropReason)])
		})
	}
}

func TestIsDefaultRoute(t *testing.T) {
	tests := []struct {
		Route           netlink.Route
		ShouldBeDefault bool
	}{
		{Route: netlink.Route{Dst: nil}, ShouldBeDefault: true},
		{Route: netlink.Route{Dst: &net.IPNet{IP: net.IPv4zero, Mask: net.CIDRMask(0, 32)}}, ShouldBeDefault: true},
		{Route: netlink.Route{Dst: &net.IPNet{IP: net.IPv6zero, Mask: net.CIDRMask(0, 128)}}, ShouldBeDefault: true},
		{Route: netlink.Route{Dst: &net.IPNet{IP: net.IPv4(168, 192, 1, 0), Mask: net.CIDRMask(8, 32)}}, ShouldBeDefault: false},
		{Route: netlink.Route{Dst: &net.IPNet{IP: net.IPv6loopback, Mask: net.CIDRMask(64, 128)}}, ShouldBeDefault: false},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%s should be default: %t", test.Route.Dst, test.ShouldBeDefault), func(t *testing.T) {
			assert.Equal(t, test.ShouldBeDefault, isDefaultRoute(test.Route))
		})
	}
}
