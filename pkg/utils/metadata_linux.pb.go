// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        v4.24.2
// source: pkg/utils/metadata_linux.proto

package utils

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type DNSType int32

const (
	DNSType_UNKNOWN  DNSType = 0
	DNSType_QUERY    DNSType = 1
	DNSType_RESPONSE DNSType = 2
)

// Enum value maps for DNSType.
var (
	DNSType_name = map[int32]string{
		0: "UNKNOWN",
		1: "QUERY",
		2: "RESPONSE",
	}
	DNSType_value = map[string]int32{
		"UNKNOWN":  0,
		"QUERY":    1,
		"RESPONSE": 2,
	}
)

func (x DNSType) Enum() *DNSType {
	p := new(DNSType)
	*p = x
	return p
}

func (x DNSType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (DNSType) Descriptor() protoreflect.EnumDescriptor {
	return file_pkg_utils_metadata_linux_proto_enumTypes[0].Descriptor()
}

func (DNSType) Type() protoreflect.EnumType {
	return &file_pkg_utils_metadata_linux_proto_enumTypes[0]
}

func (x DNSType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use DNSType.Descriptor instead.
func (DNSType) EnumDescriptor() ([]byte, []int) {
	return file_pkg_utils_metadata_linux_proto_rawDescGZIP(), []int{0}
}

// Ref: pkg/plugin/dropreason/_cprog/drop_reason.h.
type DropReason int32

const (
	DropReason_IPTABLE_RULE_DROP  DropReason = 0
	DropReason_IPTABLE_NAT_DROP   DropReason = 1
	DropReason_TCP_CONNECT_BASIC  DropReason = 2
	DropReason_TCP_ACCEPT_BASIC   DropReason = 3
	DropReason_TCP_CLOSE_BASIC    DropReason = 4
	DropReason_CONNTRACK_ADD_DROP DropReason = 5
	DropReason_UNKNOWN_DROP       DropReason = 6
)

// Enum value maps for DropReason.
var (
	DropReason_name = map[int32]string{
		0: "IPTABLE_RULE_DROP",
		1: "IPTABLE_NAT_DROP",
		2: "TCP_CONNECT_BASIC",
		3: "TCP_ACCEPT_BASIC",
		4: "TCP_CLOSE_BASIC",
		5: "CONNTRACK_ADD_DROP",
		6: "UNKNOWN_DROP",
	}
	DropReason_value = map[string]int32{
		"IPTABLE_RULE_DROP":  0,
		"IPTABLE_NAT_DROP":   1,
		"TCP_CONNECT_BASIC":  2,
		"TCP_ACCEPT_BASIC":   3,
		"TCP_CLOSE_BASIC":    4,
		"CONNTRACK_ADD_DROP": 5,
		"UNKNOWN_DROP":       6,
	}
)

func (x DropReason) Enum() *DropReason {
	p := new(DropReason)
	*p = x
	return p
}

func (x DropReason) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (DropReason) Descriptor() protoreflect.EnumDescriptor {
	return file_pkg_utils_metadata_linux_proto_enumTypes[1].Descriptor()
}

func (DropReason) Type() protoreflect.EnumType {
	return &file_pkg_utils_metadata_linux_proto_enumTypes[1]
}

func (x DropReason) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use DropReason.Descriptor instead.
func (DropReason) EnumDescriptor() ([]byte, []int) {
	return file_pkg_utils_metadata_linux_proto_rawDescGZIP(), []int{1}
}

type RetinaMetadata struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Bytes uint32 `protobuf:"varint,1,opt,name=bytes,proto3" json:"bytes,omitempty"`
	// DNS metadata.
	DnsType      DNSType `protobuf:"varint,2,opt,name=dns_type,json=dnsType,proto3,enum=utils.DNSType" json:"dns_type,omitempty"`
	NumResponses uint32  `protobuf:"varint,3,opt,name=num_responses,json=numResponses,proto3" json:"num_responses,omitempty"`
	// TCP ID. Either Tsval or Tsecr will be set.
	TcpId uint64 `protobuf:"varint,4,opt,name=tcp_id,json=tcpId,proto3" json:"tcp_id,omitempty"`
	// Drop reason in Retina.
	DropReason DropReason `protobuf:"varint,5,opt,name=drop_reason,json=dropReason,proto3,enum=utils.DropReason" json:"drop_reason,omitempty"`
	// Sampling metadata, for packetparser.
	PreviouslyObservedPackets  uint32            `protobuf:"varint,6,opt,name=previously_observed_packets,json=previouslyObservedPackets,proto3" json:"previously_observed_packets,omitempty"`
	PreviouslyObservedBytes    uint32            `protobuf:"varint,7,opt,name=previously_observed_bytes,json=previouslyObservedBytes,proto3" json:"previously_observed_bytes,omitempty"`
	PreviouslyObservedTcpFlags map[string]uint32 `protobuf:"bytes,8,rep,name=previously_observed_tcp_flags,json=previouslyObservedTcpFlags,proto3" json:"previously_observed_tcp_flags,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"varint,2,opt,name=value,proto3"`
}

func (x *RetinaMetadata) Reset() {
	*x = RetinaMetadata{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_utils_metadata_linux_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RetinaMetadata) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RetinaMetadata) ProtoMessage() {}

func (x *RetinaMetadata) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_utils_metadata_linux_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RetinaMetadata.ProtoReflect.Descriptor instead.
func (*RetinaMetadata) Descriptor() ([]byte, []int) {
	return file_pkg_utils_metadata_linux_proto_rawDescGZIP(), []int{0}
}

func (x *RetinaMetadata) GetBytes() uint32 {
	if x != nil {
		return x.Bytes
	}
	return 0
}

func (x *RetinaMetadata) GetDnsType() DNSType {
	if x != nil {
		return x.DnsType
	}
	return DNSType_UNKNOWN
}

func (x *RetinaMetadata) GetNumResponses() uint32 {
	if x != nil {
		return x.NumResponses
	}
	return 0
}

func (x *RetinaMetadata) GetTcpId() uint64 {
	if x != nil {
		return x.TcpId
	}
	return 0
}

func (x *RetinaMetadata) GetDropReason() DropReason {
	if x != nil {
		return x.DropReason
	}
	return DropReason_IPTABLE_RULE_DROP
}

func (x *RetinaMetadata) GetPreviouslyObservedPackets() uint32 {
	if x != nil {
		return x.PreviouslyObservedPackets
	}
	return 0
}

func (x *RetinaMetadata) GetPreviouslyObservedBytes() uint32 {
	if x != nil {
		return x.PreviouslyObservedBytes
	}
	return 0
}

func (x *RetinaMetadata) GetPreviouslyObservedTcpFlags() map[string]uint32 {
	if x != nil {
		return x.PreviouslyObservedTcpFlags
	}
	return nil
}

var File_pkg_utils_metadata_linux_proto protoreflect.FileDescriptor

var file_pkg_utils_metadata_linux_proto_rawDesc = []byte{
	0x0a, 0x1e, 0x70, 0x6b, 0x67, 0x2f, 0x75, 0x74, 0x69, 0x6c, 0x73, 0x2f, 0x6d, 0x65, 0x74, 0x61,
	0x64, 0x61, 0x74, 0x61, 0x5f, 0x6c, 0x69, 0x6e, 0x75, 0x78, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x12, 0x05, 0x75, 0x74, 0x69, 0x6c, 0x73, 0x22, 0x86, 0x04, 0x0a, 0x0e, 0x52, 0x65, 0x74, 0x69,
	0x6e, 0x61, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x12, 0x14, 0x0a, 0x05, 0x62, 0x79,
	0x74, 0x65, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x05, 0x62, 0x79, 0x74, 0x65, 0x73,
	0x12, 0x29, 0x0a, 0x08, 0x64, 0x6e, 0x73, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x0e, 0x32, 0x0e, 0x2e, 0x75, 0x74, 0x69, 0x6c, 0x73, 0x2e, 0x44, 0x4e, 0x53, 0x54, 0x79,
	0x70, 0x65, 0x52, 0x07, 0x64, 0x6e, 0x73, 0x54, 0x79, 0x70, 0x65, 0x12, 0x23, 0x0a, 0x0d, 0x6e,
	0x75, 0x6d, 0x5f, 0x72, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x73, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x0d, 0x52, 0x0c, 0x6e, 0x75, 0x6d, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x73,
	0x12, 0x15, 0x0a, 0x06, 0x74, 0x63, 0x70, 0x5f, 0x69, 0x64, 0x18, 0x04, 0x20, 0x01, 0x28, 0x04,
	0x52, 0x05, 0x74, 0x63, 0x70, 0x49, 0x64, 0x12, 0x32, 0x0a, 0x0b, 0x64, 0x72, 0x6f, 0x70, 0x5f,
	0x72, 0x65, 0x61, 0x73, 0x6f, 0x6e, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x11, 0x2e, 0x75,
	0x74, 0x69, 0x6c, 0x73, 0x2e, 0x44, 0x72, 0x6f, 0x70, 0x52, 0x65, 0x61, 0x73, 0x6f, 0x6e, 0x52,
	0x0a, 0x64, 0x72, 0x6f, 0x70, 0x52, 0x65, 0x61, 0x73, 0x6f, 0x6e, 0x12, 0x3e, 0x0a, 0x1b, 0x70,
	0x72, 0x65, 0x76, 0x69, 0x6f, 0x75, 0x73, 0x6c, 0x79, 0x5f, 0x6f, 0x62, 0x73, 0x65, 0x72, 0x76,
	0x65, 0x64, 0x5f, 0x70, 0x61, 0x63, 0x6b, 0x65, 0x74, 0x73, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0d,
	0x52, 0x19, 0x70, 0x72, 0x65, 0x76, 0x69, 0x6f, 0x75, 0x73, 0x6c, 0x79, 0x4f, 0x62, 0x73, 0x65,
	0x72, 0x76, 0x65, 0x64, 0x50, 0x61, 0x63, 0x6b, 0x65, 0x74, 0x73, 0x12, 0x3a, 0x0a, 0x19, 0x70,
	0x72, 0x65, 0x76, 0x69, 0x6f, 0x75, 0x73, 0x6c, 0x79, 0x5f, 0x6f, 0x62, 0x73, 0x65, 0x72, 0x76,
	0x65, 0x64, 0x5f, 0x62, 0x79, 0x74, 0x65, 0x73, 0x18, 0x07, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x17,
	0x70, 0x72, 0x65, 0x76, 0x69, 0x6f, 0x75, 0x73, 0x6c, 0x79, 0x4f, 0x62, 0x73, 0x65, 0x72, 0x76,
	0x65, 0x64, 0x42, 0x79, 0x74, 0x65, 0x73, 0x12, 0x78, 0x0a, 0x1d, 0x70, 0x72, 0x65, 0x76, 0x69,
	0x6f, 0x75, 0x73, 0x6c, 0x79, 0x5f, 0x6f, 0x62, 0x73, 0x65, 0x72, 0x76, 0x65, 0x64, 0x5f, 0x74,
	0x63, 0x70, 0x5f, 0x66, 0x6c, 0x61, 0x67, 0x73, 0x18, 0x08, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x35,
	0x2e, 0x75, 0x74, 0x69, 0x6c, 0x73, 0x2e, 0x52, 0x65, 0x74, 0x69, 0x6e, 0x61, 0x4d, 0x65, 0x74,
	0x61, 0x64, 0x61, 0x74, 0x61, 0x2e, 0x50, 0x72, 0x65, 0x76, 0x69, 0x6f, 0x75, 0x73, 0x6c, 0x79,
	0x4f, 0x62, 0x73, 0x65, 0x72, 0x76, 0x65, 0x64, 0x54, 0x63, 0x70, 0x46, 0x6c, 0x61, 0x67, 0x73,
	0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x1a, 0x70, 0x72, 0x65, 0x76, 0x69, 0x6f, 0x75, 0x73, 0x6c,
	0x79, 0x4f, 0x62, 0x73, 0x65, 0x72, 0x76, 0x65, 0x64, 0x54, 0x63, 0x70, 0x46, 0x6c, 0x61, 0x67,
	0x73, 0x1a, 0x4d, 0x0a, 0x1f, 0x50, 0x72, 0x65, 0x76, 0x69, 0x6f, 0x75, 0x73, 0x6c, 0x79, 0x4f,
	0x62, 0x73, 0x65, 0x72, 0x76, 0x65, 0x64, 0x54, 0x63, 0x70, 0x46, 0x6c, 0x61, 0x67, 0x73, 0x45,
	0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01,
	0x2a, 0x2f, 0x0a, 0x07, 0x44, 0x4e, 0x53, 0x54, 0x79, 0x70, 0x65, 0x12, 0x0b, 0x0a, 0x07, 0x55,
	0x4e, 0x4b, 0x4e, 0x4f, 0x57, 0x4e, 0x10, 0x00, 0x12, 0x09, 0x0a, 0x05, 0x51, 0x55, 0x45, 0x52,
	0x59, 0x10, 0x01, 0x12, 0x0c, 0x0a, 0x08, 0x52, 0x45, 0x53, 0x50, 0x4f, 0x4e, 0x53, 0x45, 0x10,
	0x02, 0x2a, 0xa5, 0x01, 0x0a, 0x0a, 0x44, 0x72, 0x6f, 0x70, 0x52, 0x65, 0x61, 0x73, 0x6f, 0x6e,
	0x12, 0x15, 0x0a, 0x11, 0x49, 0x50, 0x54, 0x41, 0x42, 0x4c, 0x45, 0x5f, 0x52, 0x55, 0x4c, 0x45,
	0x5f, 0x44, 0x52, 0x4f, 0x50, 0x10, 0x00, 0x12, 0x14, 0x0a, 0x10, 0x49, 0x50, 0x54, 0x41, 0x42,
	0x4c, 0x45, 0x5f, 0x4e, 0x41, 0x54, 0x5f, 0x44, 0x52, 0x4f, 0x50, 0x10, 0x01, 0x12, 0x15, 0x0a,
	0x11, 0x54, 0x43, 0x50, 0x5f, 0x43, 0x4f, 0x4e, 0x4e, 0x45, 0x43, 0x54, 0x5f, 0x42, 0x41, 0x53,
	0x49, 0x43, 0x10, 0x02, 0x12, 0x14, 0x0a, 0x10, 0x54, 0x43, 0x50, 0x5f, 0x41, 0x43, 0x43, 0x45,
	0x50, 0x54, 0x5f, 0x42, 0x41, 0x53, 0x49, 0x43, 0x10, 0x03, 0x12, 0x13, 0x0a, 0x0f, 0x54, 0x43,
	0x50, 0x5f, 0x43, 0x4c, 0x4f, 0x53, 0x45, 0x5f, 0x42, 0x41, 0x53, 0x49, 0x43, 0x10, 0x04, 0x12,
	0x16, 0x0a, 0x12, 0x43, 0x4f, 0x4e, 0x4e, 0x54, 0x52, 0x41, 0x43, 0x4b, 0x5f, 0x41, 0x44, 0x44,
	0x5f, 0x44, 0x52, 0x4f, 0x50, 0x10, 0x05, 0x12, 0x10, 0x0a, 0x0c, 0x55, 0x4e, 0x4b, 0x4e, 0x4f,
	0x57, 0x4e, 0x5f, 0x44, 0x52, 0x4f, 0x50, 0x10, 0x06, 0x42, 0x27, 0x5a, 0x25, 0x67, 0x69, 0x74,
	0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x6d, 0x69, 0x63, 0x72, 0x6f, 0x73, 0x6f, 0x66,
	0x74, 0x2f, 0x72, 0x65, 0x74, 0x69, 0x6e, 0x61, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x75, 0x74, 0x69,
	0x6c, 0x73, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_pkg_utils_metadata_linux_proto_rawDescOnce sync.Once
	file_pkg_utils_metadata_linux_proto_rawDescData = file_pkg_utils_metadata_linux_proto_rawDesc
)

func file_pkg_utils_metadata_linux_proto_rawDescGZIP() []byte {
	file_pkg_utils_metadata_linux_proto_rawDescOnce.Do(func() {
		file_pkg_utils_metadata_linux_proto_rawDescData = protoimpl.X.CompressGZIP(file_pkg_utils_metadata_linux_proto_rawDescData)
	})
	return file_pkg_utils_metadata_linux_proto_rawDescData
}

var file_pkg_utils_metadata_linux_proto_enumTypes = make([]protoimpl.EnumInfo, 2)
var file_pkg_utils_metadata_linux_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_pkg_utils_metadata_linux_proto_goTypes = []any{
	(DNSType)(0),           // 0: utils.DNSType
	(DropReason)(0),        // 1: utils.DropReason
	(*RetinaMetadata)(nil), // 2: utils.RetinaMetadata
	nil,                    // 3: utils.RetinaMetadata.PreviouslyObservedTcpFlagsEntry
}
var file_pkg_utils_metadata_linux_proto_depIdxs = []int32{
	0, // 0: utils.RetinaMetadata.dns_type:type_name -> utils.DNSType
	1, // 1: utils.RetinaMetadata.drop_reason:type_name -> utils.DropReason
	3, // 2: utils.RetinaMetadata.previously_observed_tcp_flags:type_name -> utils.RetinaMetadata.PreviouslyObservedTcpFlagsEntry
	3, // [3:3] is the sub-list for method output_type
	3, // [3:3] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_pkg_utils_metadata_linux_proto_init() }
func file_pkg_utils_metadata_linux_proto_init() {
	if File_pkg_utils_metadata_linux_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_pkg_utils_metadata_linux_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*RetinaMetadata); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_pkg_utils_metadata_linux_proto_rawDesc,
			NumEnums:      2,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pkg_utils_metadata_linux_proto_goTypes,
		DependencyIndexes: file_pkg_utils_metadata_linux_proto_depIdxs,
		EnumInfos:         file_pkg_utils_metadata_linux_proto_enumTypes,
		MessageInfos:      file_pkg_utils_metadata_linux_proto_msgTypes,
	}.Build()
	File_pkg_utils_metadata_linux_proto = out.File
	file_pkg_utils_metadata_linux_proto_rawDesc = nil
	file_pkg_utils_metadata_linux_proto_goTypes = nil
	file_pkg_utils_metadata_linux_proto_depIdxs = nil
}
