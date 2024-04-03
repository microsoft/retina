// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package utils

import (
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/cilium/cilium/api/v1/flow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"

	"github.com/microsoft/retina/pkg/log"
)

func TestToFlow(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	ts := int64(1649748687588860)
	f := ToFlow(ts, net.ParseIP("1.1.1.1").To4(),
		net.ParseIP("2.2.2.2").To4(),
		443, 80, 6, uint32(1), flow.Verdict_FORWARDED, 0)
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
	assert.EqualValues(t, f.L4.Protocol.(*flow.Layer4_TCP).TCP.SourcePort, uint32(443))
	assert.EqualValues(t, f.L4.Protocol.(*flow.Layer4_TCP).TCP.DestinationPort, uint32(80))
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
		f = ToFlow(ts, net.ParseIP("1.1.1.1").To4(),
			net.ParseIP("2.2.2.2").To4(),
			443, 80, 6, uint32(val), flow.Verdict_FORWARDED, 0)
		assert.EqualValues(t, f.TraceObservationPoint, expectedObsPoint[idx])
		assert.EqualValues(t, f.GetEventType().GetSubType(), expectedSubtype[idx])
	}
}

func TestAddPacketSize(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	ts := int64(1649748687588864)
	f := ToFlow(ts, net.ParseIP("1.1.1.1").To4(),
		net.ParseIP("2.2.2.2").To4(),
		443, 80, 6, uint32(1), flow.Verdict_FORWARDED, 0)
	AddPacketSize(f, uint64(100))

	res := PacketSize(f)
	assert.EqualValues(t, res, uint64(100))
}

func TestTcpID(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	ts := int64(1649748687588864)
	f := ToFlow(ts, net.ParseIP("1.1.1.1").To4(),
		net.ParseIP("2.2.2.2").To4(),
		443, 80, 6, uint32(1), flow.Verdict_FORWARDED, 0)
	AddTcpID(f, uint64(1234))
	assert.EqualValues(t, GetTcpID(f), uint64(1234))
}

func TestGetOutgoingInterface_RouteResolves(t *testing.T) {
	const (
		InterfaceName  = "Eth0"
		InterfaceType  = "veth"
		InterfaceIndex = 42
	)

	knownLink := &netlink.Veth{LinkAttrs: netlink.LinkAttrs{Name: InterfaceName, Index: InterfaceIndex}}

	routeGet = func(_ net.IP) ([]netlink.Route, error) {
		return []netlink.Route{{LinkIndex: InterfaceIndex}}, nil
	}
	linkByIndex = func(index int) (netlink.Link, error) {
		if index == InterfaceIndex {
			return knownLink, nil
		}
		return nil, fmt.Errorf("route not found")
	}
	showLink = func() ([]netlink.Link, error) {
		t.Error("should never get here")
		return nil, fmt.Errorf("this function should not be called")
	}

	link, err := GetOutgoingInterface(InterfaceName, InterfaceType)
	require.NoError(t, err)
	require.Equal(t, knownLink, link)
}

func TestGetOutgoingInterface_RouteDoesNotResolves(t *testing.T) {
	const (
		InterfaceName  = "Eth0"
		InterfaceType  = "veth"
		InterfaceIndex = 42
	)

	knownLink := &netlink.Veth{LinkAttrs: netlink.LinkAttrs{Name: InterfaceName, Index: InterfaceIndex}}

	routeGet = func(_ net.IP) ([]netlink.Route, error) {
		return nil, errors.New("route not found")
	}
	linkByIndex = func(index int) (netlink.Link, error) {
		if index == InterfaceIndex {
			return knownLink, nil
		}
		return nil, fmt.Errorf("route not found")
	}
	showLink = func() ([]netlink.Link, error) {
		return []netlink.Link{knownLink}, nil
	}

	link, err := GetOutgoingInterface(InterfaceName, InterfaceType)
	require.NoError(t, err)
	require.Equal(t, knownLink, link)
}
