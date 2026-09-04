// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package ebpfwindows

import (
	"errors"
	"fmt"
	"net"

	"github.com/cilium/cilium/pkg/byteorder"
	"github.com/cilium/cilium/pkg/identity"
	"github.com/cilium/cilium/pkg/types"
)

const (
	// traceNotifyV0Len is the amount of packet data provided in a trace notification v0.
	traceNotifyV0Len = 32
	// traceNotifyV1Len is the amount of packet data provided in a trace notification v1.
	traceNotifyV1Len = 48
)

const (
	// TraceNotifyFlagIsIPv6 is set in TraceNotify.Flags when the
	// notification refers to an IPv6 flow
	TraceNotifyFlagIsIPv6 uint8 = 1 << iota
	// TraceNotifyFlagIsL3Device is set in TraceNotify.Flags when the
	// notification refers to a L3 device.
	TraceNotifyFlagIsL3Device
)

const (
	TraceNotifyVersion0 = iota
	TraceNotifyVersion1
)

var (
	errTraceNotifyLength      = errors.New("unexpected TraceNotify data length")
	errUnrecognizedTraceEvent = errors.New("unrecognized trace event")
)

// TraceNotify is the message format of a trace notification in the BPF ring buffer
type TraceNotify struct {
	Type     uint8
	ObsPoint uint8
	Source   uint16
	Hash     uint32
	OrigLen  uint32
	CapLen   uint16
	Version  uint16
	SrcLabel identity.NumericIdentity
	DstLabel identity.NumericIdentity
	DstID    uint16
	Reason   uint8
	Flags    uint8
	Ifindex  uint32
	OrigIP   types.IPv6
	// data
}

// decodeTraceNotify decodes the trace notify message in 'data' into the struct.
func (tn *TraceNotify) decodeTraceNotify(data []byte) error {
	if l := len(data); l < traceNotifyV0Len {
		return fmt.Errorf("%w: expected at least %d but got %d", errTraceNotifyLength, traceNotifyV0Len, l)
	}

	version := byteorder.Native.Uint16(data[14:16])

	// Check against max version.
	if version > TraceNotifyVersion1 {
		return fmt.Errorf("%w: version %d", errUnrecognizedTraceEvent, version)
	}

	// Decode logic for version >= v1.
	if version >= TraceNotifyVersion1 {
		if l := len(data); l < traceNotifyV1Len {
			return fmt.Errorf("%w (version %d): expected at least %d but got %d", errTraceNotifyLength, version, traceNotifyV1Len, l)
		}
		copy(tn.OrigIP[:], data[32:48])
	}

	// Decode logic for version >= v0.
	tn.Type = data[0]
	tn.ObsPoint = data[1]
	tn.Source = byteorder.Native.Uint16(data[2:4])
	tn.Hash = byteorder.Native.Uint32(data[4:8])
	tn.OrigLen = byteorder.Native.Uint32(data[8:12])
	tn.CapLen = byteorder.Native.Uint16(data[12:14])
	tn.Version = version
	tn.SrcLabel = identity.NumericIdentity(byteorder.Native.Uint32(data[16:20]))
	tn.DstLabel = identity.NumericIdentity(byteorder.Native.Uint32(data[20:24]))
	tn.DstID = byteorder.Native.Uint16(data[24:26])
	tn.Reason = data[26]
	tn.Flags = data[27]
	tn.Ifindex = byteorder.Native.Uint32(data[28:32])

	return nil
}

// IsEncrypted returns true when the notification has the encrypt flag set,
// false otherwise.
func (tn *TraceNotify) IsEncrypted() bool {
	return (tn.Reason & TraceReasonEncryptMask) != 0
}

// TraceReason returns the trace reason for this notification, see the
// TraceReason* constants.
func (tn *TraceNotify) TraceReason() uint8 {
	return tn.Reason & ^TraceReasonEncryptMask
}

// TraceReasonIsKnown returns false when the trace reason is unknown, true
// otherwise.
func (tn *TraceNotify) TraceReasonIsKnown() bool {
	return tn.TraceReason() != TraceReasonUnknown
}

// TraceReasonIsReply returns true when the trace reason is TraceReasonCtReply,
// false otherwise.
func (tn *TraceNotify) TraceReasonIsReply() bool {
	return tn.TraceReason() == TraceReasonCtReply
}

// TraceReasonIsEncap returns true when the trace reason is encapsulation
// related, false otherwise.
func (tn *TraceNotify) TraceReasonIsEncap() bool {
	switch tn.TraceReason() {
	case TraceReasonSRv6Encap, TraceReasonEncryptOverlay:
		return true
	}
	return false
}

// TraceReasonIsDecap returns true when the trace reason is decapsulation
// related, false otherwise.
func (tn *TraceNotify) TraceReasonIsDecap() bool {
	return tn.TraceReason() == TraceReasonSRv6Decap
}

var traceNotifyLength = map[uint16]uint{
	TraceNotifyVersion0: traceNotifyV0Len,
	TraceNotifyVersion1: traceNotifyV1Len,
}

/* Reasons for forwarding a packet, keep in sync with api/v1/flow/flow.proto */
const (
	TraceReasonPolicy = iota
	TraceReasonCtEstablished
	TraceReasonCtReply
	TraceReasonCtRelated
	TraceReasonCtDeprecatedReopened
	TraceReasonUnknown
	TraceReasonSRv6Encap
	TraceReasonSRv6Decap
	TraceReasonEncryptOverlay
	// TraceReasonEncryptMask is the bit used to indicate encryption or not.
	TraceReasonEncryptMask = uint8(0x80)
)

// DecodeTraceNotify will decode 'data' into the provided TraceNotify structure
func DecodeTraceNotify(data []byte, tn *TraceNotify) error {
	return tn.decodeTraceNotify(data)
}

// IsL3Device returns true if the trace comes from an L3 device.
func (tn *TraceNotify) IsL3Device() bool {
	return tn.Flags&TraceNotifyFlagIsL3Device != 0
}

// IsIPv6 returns true if the trace refers to an IPv6 packet.
func (tn *TraceNotify) IsIPv6() bool {
	return tn.Flags&TraceNotifyFlagIsIPv6 != 0
}

// OriginalIP returns the original source IP if reverse NAT was performed on
// the flow
func (tn *TraceNotify) OriginalIP() net.IP {
	if tn.IsIPv6() {
		return tn.OrigIP[:]
	}
	return tn.OrigIP[:4]
}

// DataOffset returns the offset from the beginning of TraceNotify where the
// trace notify data begins.
//
// Returns zero for invalid or unknown TraceNotify messages.
func (tn *TraceNotify) DataOffset() uint {
	return traceNotifyLength[tn.Version]
}
