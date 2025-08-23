// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package ebpfwindows

import (
	"errors"
	"fmt"

	"github.com/cilium/cilium/pkg/byteorder"
	"github.com/cilium/cilium/pkg/identity"
)

const (
	DropNotifyVersion0 = iota
	DropNotifyVersion1
	DropNotifyVersion2
)

const (
	// dropNotifyV1Len is the amount of packet data provided in a v0/v1 drop notification.
	dropNotifyV1Len = 36
)

var dropNotifyLengthFromVersion = map[uint16]uint{
	DropNotifyVersion0: dropNotifyV1Len, // retain backwards compatibility for testing.
	DropNotifyVersion1: dropNotifyV1Len,
}

var (
	errUnexpectedDropNotifyLength = errors.New("unexpected DropNotify data length")
	errInvalidDropNotifyVersion   = errors.New("invalid DropNotify version")
)

// DropNotify is the message format of a drop notification in the BPF ring buffer
type DropNotify struct {
	Type     uint8
	SubType  uint8
	Source   uint16
	Hash     uint32
	OrigLen  uint32
	CapLen   uint16
	Version  uint16
	SrcLabel identity.NumericIdentity
	DstLabel identity.NumericIdentity
	DstID    uint32
	Line     uint16
	File     uint8
	ExtError int8
	Ifindex  uint32
}

// DecodeDropNotify will decode 'data' into the provided DropNotify structure
func DecodePktmonDrop(data []byte, dn *DropNotify) error {
	return dn.decodePktmonDrop(data)
}

func (n *DropNotify) decodePktmonDrop(data []byte) error {
	if l := len(data); l < dropNotifyV1Len {
		return fmt.Errorf("%w: expected at least %d but got %d", errUnexpectedDropNotifyLength, dropNotifyV1Len, l)
	}

	version := byteorder.Native.Uint16(data[14:16])

	// Check against max version.
	if version > DropNotifyVersion1 {
		return fmt.Errorf("%w: Unrecognized drop event version %d", errInvalidDropNotifyVersion, version)
	}

	// Decode logic for version >= v0/v1.
	n.Type = data[0]
	n.SubType = data[1]
	n.Source = byteorder.Native.Uint16(data[2:4])
	n.Hash = byteorder.Native.Uint32(data[4:8])
	n.OrigLen = byteorder.Native.Uint32(data[8:12])
	n.CapLen = byteorder.Native.Uint16(data[12:14])
	n.Version = version
	n.SrcLabel = identity.NumericIdentity(byteorder.Native.Uint32(data[16:20]))
	n.DstLabel = identity.NumericIdentity(byteorder.Native.Uint32(data[20:24]))
	n.DstID = byteorder.Native.Uint32(data[24:28])
	n.Line = byteorder.Native.Uint16(data[28:30])
	n.File = data[30]
	n.ExtError = int8(data[31])
	n.Ifindex = byteorder.Native.Uint32(data[32:36])

	return nil
}

// DecodeDropNotify will decode 'data' into the provided DropNotify structure
func DecodeDropNotify(data []byte, dn *DropNotify) error {
	return dn.decodeDropNotify(data)
}

func (n *DropNotify) decodeDropNotify(data []byte) error {
	if l := len(data); l < dropNotifyV1Len {
		return fmt.Errorf("%w: expected at least %d but got %d", errUnexpectedDropNotifyLength, dropNotifyV1Len, l)
	}

	version := byteorder.Native.Uint16(data[14:16])

	// Check against max version.
	if version > DropNotifyVersion1 {
		return fmt.Errorf("%w: Unrecognized drop event version %d", errInvalidDropNotifyVersion, version)
	}

	// Decode logic for version >= v0/v1.
	n.Type = data[0]
	n.SubType = data[1]
	n.Source = byteorder.Native.Uint16(data[2:4])
	n.Hash = byteorder.Native.Uint32(data[4:8])
	n.OrigLen = byteorder.Native.Uint32(data[8:12])
	n.CapLen = byteorder.Native.Uint16(data[12:14])
	n.Version = version
	n.SrcLabel = identity.NumericIdentity(byteorder.Native.Uint32(data[16:20]))
	n.DstLabel = identity.NumericIdentity(byteorder.Native.Uint32(data[20:24]))
	n.DstID = byteorder.Native.Uint32(data[24:28])
	n.Line = byteorder.Native.Uint16(data[28:30])
	n.File = data[30]
	n.ExtError = int8(data[31])
	n.Ifindex = byteorder.Native.Uint32(data[32:36])

	return nil
}

// IsL3Device returns true if the trace comes from an L3 device.
func (n *DropNotify) IsL3Device() bool {
	return false
}

// IsIPv6 returns true if the trace refers to an IPv6 packet.
func (n *DropNotify) IsIPv6() bool {
	return false
}

// DataOffset returns the offset from the beginning of DropNotify where the
// notification data begins.
//
// Returns zero for invalid or unknown DropNotify messages.
func (n *DropNotify) DataOffset() uint {
	return dropNotifyLengthFromVersion[n.Version]
}
