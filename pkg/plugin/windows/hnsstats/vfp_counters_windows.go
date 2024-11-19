// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package hnsstats

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// IN/OUT Direction of vSwitch VFP port
type Direction int

const (
	OUT Direction = iota
	IN
)

type VfpPortStatsData struct {
	// IN counters
	In VfpDirectedPortCounters
	// OUT counters
	Out VfpDirectedPortCounters
}

type VfpDirectedPortCounters struct {
	direction Direction
	// Contains TCP-level counters
	TcpCounters VfpTcpStats
	// Contains generic drop counters
	DropCounters VfpPacketDropStats
}

type VfpTcpStats struct {
	ConnectionCounters VfpTcpConnectionStats
	PacketCounters     VfpTcpPacketStats
}

type VfpPacketDropStats struct {
	AclDropPacketCount uint64
}

type VfpTcpPacketStats struct {
	SynPacketCount    uint64
	SynAckPacketCount uint64
	FinPacketCount    uint64
	RstPacketCount    uint64
}

type VfpTcpConnectionStats struct {
	VerifiedCount            uint64
	TimedOutCount            uint64
	ResetCount               uint64
	ResetSynCount            uint64
	ClosedFinCount           uint64
	TcpHalfOpenTimeoutsCount uint64
	TimeWaitExpiredCount     uint64
}

// Attach TCP counters from vfpctrl /get-port-counters lines
func attachVfpCounter(stats *VfpDirectedPortCounters, identifier string, ucount uint64) {
	// TCP Packet identifiers
	synIdentifier := "SYNpackets"
	synAckIdentifier := "SYN-ACKpackets"
	finIdentifier := "FINpackets"
	rstIdentifier := "RSTpackets"
	// TCP conn identifiers
	tcpConnVerifiedIdentifier := "TCPConnectionsVerified"
	tcpConnTimedOutIdentifier := "TCPConnectionsTimedOut"
	tcpResetIdentifier := "TCPConnectionsReset"
	tcpResetSynIdentifier := "TCPConnectionsResetbySYN"
	tcpConnClosedFinIdentifier := "TCPConnectionsClosedbyFIN"
	tcpHalfOpenTimeoutIdentifier := "TCPHalfOpenTimeouts"
	tcpConnExpiredTimeWaitIdentifier := "TCPConnectionsExpiredtoTimeWait"
	// Drop Packet counters
	dropAclPacketCounter := "DroppedACLpackets"

	switch identifier {
	// TCP packet counters
	case synIdentifier:
		stats.TcpCounters.PacketCounters.SynPacketCount = ucount
	case synAckIdentifier:
		stats.TcpCounters.PacketCounters.SynAckPacketCount = ucount
	case finIdentifier:
		stats.TcpCounters.PacketCounters.FinPacketCount = ucount
	case rstIdentifier:
		stats.TcpCounters.PacketCounters.RstPacketCount = ucount
	// TCP connection ucounters
	case tcpConnVerifiedIdentifier:
		stats.TcpCounters.ConnectionCounters.VerifiedCount = ucount
	case tcpConnTimedOutIdentifier:
		stats.TcpCounters.ConnectionCounters.TimedOutCount = ucount
	case tcpResetIdentifier:
		stats.TcpCounters.ConnectionCounters.ResetCount = ucount
	case tcpResetSynIdentifier:
		stats.TcpCounters.ConnectionCounters.ResetSynCount = ucount
	case tcpConnClosedFinIdentifier:
		stats.TcpCounters.ConnectionCounters.ClosedFinCount = ucount
	case tcpHalfOpenTimeoutIdentifier:
		stats.TcpCounters.ConnectionCounters.TcpHalfOpenTimeoutsCount = ucount
	case tcpConnExpiredTimeWaitIdentifier:
		stats.TcpCounters.ConnectionCounters.TimeWaitExpiredCount = ucount
	case dropAclPacketCounter:
		stats.DropCounters.AclDropPacketCount = ucount
	}
}

