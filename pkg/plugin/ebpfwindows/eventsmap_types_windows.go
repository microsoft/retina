package ebpfwindows

import "net"

// Notification types
const (
	NotifyUnspec        = 0
	NotifyDrop          = 1
	NotifyDebugMessage  = 2
	NotifyDebugCapture  = 3
	NotifyTrace         = 4
	NotifyPolicyVerdict = 5
	NotifyCapture       = 6
	NotifyTraceSock     = 7
)

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
	Ipv6     bool
	Ifindex  uint32
	OrigIP   net.IP
}
