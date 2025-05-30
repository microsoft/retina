package ebpfwindows

import (
	"fmt"

	metrics "github.com/microsoft/retina/pkg/metrics"
)

// DropMin numbers less than this are non-drop reason codes
var DropMin uint8 = 130

// DropInvalid is the Invalid packet reason.
var DropInvalid uint8 = 2

// Packet Monitor drop reason
var DropPacketMonitor uint8 = 220

// These values are shared with bpf/lib/common.h and api/v1/flow/flow.proto.
var dropErrors = map[uint8]string{
	0:   "Reason_Success",
	2:   "Reason_InvalidPacket",
	3:   "Reason_PlainText",
	4:   "Reason_InterfaceDecrypted",
	5:   "Reason_LbNoBackendSlot",
	6:   "Reason_LbNoBackend",
	7:   "Reason_LbReverseNatUpdate",
	8:   "Resaon_LbReverseNatStale",
	9:   "Reason_FragmentedPacket",
	10:  "Reason_FragmentedPacketUpdated",
	11:  "Reason_MissedCustomCall",
	132: "DropReason_InvalidSIP",
	133: "DropReason_Policy",
	134: "DropReason_Invalid",
	135: "DropReason_CTInvalidHdr",
	136: "DropReason_FragNeeded",
	137: "DropReason_CTUnknownProto",
	138: "DropReason_UnknownL3",
	139: "DropReason_MissedTailCall",
	140: "DropReason_WriteError",
	141: "DropReason_UnknownL4",
	142: "DropReason_UnknownICMPCode",
	143: "DropReason_UnknownICMPType",
	144: "DropReason_UnknownICMP6Code",
	145: "DropReason_UnknownICMP6Type",
	146: "DropReason_UnknownICMP6Type",
	147: "DropReason_NoTunnelKey",
	148: "DropReason_Unknown",
	149: "DropReason_Unknown",
	150: "DropReason_UnknownTarget",
	151: "DropReason_Unroutable",
	152: "DropReason_Unknown",
	153: "DropReason_CSUM_L3",
	154: "DropReason_CSUM_L4",
	155: "DropReason_CTCreateFailed",
	156: "DropReason_InvalidExthdr",
	157: "DropReason_FragNoSupport",
	158: "DropReason_NoService",
	159: "DropReason_UnsuppServiceProto",
	160: "DropReason_NoTunnelEndpoint",
	161: "DropReason_NAT46X64Disabled",
	162: "DropReason_EDTHorizon",
	163: "DropReason_UnknownCT",
	164: "DropReason_HostUnreachable",
	165: "DropReason_NoConfig",
	166: "DropReason_UnsupportedL2",
	167: "DropReason_NatNoMapping",
	168: "DropReason_NatUnsuppProto",
	169: "DropReason_NoFIB",
	170: "DropReason_EncapProhibited",
	171: "DropReason_InvalidIdentity",
	172: "DropReason_UnknownSender",
	173: "DropReason_NatNotNeeded",
	174: "DropReason_IsClusterIP",
	175: "DropReason_FragNotFound",
	176: "DropReason_ForbiddenICMP6",
	177: "DropReason_NotInSrcRange",
	178: "DropReason_ProxyLookupFailed",
	179: "DropReason_ProxySetFailed",
	180: "DropReason_ProxyUnknownProto",
	181: "DropReason_PolicyDeny",
	182: "DropReason_VlanFiltered",
	183: "DropReason_InvalidVNI",
	184: "DropReason_InvalidTCBuffer",
	185: "DropReason_NoSID",
	186: "DropReason_MissingSRv6State",
	187: "DropReason_NAT46",
	188: "DropReason_NAT64",
	189: "DropReason_PolicyAuthRequired",
	190: "DropReason_CTNoMapFound",
	191: "DropReason_SNATNoMapFound",
	192: "DropReason_InvalidClusterID",
	193: "DropReason_DSR_ENCAP_UNSUPP_PROTO",
	194: "DropReason_NoEgressGateway",
	195: "DropReason_UnencryptedTraffic",
	196: "DropReason_TTLExceeded",
	197: "DropReason_NoNodeID",
	198: "DropReason_RateLimited",
	199: "DropReason_IGMPHandled",
	200: "DropReason_IGMPSubscribed",
	201: "DropReason_MulticastHandled",
	202: "DropReason_HostNotReady",
	203: "DropReason_EpNotReady",
	220: "DropReason_PacketMonitor",
}

// Keep in sync with __id_for_file in bpf/lib/source_info.h.
var files = map[uint8]string{
	// source files from bpf/
	1: "bpf_host.c",
	2: "bpf_lxc.c",
	3: "bpf_overlay.c",
	4: "bpf_xdp.c",
	5: "bpf_sock.c",
	6: "bpf_network.c",

	// header files from bpf/lib/
	101: "arp.h",
	102: "drop.h",
	103: "srv6.h",
	104: "icmp6.h",
	105: "nodeport.h",
	106: "lb.h",
	107: "mcast.h",
	108: "ipv4.h",
	109: "conntrack.h",
	110: "l3.h",
	111: "trace.h",
	112: "encap.h",
	113: "encrypt.h",
}

// BPFFileName returns the file name for the given BPF file id.
func BPFFileName(id uint8) string {
	if name, ok := files[id]; ok {
		return name
	}
	return fmt.Sprintf("unknown(%d)", id)
}

func extendedReason(extError uint32) string {
	if extError == 0 {
		return ""
	}

	// Check if the extended error is a known drop reason
	dropReason := metrics.GetDropReason(extError)
	return dropReason.String()
}

func DropReasonExt(reason uint8, extError uint32) string {
	var ext string
	if err, ok := dropErrors[reason]; ok {
		if ext = extendedReason(extError); ext == "" {
			return err
		}
		return err + ", " + ext
	}
	return fmt.Sprintf("%d, %d", reason, extError)
}

// DropReason prints the drop reason in a human readable string
func DropReason(reason uint8) string {
	return DropReasonExt(reason, uint32(0))
}