// This will extract all the VFP Port counters.
func parseVfpPortCounters(countersRaw string) (*VfpPortStatsData, error) {
	delim := "\r\n"
	portCounters := &VfpPortStatsData{}
	DirectionInIdentifier := "Direction-IN"

	// Remove spaces
	countersRaw = strings.ReplaceAll(countersRaw, " ", "")
	// Split string into Direction OUT/IN. Out always comes first (direction = 0).
	for direction, countersRaw := range strings.Split(countersRaw, DirectionInIdentifier) {
		for _, line := range strings.Split(countersRaw, delim) {
			// Identifier: Value
			fields := strings.Split(line, ":")
			// Skip redundant lines
			if len(fields) != 2 {
				continue
			}
			// Extract count and convert to uint64
			count, err := strconv.ParseInt(fields[1], 10, 64)
			identifier := fields[0]
			if err != nil {
				return nil, err
			}
			var ucount uint64 = uint64(count)
			// Populate VfpPort stats
			switch Direction(direction) {
			case OUT:
				portCounters.Out.direction = OUT
				attachVfpCounter(&portCounters.Out, identifier, ucount)
			case IN:
				portCounters.In.direction = IN
				attachVfpCounter(&portCounters.In, identifier, ucount)
			}
		}
	}
	return portCounters, nil
}

// getVfpPortCountersRaw will return the raw vfpctrl port counter output for the given port.
func getVfpPortCountersRaw(portGUID string) (string, error) {
	vfpCmd := fmt.Sprintf("vfpctrl /port %s /get-port-counter", portGUID)

	cmd := exec.Command("cmd", "/c", vfpCmd)
	out, err := cmd.Output()

	return string(out), errors.Wrap(err, "errored while running vfpctrl /get-port-counter")
}

// TODO: Remove this once Resources.Allocators.EndpointPortGuid gets added to hcsshim Endpoint struct
// Lists all vSwitch ports
func listvPorts() ([]byte, error) {
	out, err := exec.Command("cmd", "/c", "vfpctrl /list-vmswitch-port").CombinedOutput()
	return out, errors.Wrap(err, "errored while running vfpctrl /list-vmswitch-port")
}

// TODO: Remove this once Resources.Allocators.EndpointPortGuid gets added to hcsshim Endpoint struct
// getMacToPortGuidMap returns a map storing  MAC Address => VFP Port GUID mappings
func getMacToPortGuidMap() (kv map[string]string, err error) {
	kv = make(map[string]string)
	out, err := listvPorts()
	if err != nil {
		return kv, err
	}
	// Some very case-specific parsing
	delim := "\r\n"
	portList := string(out)
	portList = strings.ReplaceAll(portList, " ", "")
	for _, port := range strings.Split(portList, delim+delim) {
		// Skip port if there is no Portname or MACAddress field.
		if !strings.Contains(port, "Portname") || !strings.Contains(port, "MACaddress") {
			continue
		}
		// Populate map[mac] => VFP port name guid
		var mac string
		var portName string
		for _, line := range strings.Split(port, delim) {
			fields := strings.Split(line, ":")
			switch fields[0] {
			case "Portname":
				portName = fields[1]
			case "MACaddress":
				mac = fields[1]
			}
		}
		kv[mac] = portName
	}
	return
}

func (tcpCounters *VfpTcpStats) String() string {
	return fmt.Sprintf(
		"Packets:\n SYN %d,\n SYN-ACK %d,\n FIN %d,\n RST %d\n"+
			"TCP Connections:\n Verified %d,\n TimedOut %d,\n Reset %d,\n Reset-Syn %d,\n"+
			" ClosedFin %d,\n HalfOpenTimeout %d,\n TimeWaitExpired %d\n ",
		tcpCounters.PacketCounters.SynPacketCount, tcpCounters.PacketCounters.SynAckPacketCount,
		tcpCounters.PacketCounters.FinPacketCount, tcpCounters.PacketCounters.RstPacketCount,
		tcpCounters.ConnectionCounters.VerifiedCount, tcpCounters.ConnectionCounters.TimedOutCount,
		tcpCounters.ConnectionCounters.ResetCount, tcpCounters.ConnectionCounters.ResetSynCount,
		tcpCounters.ConnectionCounters.ClosedFinCount, tcpCounters.ConnectionCounters.TcpHalfOpenTimeoutsCount,
		tcpCounters.ConnectionCounters.TimeWaitExpiredCount)
}

func (drops *VfpPacketDropStats) String() string {
	return fmt.Sprintf("Drops:\n Dropped ACL packets %d\n", drops.AclDropPacketCount)
}

func (v *VfpPortStatsData) String() string {
	return fmt.Sprintf("\nDirection IN:\n %s%s\n", v.In.TcpCounters.String(), v.In.DropCounters.String()) + fmt.Sprintf("Direction OUT:\n %s%s\n", v.Out.TcpCounters.String(), v.Out.DropCounters.String())
}
