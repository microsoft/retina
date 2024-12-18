package ebpfwindows

import (
	"fmt"
	"net"
)

// IP represents an IPv4 or IPv4 or IPv6 address
type IP struct {
	Address [16]byte
}

// TraceSockNotify is the notification for a socket trace
type TraceSockNotify struct {
	Type       uint8
	XlatePoint uint8
	DstIP      IP
	DstPort    uint16
	SockCookie uint64
	CgroupID   uint64
	L4Proto    uint8
	IPv6       bool
	Pad        uint8
}

// NotifyCommonHdr is the common header for all notifications
type NotifyCommonHdr struct {
	Type    uint8
	Subtype uint8
	Source  uint16
	Hash    uint32
}

// NotifyCaptureHdr is the common header for all capture notifications
type NotifyCaptureHdr struct {
	NotifyCommonHdr
	LenOrig uint32 // Length of original packet
	LenCap  uint16 // Length of captured bytes
	Version uint16 // Capture header version
}

// DropNotify is the notification for a packet drop
type DropNotify struct {
	NotifyCaptureHdr
	SrcLabel uint32
	DstLabel uint32
	DstID    uint32 // 0 for egress
	Line     uint16
	File     uint8
	ExtError int8
	Ifindex  uint32
}

// TraceNotify is the notification for a packet trace
type TraceNotify struct {
	NotifyCaptureHdr
	SrcLabel uint32
	DstLabel uint32
	DstID    uint16
	Reason   uint8
	IPv6     bool
	Pad      uint8
	Ifindex  uint32
	OrigIP   IP
}

// Notification types
const (
	CILIUM_NOTIFY_UNSPEC         = 0
	CILIUM_NOTIFY_DROP           = 1
	CILIUM_NOTIFY_DBG_MSG        = 2
	CILIUM_NOTIFY_DBG_CAPTURE    = 3
	CILIUM_NOTIFY_TRACE          = 4
	CILIUM_NOTIFY_POLICY_VERDICT = 5
	CILIUM_NOTIFY_CAPTURE        = 6
	CILIUM_NOTIFY_TRACE_SOCK     = 7
)

// String returns a string representation of the DropNotify
func (k *DropNotify) String() string {

	return fmt.Sprintf("Ifindex: %d, SrcLabel:%d, DstLabel:%d, File: %s, Line: %d", k.Ifindex, k.SrcLabel, k.DstLabel, BPFFileName(k.File), k.Line)
}

// String returns a string representation of the TraceNotify
func (k *TraceNotify) String() string {
	var ipAddress string = ""

	if k.IPv6 {
		ipAddress = net.IP(k.OrigIP.Address[:]).String()
	} else {
		ipAddress = net.IP(k.OrigIP.Address[:3]).String()
	}

	return fmt.Sprintf("Ifindex: %d, SrcLabel:%d, DstLabel:%d, IpV6:%t, OrigIP:%s", k.Ifindex, k.SrcLabel, k.DstLabel, k.IPv6, ipAddress)
}

// String returns a string representation of the TraceSockNotify
func (k *TraceSockNotify) String() string {

	var ipAddress string = ""

	if k.IPv6 {
		ipAddress = net.IP(k.DstIP.Address[:]).String()
	} else {
		ipAddress = net.IP(k.DstIP.Address[:3]).String()
	}

	return fmt.Sprintf("DstIP:%s, DstPort:%d, SockCookie:%d, CgroupID:%d, L4Proto:%d, IPv6:%t", ipAddress, k.DstPort, k.SockCookie, k.CgroupID, k.L4Proto, k.IPv6)
}
