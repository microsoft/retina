package ebpfwindows

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
)

// IP represents an IPv4 or IPv4 or IPv6 address
type IP struct {
	Address uint32
	Pad1    uint32
	Pad2    uint32
	Pad3    uint32
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
	Data [128]byte
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
	Data [128]byte
}

// TraceNotify is the notification for a packet trace
type TraceNotify struct {
	NotifyCaptureHdr
	SrcLabel uint32
	DstLabel uint32
	DstID    uint16
	Reason   uint8
	IPv6     bool
	Ifindex  uint32
	OrigIP   IP
	Data [128]byte
}

// Notification types
const (
	CiliumNotifyUnspec        = 0
	CiliumNotifyDrop          = 1
	CiliumNotifyDebugMessage  = 2
	CiliumNotifyDebugCapture  = 3
	CiliumNotifyTrace         = 4
	CiliumNotifyPolicyVerdict = 5
	CiliumNotifyCapture       = 6
	CiliumNotifyTraceSock     = 7
)

func (ip *IP) ConvertToString(IPv6 bool) string {
	var ipAddress string
	var buf bytes.Buffer

	err := binary.Write(&buf, binary.BigEndian, *ip)

	if err != nil {
		return ""
	}

	byteArray := buf.Bytes()

	if IPv6 {
		ipAddress = net.IP(byteArray[:16]).String()
	} else {
		ipAddress = net.IP(byteArray[:4]).String()
	}

	return ipAddress

}

// String returns a string representation of the DropNotify
func (k *DropNotify) String() string {

	return fmt.Sprintf("Ifindex: %d, SrcLabel:%d, DstLabel:%d, File: %s, Line: %d", k.Ifindex, k.SrcLabel, k.DstLabel, BPFFileName(k.File), k.Line)
}

// String returns a string representation of the TraceNotify
func (k *TraceNotify) String() string {
	ipAddress := k.OrigIP.ConvertToString(k.IPv6)
	return fmt.Sprintf("Ifindex: %d, SrcLabel:%d, DstLabel:%d, IpV6:%t, OrigIP:%s", k.Ifindex, k.SrcLabel, k.DstLabel, k.IPv6, ipAddress)
}

// String returns a string representation of the TraceSockNotify
func (k *TraceSockNotify) String() string {
	ipAddress := k.DstIP.ConvertToString(k.IPv6)
	return fmt.Sprintf("DstIP:%s, DstPort:%d, SockCookie:%d, CgroupID:%d, L4Proto:%d, IPv6:%t", ipAddress, k.DstPort, k.SockCookie, k.CgroupID, k.L4Proto, k.IPv6)
}
