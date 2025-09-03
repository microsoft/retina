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
	dropNotifyV1Len       = 36
	dropPktmonNotifyV1Len = 57
)

var dropNotifyLengthFromVersion = map[uint16]uint{
	DropNotifyVersion0: dropNotifyV1Len, // retain backwards compatibility for testing.
	DropNotifyVersion1: dropNotifyV1Len,
}

var pktmonDropNotifyLengthFromVersion = map[uint16]uint{
	DropNotifyVersion1: dropPktmonNotifyV1Len,
}

var (
	errUnexpectedDropNotifyLength     = errors.New("unexpected DropNotify data length")
	errInvalidDropNotifyVersion       = errors.New("invalid DropNotify version")
	errInvalidPktmonDropNotifyVersion = errors.New("invalid Pktmon DropNotify version")
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

type NetEventDataHeader struct {
	Type    uint8  // uint8_t type
	Version uint16 // uint16_t version
}

type PktmonEvtStreamPacketDescriptor struct {
	PacketOriginalLength uint32 // uint32_t packet_original_length
	PacketLoggedLength   uint32 // uint32_t packet_logged_length
	PacketMetadataLength uint32 // uint32_t packet_metadata_length
}

type PktmonEvtStreamMetadata struct {
	PktGroupID      uint64 // uint64_t pkt_groupid
	PktCount        uint16 // uint16_t pkt_count
	AppearanceCount uint16 // uint16_t appearance_count
	DirectionName   uint16 // uint16_t direction_name
	PacketType      uint16 // uint16_t packet_type
	ComponentID     uint16 // uint16_t component_id
	EdgeID          uint16 // uint16_t edge_id
	FilterID        uint16 // uint16_t filter_id
	DropReason      uint32 // uint32_t drop_reason
	DropLocation    uint32 // uint32_t drop_location
	ProcNum         uint16 // uint16_t proc_num
	Timestamp       uint64 // uint64_t timestamp
}

type PktmonEvtStreamPacketHeader struct {
	EventID          uint8                           // uint8_t eventid
	PacketDescriptor PktmonEvtStreamPacketDescriptor // pktmon_evt_stream_packet_descriptor
	Metadata         PktmonEvtStreamMetadata         // pktmon_evt_stream_metadata
}

type PktmonDropNotify struct {
	VersionHeader NetEventDataHeader          // netevent_data_header_t version_header
	PktmonHeader  PktmonEvtStreamPacketHeader // pktmon_evt_stream_packet_header pktmon_header
}

// DecodeDropNotify will decode 'data' into the provided DropNotify structure
func DecodePktmonDrop(data []byte, pdn *PktmonDropNotify) error {
	if err := pdn.decodePktmonDrop(data); err != nil {
		return err
	}
	return nil
}

// DataOffset returns the offset from the beginning of PktmonDropNotify where the
// notification data begins.
func (n *PktmonDropNotify) DataOffset() uint {
	return pktmonDropNotifyLengthFromVersion[n.VersionHeader.Version]
}

func (n *PktmonDropNotify) decodePktmonDrop(data []byte) error {
	if l := len(data); l < dropPktmonNotifyV1Len {
		return fmt.Errorf("%w: expected at least %d but got %d", errUnexpectedDropNotifyLength, dropPktmonNotifyV1Len, l)
	}
	version := byteorder.Native.Uint16(data[2:4])

	// Check against max version.
	if version > DropNotifyVersion1 {
		return fmt.Errorf("%w: Unrecognized drop event version %d", errInvalidPktmonDropNotifyVersion, version)
	}

	// Decode logic for version >= v0/v1.
	n.VersionHeader.Type = data[0]
	n.VersionHeader.Version = version
	n.PktmonHeader.EventID = data[4]
	n.PktmonHeader.PacketDescriptor.PacketOriginalLength = byteorder.Native.Uint32(data[5:9])
	n.PktmonHeader.PacketDescriptor.PacketLoggedLength = byteorder.Native.Uint32(data[9:13])
	n.PktmonHeader.PacketDescriptor.PacketMetadataLength = byteorder.Native.Uint32(data[13:17])
	n.PktmonHeader.Metadata.PktGroupID = byteorder.Native.Uint64(data[17:25])
	n.PktmonHeader.Metadata.PktCount = byteorder.Native.Uint16(data[25:27])
	n.PktmonHeader.Metadata.AppearanceCount = byteorder.Native.Uint16(data[27:29])
	n.PktmonHeader.Metadata.DirectionName = byteorder.Native.Uint16(data[29:31])
	n.PktmonHeader.Metadata.PacketType = byteorder.Native.Uint16(data[31:33])
	n.PktmonHeader.Metadata.ComponentID = byteorder.Native.Uint16(data[33:35])
	n.PktmonHeader.Metadata.EdgeID = byteorder.Native.Uint16(data[35:37])
	n.PktmonHeader.Metadata.FilterID = byteorder.Native.Uint16(data[37:39])
	n.PktmonHeader.Metadata.DropReason = byteorder.Native.Uint32(data[39:43])
	n.PktmonHeader.Metadata.DropLocation = byteorder.Native.Uint32(data[43:47])
	n.PktmonHeader.Metadata.ProcNum = byteorder.Native.Uint16(data[47:49])
	n.PktmonHeader.Metadata.Timestamp = byteorder.Native.Uint64(data[49:57])
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
