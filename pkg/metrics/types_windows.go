package metrics

import (
	"fmt"

	"github.com/cilium/cilium/api/v1/flow"
)

func GetDropTypeFlowDropReason(dr flow.DropReason) string {
	return dr.String()
}

func GetDropReason(reason uint32) DropReasonType {
	if v, ok := dropReasons[reason]; ok {
		return v.Reason
	}
	return Drop_Unknown
}

func (d DropReasonType) String() string {
	if v, ok := dropReasons[uint32(d)]; ok {
		return v.String
	}
	return fmt.Sprintf("UnknownDropReason(%d)", d)
}

type DropReasonWin struct {
	Reason DropReasonType
	String string
}

var dropReasons = map[uint32]DropReasonWin{
	0:  {Drop_Unknown, "Drop_Unknown"},
	1:  {Drop_InvalidData, "Drop_InvalidData"},
	2:  {Drop_InvalidPacket, "Drop_InvalidPacket"},
	3:  {Drop_Resources, "Drop_Resources"},
	4:  {Drop_NotReady, "Drop_NotReady"},
	5:  {Drop_Disconnected, "Drop_Disconnected"},
	6:  {Drop_NotAccepted, "Drop_NotAccepted"},
	7:  {Drop_Busy, "Drop_Busy"},
	8:  {Drop_Filtered, "Drop_Filtered"},
	9:  {Drop_FilteredVLAN, "Drop_FilteredVLAN"},
	10: {Drop_UnauthorizedVLAN, "Drop_UnauthorizedVLAN"},
	11: {Drop_UnauthorizedMAC, "Drop_UnauthorizedMAC"},
	12: {Drop_FailedSecurityPolicy, "Drop_FailedSecurityPolicy"},
	13: {Drop_FailedPvlanSetting, "Drop_FailedPvlanSetting"},
	14: {Drop_Qos, "Drop_Qos"},
	15: {Drop_Ipsec, "Drop_Ipsec"},
	16: {Drop_MacSpoofing, "Drop_MacSpoofing"},
	17: {Drop_DhcpGuard, "Drop_DhcpGuard"},
	18: {Drop_RouterGuard, "Drop_RouterGuard"},
	19: {Drop_BridgeReserved, "Drop_BridgeReserved"},
	20: {Drop_VirtualSubnetId, "Drop_VirtualSubnetId"},
	21: {Drop_RequiredExtensionMissing, "Drop_RequiredExtensionMissing"},
	22: {Drop_InvalidConfig, "Drop_InvalidConfig"},
	23: {Drop_MTUMismatch, "Drop_MTUMismatch"},
	24: {Drop_NativeFwdingReq, "Drop_NativeFwdingReq"},
	25: {Drop_InvalidVlanFormat, "Drop_InvalidVlanFormat"},
	26: {Drop_InvalidDestMac, "Drop_InvalidDestMac"},
	27: {Drop_InvalidSourceMac, "Drop_InvalidSourceMac"},
	28: {Drop_InvalidFirstNBTooSmall, "Drop_InvalidFirstNBTooSmall"},
	29: {Drop_Wnv, "Drop_Wnv"},
	30: {Drop_StormLimit, "Drop_StormLimit"},
	31: {Drop_InjectedIcmp, "Drop_InjectedIcmp"},
	32: {Drop_FailedDestinationListUpdate, "Drop_FailedDestinationListUpdate"},
	33: {Drop_NicDisabled, "Drop_NicDisabled"},
	34: {Drop_FailedPacketFilter, "Drop_FailedPacketFilter"},
	35: {Drop_SwitchDataFlowDisabled, "Drop_SwitchDataFlowDisabled"},
	36: {Drop_FilteredIsolationUntagged, "Drop_FilteredIsolationUntagged"},
	37: {Drop_InvalidPDQueue, "Drop_InvalidPDQueue"},
	38: {Drop_LowPower, "Drop_LowPower"},

	//
	// Generalerrors
	//
	201: {Drop_Pause, "Drop_Pause"},
	202: {Drop_Reset, "Drop_Reset"},
	203: {Drop_SendAborted, "Drop_SendAborted"},
	204: {Drop_ProtocolNotBound, "Drop_ProtocolNotBound"},
	205: {Drop_Failure, "Drop_Failure"},
	206: {Drop_InvalidLength, "Drop_InvalidLength"},
	207: {Drop_HostOutOfMemory, "Drop_HostOutOfMemory"},
	208: {Drop_FrameTooLong, "Drop_FrameTooLong"},
	209: {Drop_FrameTooShort, "Drop_FrameTooShort"},
	210: {Drop_FrameLengthError, "Drop_FrameLengthError"},
	211: {Drop_CrcError, "Drop_CrcError"},
	212: {Drop_BadFrameChecksum, "Drop_BadFrameChecksum"},
	213: {Drop_FcsError, "Drop_FcsError"},
	214: {Drop_SymbolError, "Drop_SymbolError"},
	215: {Drop_HeadQTimeout, "Drop_HeadQTimeout"},
	216: {Drop_StalledDiscard, "Drop_StalledDiscard"},
	217: {Drop_RxQFull, "Drop_RxQFull"},
	218: {Drop_PhysLayerError, "Drop_PhysLayerError"},
	219: {Drop_DmaError, "Drop_DmaError"},
	220: {Drop_FirmwareError, "Drop_FirmwareError"},
	221: {Drop_DecryptionFailed, "Drop_DecryptionFailed"},
	222: {Drop_BadSignature, "Drop_BadSignature"},
	223: {Drop_CoalescingError, "Drop_CoalescingError"},
	225: {Drop_VlanSpoofing, "Drop_VlanSpoofing"},
	226: {Drop_UnallowedEtherType, "Drop_UnallowedEtherType"},
	227: {Drop_VportDown, "Drop_VportDown"},
	228: {Drop_SteeringMismatch, "Drop_SteeringMismatch"},

	//
	// NetVscerrors
	//
	401: {Drop_MicroportError, "Drop_MicroportError"},
	402: {Drop_VfNotReady, "Drop_VfNotReady"},
	403: {Drop_MicroportNotReady, "Drop_MicroportNotReady"},
	404: {Drop_VMBusError, "Drop_VMBusError"},

	//
	// TcpipFLerrors
	//
	601: {Drop_FL_LoopbackPacket, "Drop_FL_LoopbackPacket"},
	602: {Drop_FL_InvalidSnapHeader, "Drop_FL_InvalidSnapHeader"},
	603: {Drop_FL_InvalidEthernetType, "Drop_FL_InvalidEthernetType"},
	604: {Drop_FL_InvalidPacketLength, "Drop_FL_InvalidPacketLength"},
	605: {Drop_FL_HeaderNotContiguous, "Drop_FL_HeaderNotContiguous"},
	606: {Drop_FL_InvalidDestinationType, "Drop_FL_InvalidDestinationType"},
	607: {Drop_FL_InterfaceNotReady, "Drop_FL_InterfaceNotReady"},
	608: {Drop_FL_ProviderNotReady, "Drop_FL_ProviderNotReady"},
	609: {Drop_FL_InvalidLsoInfo, "Drop_FL_InvalidLsoInfo"},
	610: {Drop_FL_InvalidUsoInfo, "Drop_FL_InvalidUsoInfo"},
	611: {Drop_FL_InvalidMedium, "Drop_FL_InvalidMedium"},
	612: {Drop_FL_InvalidArpHeader, "Drop_FL_InvalidArpHeader"},
	613: {Drop_FL_NoClientInterface, "Drop_FL_NoClientInterface"},
	614: {Drop_FL_TooManyNetBuffers, "Drop_FL_TooManyNetBuffers"},
	615: {Drop_FL_FlsNpiClientDrop, "Drop_FL_FlsNpiClientDrop"},

	//
	// VFPerrors
	//
	701: {Drop_ArpGuard, "Drop_ArpGuard"},
	702: {Drop_ArpLimiter, "Drop_ArpLimiter"},
	703: {Drop_DhcpLimiter, "Drop_DhcpLimiter"},
	704: {Drop_BlockBroadcast, "Drop_BlockBroadcast"},
	705: {Drop_BlockNonIp, "Drop_BlockNonIp"},
	706: {Drop_ArpFilter, "Drop_ArpFilter"},
	707: {Drop_Ipv4Guard, "Drop_Ipv4Guard"},
	708: {Drop_Ipv6Guard, "Drop_Ipv6Guard"},
	709: {Drop_MacGuard, "Drop_MacGuard"},
	710: {Drop_BroadcastNoDestinations, "Drop_BroadcastNoDestinations"},
	711: {Drop_UnicastNoDestination, "Drop_UnicastNoDestination"},
	712: {Drop_UnicastPortNotReady, "Drop_UnicastPortNotReady"},
	713: {Drop_SwitchCallbackFailed, "Drop_SwitchCallbackFailed"},
	714: {Drop_Icmpv6Limiter, "Drop_Icmpv6Limiter"},
	715: {Drop_Intercept, "Drop_Intercept"},
	716: {Drop_InterceptBlock, "Drop_InterceptBlock"},
	717: {Drop_NDPGuard, "Drop_NDPGuard"},
	718: {Drop_PortBlocked, "Drop_PortBlocked"},
	719: {Drop_NicSuspended, "Drop_NicSuspended"},

	//
	// TcpipNLerrors
	//
	901: {Drop_NL_BadSourceAddress, "Drop_NL_BadSourceAddress"},
	902: {Drop_NL_NotLocallyDestined, "Drop_NL_NotLocallyDestined"},
	903: {Drop_NL_ProtocolUnreachable, "Drop_NL_ProtocolUnreachable"},
	904: {Drop_NL_PortUnreachable, "Drop_NL_PortUnreachable"},
	905: {Drop_NL_BadLength, "Drop_NL_BadLength"},
	906: {Drop_NL_MalformedHeader, "Drop_NL_MalformedHeader"},
	907: {Drop_NL_NoRoute, "Drop_NL_NoRoute"},
	908: {Drop_NL_BeyondScope, "Drop_NL_BeyondScope"},
	909: {Drop_NL_InspectionDrop, "Drop_NL_InspectionDrop"},
	910: {Drop_NL_TooManyDecapsulations, "Drop_NL_TooManyDecapsulations"},
	911: {Drop_NL_AdministrativelyProhibited, "Drop_NL_AdministrativelyProhibited"},
	912: {Drop_NL_BadChecksum, "Drop_NL_BadChecksum"},
	913: {Drop_NL_ReceivePathMax, "Drop_NL_ReceivePathMax"},
	914: {Drop_NL_HopLimitExceeded, "Drop_NL_HopLimitExceeded"},
	915: {Drop_NL_AddressUnreachable, "Drop_NL_AddressUnreachable"},
	916: {Drop_NL_RscPacket, "Drop_NL_RscPacket"},
	917: {Drop_NL_ForwardPathMax, "Drop_NL_ForwardPathMax"},
	918: {Drop_NL_ArbitrationUnhandled, "Drop_NL_ArbitrationUnhandled"},
	919: {Drop_NL_InspectionAbsorb, "Drop_NL_InspectionAbsorb"},
	920: {Drop_NL_DontFragmentMtuExceeded, "Drop_NL_DontFragmentMtuExceeded"},
	921: {Drop_NL_BufferLengthExceeded, "Drop_NL_BufferLengthExceeded"},
	922: {Drop_NL_AddressResolutionTimeout, "Drop_NL_AddressResolutionTimeout"},
	923: {Drop_NL_AddressResolutionFailure, "Drop_NL_AddressResolutionFailure"},
	924: {Drop_NL_IpsecFailure, "Drop_NL_IpsecFailure"},
	925: {Drop_NL_ExtensionHeadersFailure, "Drop_NL_ExtensionHeadersFailure"},
	926: {Drop_NL_IpsnpiClientDrop, "Drop_NL_IpsnpiClientDrop"},
	927: {Drop_NL_UnsupportedOffload, "Drop_NL_UnsupportedOffload"},
	928: {Drop_NL_RoutingFailure, "Drop_NL_RoutingFailure"},
	929: {Drop_NL_AncillaryDataFailure, "Drop_NL_AncillaryDataFailure"},
	930: {Drop_NL_RawDataFailure, "Drop_NL_RawDataFailure"},
	931: {Drop_NL_SessionStateFailure, "Drop_NL_SessionStateFailure"},
	932: {Drop_NL_IpsnpiModifiedButNotForwarded, "Drop_NL_IpsnpiModifiedButNotForwarded"},
	933: {Drop_NL_IpsnpiNoNextHop, "Drop_NL_IpsnpiNoNextHop"},
	934: {Drop_NL_IpsnpiNoCompartment, "Drop_NL_IpsnpiNoCompartment"},
	935: {Drop_NL_IpsnpiNoInterface, "Drop_NL_IpsnpiNoInterface"},
	936: {Drop_NL_IpsnpiNoSubInterface, "Drop_NL_IpsnpiNoSubInterface"},
	937: {Drop_NL_IpsnpiInterfaceDisabled, "Drop_NL_IpsnpiInterfaceDisabled"},
	938: {Drop_NL_IpsnpiSegmentationFailed, "Drop_NL_IpsnpiSegmentationFailed"},
	939: {Drop_NL_IpsnpiNoEthernetHeader, "Drop_NL_IpsnpiNoEthernetHeader"},
	940: {Drop_NL_IpsnpiUnexpectedFragment, "Drop_NL_IpsnpiUnexpectedFragment"},
	941: {Drop_NL_IpsnpiUnsupportedInterfaceType, "Drop_NL_IpsnpiUnsupportedInterfaceType"},
	942: {Drop_NL_IpsnpiInvalidLsoInfo, "Drop_NL_IpsnpiInvalidLsoInfo"},
	943: {Drop_NL_IpsnpiInvalidUsoInfo, "Drop_NL_IpsnpiInvalidUsoInfo"},
	944: {Drop_NL_InternalError, "Drop_NL_InternalError"},
	945: {Drop_NL_AdministrativelyConfigured, "Drop_NL_AdministrativelyConfigured"},
	946: {Drop_NL_BadOption, "Drop_NL_BadOption"},
	947: {Drop_NL_LoopbackDisallowed, "Drop_NL_LoopbackDisallowed"},
	948: {Drop_NL_SmallerScope, "Drop_NL_SmallerScope"},
	949: {Drop_NL_QueueFull, "Drop_NL_QueueFull"},
	950: {Drop_NL_InterfaceDisabled, "Drop_NL_InterfaceDisabled"},

	951: {Drop_NL_IcmpGeneric, "Drop_NL_IcmpGeneric"},
	952: {Drop_NL_IcmpTruncatedHeader, "Drop_NL_IcmpTruncatedHeader"},
	953: {Drop_NL_IcmpInvalidChecksum, "Drop_NL_IcmpInvalidChecksum"},
	954: {Drop_NL_IcmpInspection, "Drop_NL_IcmpInspection"},
	955: {Drop_NL_IcmpNeighborDiscoveryLoopback, "Drop_NL_IcmpNeighborDiscoveryLoopback"},
	956: {Drop_NL_IcmpUnknownType, "Drop_NL_IcmpUnknownType"},
	957: {Drop_NL_IcmpTruncatedIpHeader, "Drop_NL_IcmpTruncatedIpHeader"},
	958: {Drop_NL_IcmpOversizedIpHeader, "Drop_NL_IcmpOversizedIpHeader"},
	959: {Drop_NL_IcmpNoHandler, "Drop_NL_IcmpNoHandler"},
	960: {Drop_NL_IcmpRespondingToError, "Drop_NL_IcmpRespondingToError"},
	961: {Drop_NL_IcmpInvalidSource, "Drop_NL_IcmpInvalidSource"},
	962: {Drop_NL_IcmpInterfaceRateLimit, "Drop_NL_IcmpInterfaceRateLimit"},
	963: {Drop_NL_IcmpPathRateLimit, "Drop_NL_IcmpPathRateLimit"},
	964: {Drop_NL_IcmpNoRoute, "Drop_NL_IcmpNoRoute"},
	965: {Drop_NL_IcmpMatchingRequestNotFound, "Drop_NL_IcmpMatchingRequestNotFound"},
	966: {Drop_NL_IcmpBufferTooSmall, "Drop_NL_IcmpBufferTooSmall"},
	967: {Drop_NL_IcmpAncillaryDataQuery, "Drop_NL_IcmpAncillaryDataQuery"},
	968: {Drop_NL_IcmpIncorrectHopLimit, "Drop_NL_IcmpIncorrectHopLimit"},
	969: {Drop_NL_IcmpUnknownCode, "Drop_NL_IcmpUnknownCode"},
	970: {Drop_NL_IcmpSourceNotLinkLocal, "Drop_NL_IcmpSourceNotLinkLocal"},
	971: {Drop_NL_IcmpTruncatedNdHeader, "Drop_NL_IcmpTruncatedNdHeader"},
	972: {Drop_NL_IcmpInvalidNdOptSourceLinkAddr, "Drop_NL_IcmpInvalidNdOptSourceLinkAddr"},
	973: {Drop_NL_IcmpInvalidNdOptMtu, "Drop_NL_IcmpInvalidNdOptMtu"},
	974: {Drop_NL_IcmpInvalidNdOptPrefixInformation, "Drop_NL_IcmpInvalidNdOptPrefixInformation"},
	975: {Drop_NL_IcmpInvalidNdOptRouteInformation, "Drop_NL_IcmpInvalidNdOptRouteInformation"},
	976: {Drop_NL_IcmpInvalidNdOptRdnss, "Drop_NL_IcmpInvalidNdOptRdnss"},
	977: {Drop_NL_IcmpInvalidNdOptDnssl, "Drop_NL_IcmpInvalidNdOptDnssl"},
	978: {Drop_NL_IcmpPacketParsingFailure, "Drop_NL_IcmpPacketParsingFailure"},
	979: {Drop_NL_IcmpDisallowed, "Drop_NL_IcmpDisallowed"},
	980: {Drop_NL_IcmpInvalidRouterAdvertisement, "Drop_NL_IcmpInvalidRouterAdvertisement"},
	981: {Drop_NL_IcmpSourceFromDifferentLink, "Drop_NL_IcmpSourceFromDifferentLink"},
	982: {Drop_NL_IcmpInvalidRedirectDestinationOrTarget, "Drop_NL_IcmpInvalidRedirectDestinationOrTarget"},
	983: {Drop_NL_IcmpInvalidNdTarget, "Drop_NL_IcmpInvalidNdTarget"},
	984: {Drop_NL_IcmpNaMulticastAndSolicited, "Drop_NL_IcmpNaMulticastAndSolicited"},
	985: {Drop_NL_IcmpNdLinkLayerAddressIsLocal, "Drop_NL_IcmpNdLinkLayerAddressIsLocal"},
	986: {Drop_NL_IcmpDuplicateEchoRequest, "Drop_NL_IcmpDuplicateEchoRequest"},
	987: {Drop_NL_IcmpNotAPotentialRouter, "Drop_NL_IcmpNotAPotentialRouter"},
	988: {Drop_NL_IcmpInvalidMldQuery, "Drop_NL_IcmpInvalidMldQuery"},
	989: {Drop_NL_IcmpInvalidMldReport, "Drop_NL_IcmpInvalidMldReport"},
	990: {Drop_NL_IcmpLocallySourcedMldReport, "Drop_NL_IcmpLocallySourcedMldReport"},
	991: {Drop_NL_IcmpNotLocallyDestined, "Drop_NL_IcmpNotLocallyDestined"},

	992: {Drop_NL_ArpInvalidSource, "Drop_NL_ArpInvalidSource"},
	993: {Drop_NL_ArpInvalidTarget, "Drop_NL_ArpInvalidTarget"},
	994: {Drop_NL_ArpDlSourceIsLocal, "Drop_NL_ArpDlSourceIsLocal"},
	995: {Drop_NL_ArpNotLocallyDestined, "Drop_NL_ArpNotLocallyDestined"},

	996: {Drop_NL_NlClientDiscard, "Drop_NL_NlClientDiscard"},

	997: {Drop_NL_IpsnpiUroSegmentSizeExceedsMtu, "Drop_NL_IpsnpiUroSegmentSizeExceedsMtu"},

	998:  {Drop_NL_IcmpFragmentedPacket, "Drop_NL_IcmpFragmentedPacket"},
	999:  {Drop_NL_FirstFragmentIncomplete, "Drop_NL_FirstFragmentIncomplete"},
	1000: {Drop_NL_SourceViolation, "Drop_NL_SourceViolation"},
	1001: {Drop_NL_IcmpJumbogram, "Drop_NL_IcmpJumbogram"},
	1002: {Drop_NL_SwUsoFailure, "Drop_NL_SwUsoFailure"},

	//
	// INETdiscardreasons
	//
	1200: {Drop_INET_SourceUnspecified, "Drop_INET_SourceUnspecified"},
	1201: {Drop_INET_DestinationMulticast, "Drop_INET_DestinationMulticast"},
	1202: {Drop_INET_HeaderInvalid, "Drop_INET_HeaderInvalid"},
	1203: {Drop_INET_ChecksumInvalid, "Drop_INET_ChecksumInvalid"},
	1204: {Drop_INET_EndpointNotFound, "Drop_INET_EndpointNotFound"},
	1205: {Drop_INET_ConnectedPath, "Drop_INET_ConnectedPath"},
	1206: {Drop_INET_SessionState, "Drop_INET_SessionState"},
	1207: {Drop_INET_ReceiveInspection, "Drop_INET_ReceiveInspection"},
	1208: {Drop_INET_AckInvalid, "Drop_INET_AckInvalid"},
	1209: {Drop_INET_ExpectedSyn, "Drop_INET_ExpectedSyn"},
	1210: {Drop_INET_Rst, "Drop_INET_Rst"},
	1211: {Drop_INET_SynRcvdSyn, "Drop_INET_SynRcvdSyn"},
	1212: {Drop_INET_SimultaneousConnect, "Drop_INET_SimultaneousConnect"},
	1213: {Drop_INET_PawsFailed, "Drop_INET_PawsFailed"},
	1214: {Drop_INET_LandAttack, "Drop_INET_LandAttack"},
	1215: {Drop_INET_MissedReset, "Drop_INET_MissedReset"},
	1216: {Drop_INET_OutsideWindow, "Drop_INET_OutsideWindow"},
	1217: {Drop_INET_DuplicateSegment, "Drop_INET_DuplicateSegment"},
	1218: {Drop_INET_ClosedWindow, "Drop_INET_ClosedWindow"},
	1219: {Drop_INET_TcbRemoved, "Drop_INET_TcbRemoved"},
	1220: {Drop_INET_FinWait2, "Drop_INET_FinWait2"},
	1221: {Drop_INET_ReassemblyConflict, "Drop_INET_ReassemblyConflict"},
	1222: {Drop_INET_FinReceived, "Drop_INET_FinReceived"},
	1223: {Drop_INET_ListenerInvalidFlags, "Drop_INET_ListenerInvalidFlags"},
	1224: {Drop_INET_TcbNotInTcbTable, "Drop_INET_TcbNotInTcbTable"},
	1225: {Drop_INET_TimeWaitTcbReceivedRstOutsideWindow, "Drop_INET_TimeWaitTcbReceivedRstOutsideWindow"},
	1226: {Drop_INET_TimeWaitTcbSynAndOtherFlags, "Drop_INET_TimeWaitTcbSynAndOtherFlags"},
	1227: {Drop_INET_TimeWaitTcb, "Drop_INET_TimeWaitTcb"},
	1228: {Drop_INET_SynAckWithFastopenCookieRequest, "Drop_INET_SynAckWithFastopenCookieRequest"},
	1229: {Drop_INET_PauseAccept, "Drop_INET_PauseAccept"},
	1230: {Drop_INET_SynAttack, "Drop_INET_SynAttack"},
	1231: {Drop_INET_AcceptInspection, "Drop_INET_AcceptInspection"},
	1232: {Drop_INET_AcceptRedirection, "Drop_INET_AcceptRedirection"},

	//
	// SlbmuxError
	//
	1301: {Drop_SlbMux_ParsingFailure, "Drop_SlbMux_ParsingFailure"},
	1302: {Drop_SlbMux_FirstFragmentMiss, "Drop_SlbMux_FirstFragmentMiss"},
	1303: {Drop_SlbMux_ICMPErrorPayloadValidationFailure, "Drop_SlbMux_ICMPErrorPayloadValidationFailure"},
	1304: {Drop_SlbMux_ICMPErrorPacketMatchNoSession, "Drop_SlbMux_ICMPErrorPacketMatchNoSession"},
	1305: {Drop_SlbMux_ExternalHairpinNexthopLookupFailure, "Drop_SlbMux_ExternalHairpinNexthopLookupFailure"},
	1306: {Drop_SlbMux_NoMatchingStaticMapping, "Drop_SlbMux_NoMatchingStaticMapping"},
	1307: {Drop_SlbMux_NexthopReferenceFailure, "Drop_SlbMux_NexthopReferenceFailure"},
	1308: {Drop_SlbMux_CloningFailure, "Drop_SlbMux_CloningFailure"},
	1309: {Drop_SlbMux_TranslationFailure, "Drop_SlbMux_TranslationFailure"},
	1310: {Drop_SlbMux_HopLimitExceeded, "Drop_SlbMux_HopLimitExceeded"},
	1311: {Drop_SlbMux_PacketBiggerThanMTU, "Drop_SlbMux_PacketBiggerThanMTU"},
	1312: {Drop_SlbMux_UnexpectedRouteLookupFailure, "Drop_SlbMux_UnexpectedRouteLookupFailure"},
	1313: {Drop_SlbMux_NoRoute, "Drop_SlbMux_NoRoute"},
	1314: {Drop_SlbMux_SessionCreationFailure, "Drop_SlbMux_SessionCreationFailure"},
	1315: {Drop_SlbMux_NexthopNotOverExternalInterface, "Drop_SlbMux_NexthopNotOverExternalInterface"},
	1316: {Drop_SlbMux_NexthopExternalInterfaceMissNATInstance, "Drop_SlbMux_NexthopExternalInterfaceMissNATInstance"},
	1317: {Drop_SlbMux_NATItselfCantBeInternalNexthop, "Drop_SlbMux_NATItselfCantBeInternalNexthop"},
	1318: {Drop_SlbMux_PacketRoutableInItsArrivalCompartment, "Drop_SlbMux_PacketRoutableInItsArrivalCompartment"},
	1319: {Drop_SlbMux_PacketTransportProtocolNotSupported, "Drop_SlbMux_PacketTransportProtocolNotSupported"},
	1320: {Drop_SlbMux_PacketIsDestinedLocally, "Drop_SlbMux_PacketIsDestinedLocally"},
	1321: {Drop_SlbMux_PacketDestinationIPandPortNotSubjectToNAT, "Drop_SlbMux_PacketDestinationIPandPortNotSubjectToNAT"},
	1322: {Drop_SlbMux_MuxReject, "Drop_SlbMux_MuxReject"},
	1323: {Drop_SlbMux_DipLookupFailure, "Drop_SlbMux_DipLookupFailure"},
	1324: {Drop_SlbMux_MuxEncapsulationFailure, "Drop_SlbMux_MuxEncapsulationFailure"},
	1325: {Drop_SlbMux_InvalidDiagPacketEncapType, "Drop_SlbMux_InvalidDiagPacketEncapType"},
	1326: {Drop_SlbMux_DiagPacketIsRedirect, "Drop_SlbMux_DiagPacketIsRedirect"},
	1327: {Drop_SlbMux_UnableToHandleRedirect, "Drop_SlbMux_UnableToHandleRedirect"},

	//
	// IpsecErrors
	//
	1401: {Drop_Ipsec_BadSpi, "Drop_Ipsec_BadSpi"},
	1402: {Drop_Ipsec_SALifetimeExpired, "Drop_Ipsec_SALifetimeExpired"},
	1403: {Drop_Ipsec_WrongSA, "Drop_Ipsec_WrongSA"},
	1404: {Drop_Ipsec_ReplayCheckFailed, "Drop_Ipsec_ReplayCheckFailed"},
	1405: {Drop_Ipsec_InvalidPacket, "Drop_Ipsec_InvalidPacket"},
	1406: {Drop_Ipsec_IntegrityCheckFailed, "Drop_Ipsec_IntegrityCheckFailed"},
	1407: {Drop_Ipsec_ClearTextDrop, "Drop_Ipsec_ClearTextDrop"},
	1408: {Drop_Ipsec_AuthFirewallDrop, "Drop_Ipsec_AuthFirewallDrop"},
	1409: {Drop_Ipsec_ThrottleDrop, "Drop_Ipsec_ThrottleDrop"},
	1410: {Drop_Ipsec_Dosp_Block, "Drop_Ipsec_Dosp_Block"},
	1411: {Drop_Ipsec_Dosp_ReceivedMulticast, "Drop_Ipsec_Dosp_ReceivedMulticast"},
	1412: {Drop_Ipsec_Dosp_InvalidPacket, "Drop_Ipsec_Dosp_InvalidPacket"},
	1413: {Drop_Ipsec_Dosp_StateLookupFailed, "Drop_Ipsec_Dosp_StateLookupFailed"},
	1414: {Drop_Ipsec_Dosp_MaxEntries, "Drop_Ipsec_Dosp_MaxEntries"},
	1415: {Drop_Ipsec_Dosp_KeymodNotAllowed, "Drop_Ipsec_Dosp_KeymodNotAllowed"},
	1416: {Drop_Ipsec_Dosp_MaxPerIpRateLimitQueues, "Drop_Ipsec_Dosp_MaxPerIpRateLimitQueues"},
	1417: {Drop_Ipsec_NoMemory, "Drop_Ipsec_NoMemory"},
	1418: {Drop_Ipsec_Unsuccessful, "Drop_Ipsec_Unsuccessful"},

	//
	// NetCxDropReasons
	//
	1501: {Drop_NetCx_NetPacketLayoutParseFailure, "Drop_NetCx_NetPacketLayoutParseFailure"},
	1502: {Drop_NetCx_SoftwareChecksumFailure, "Drop_NetCx_SoftwareChecksumFailure"},
	1503: {Drop_NetCx_NicQueueStop, "Drop_NetCx_NicQueueStop"},
	1504: {Drop_NetCx_InvalidNetBufferLength, "Drop_NetCx_InvalidNetBufferLength"},
	1505: {Drop_NetCx_LSOFailure, "Drop_NetCx_LSOFailure"},
	1506: {Drop_NetCx_USOFailure, "Drop_NetCx_USOFailure"},
	1507: {Drop_NetCx_BufferBounceFailureAndPacketIgnore, "Drop_NetCx_BufferBounceFailureAndPacketIgnore"},

	//
	// Httperrors3000-4000.
	// Thesemustbeinsyncwithcmd\resource.h
	//
	3000: {Drop_Http_Begin, "Drop_Http_Begin"},

	//
	// UlErrors
	//
	3001: {Drop_Http_UlError_Begin, "Drop_Http_UlError_Begin"},
	3002: {Drop_Http_UlError, "Drop_Http_UlError"},
	3003: {Drop_Http_UlErrorVerb, "Drop_Http_UlErrorVerb"},
	3004: {Drop_Http_UlErrorUrl, "Drop_Http_UlErrorUrl"},
	3005: {Drop_Http_UlErrorHeader, "Drop_Http_UlErrorHeader"},
	3006: {Drop_Http_UlErrorHost, "Drop_Http_UlErrorHost"},
	3007: {Drop_Http_UlErrorNum, "Drop_Http_UlErrorNum"},
	3008: {Drop_Http_UlErrorFieldLength, "Drop_Http_UlErrorFieldLength"},
	3009: {Drop_Http_UlErrorRequestLength, "Drop_Http_UlErrorRequestLength"},
	3010: {Drop_Http_UlErrorUnauthorized, "Drop_Http_UlErrorUnauthorized"},

	3011: {Drop_Http_UlErrorForbiddenUrl, "Drop_Http_UlErrorForbiddenUrl"},
	3012: {Drop_Http_UlErrorNotFound, "Drop_Http_UlErrorNotFound"},
	3013: {Drop_Http_UlErrorContentLength, "Drop_Http_UlErrorContentLength"},
	3014: {Drop_Http_UlErrorPreconditionFailed, "Drop_Http_UlErrorPreconditionFailed"},
	3015: {Drop_Http_UlErrorEntityTooLarge, "Drop_Http_UlErrorEntityTooLarge"},
	3016: {Drop_Http_UlErrorUrlLength, "Drop_Http_UlErrorUrlLength"},
	3017: {Drop_Http_UlErrorRangeNotSatisfiable, "Drop_Http_UlErrorRangeNotSatisfiable"},
	3018: {Drop_Http_UlErrorMisdirectedRequest, "Drop_Http_UlErrorMisdirectedRequest"},

	3019: {Drop_Http_UlErrorInternalServer, "Drop_Http_UlErrorInternalServer"},
	3020: {Drop_Http_UlErrorNotImplemented, "Drop_Http_UlErrorNotImplemented"},
	3021: {Drop_Http_UlErrorUnavailable, "Drop_Http_UlErrorUnavailable"},
	3022: {Drop_Http_UlErrorConnectionLimit, "Drop_Http_UlErrorConnectionLimit"},
	3023: {Drop_Http_UlErrorRapidFailProtection, "Drop_Http_UlErrorRapidFailProtection"},
	3024: {Drop_Http_UlErrorRequestQueueFull, "Drop_Http_UlErrorRequestQueueFull"},
	3025: {Drop_Http_UlErrorDisabledByAdmin, "Drop_Http_UlErrorDisabledByAdmin"},
	3026: {Drop_Http_UlErrorDisabledByApp, "Drop_Http_UlErrorDisabledByApp"},
	3027: {Drop_Http_UlErrorJobObjectFired, "Drop_Http_UlErrorJobObjectFired"},
	3028: {Drop_Http_UlErrorAppPoolBusy, "Drop_Http_UlErrorAppPoolBusy"},

	3029: {Drop_Http_UlErrorVersion, "Drop_Http_UlErrorVersion"},
	3030: {Drop_Http_UlError_End, "Drop_Http_UlError_End"},

	//
	// Stream-specificfaultcodes.
	//

	3400: {Drop_Http_UxDuoFaultBegin, "Drop_Http_UxDuoFaultBegin"},
	3401: {Drop_Http_UxDuoFaultUserAbort, "Drop_Http_UxDuoFaultUserAbort"},
	3402: {Drop_Http_UxDuoFaultCollection, "Drop_Http_UxDuoFaultCollection"},
	3403: {Drop_Http_UxDuoFaultClientResetStream, "Drop_Http_UxDuoFaultClientResetStream"},
	3404: {Drop_Http_UxDuoFaultMethodNotFound, "Drop_Http_UxDuoFaultMethodNotFound"},
	3405: {Drop_Http_UxDuoFaultSchemeMismatch, "Drop_Http_UxDuoFaultSchemeMismatch"},
	3406: {Drop_Http_UxDuoFaultSchemeNotFound, "Drop_Http_UxDuoFaultSchemeNotFound"},
	3407: {Drop_Http_UxDuoFaultDataAfterEnd, "Drop_Http_UxDuoFaultDataAfterEnd"},
	3408: {Drop_Http_UxDuoFaultPathNotFound, "Drop_Http_UxDuoFaultPathNotFound"},
	3409: {Drop_Http_UxDuoFaultHalfClosedLocal, "Drop_Http_UxDuoFaultHalfClosedLocal"},
	3410: {Drop_Http_UxDuoFaultIncompatibleAuth, "Drop_Http_UxDuoFaultIncompatibleAuth"},
	3411: {Drop_Http_UxDuoFaultDeprecated3, "Drop_Http_UxDuoFaultDeprecated3"},
	3412: {Drop_Http_UxDuoFaultClientCertBlocked, "Drop_Http_UxDuoFaultClientCertBlocked"},
	3413: {Drop_Http_UxDuoFaultHeaderNameEmpty, "Drop_Http_UxDuoFaultHeaderNameEmpty"},
	3414: {Drop_Http_UxDuoFaultIllegalSend, "Drop_Http_UxDuoFaultIllegalSend"},
	3415: {Drop_Http_UxDuoFaultPushUpperAttach, "Drop_Http_UxDuoFaultPushUpperAttach"},
	3416: {Drop_Http_UxDuoFaultStreamUpperAttach, "Drop_Http_UxDuoFaultStreamUpperAttach"},
	3417: {Drop_Http_UxDuoFaultActiveStreamLimit, "Drop_Http_UxDuoFaultActiveStreamLimit"},
	3418: {Drop_Http_UxDuoFaultAuthorityNotFound, "Drop_Http_UxDuoFaultAuthorityNotFound"},
	3419: {Drop_Http_UxDuoFaultUnexpectedTail, "Drop_Http_UxDuoFaultUnexpectedTail"},
	3420: {Drop_Http_UxDuoFaultTruncated, "Drop_Http_UxDuoFaultTruncated"},
	3421: {Drop_Http_UxDuoFaultResponseHold, "Drop_Http_UxDuoFaultResponseHold"},
	3422: {Drop_Http_UxDuoFaultRequestChunked, "Drop_Http_UxDuoFaultRequestChunked"},
	3423: {Drop_Http_UxDuoFaultRequestContentLength, "Drop_Http_UxDuoFaultRequestContentLength"},
	3424: {Drop_Http_UxDuoFaultResponseChunked, "Drop_Http_UxDuoFaultResponseChunked"},
	3425: {Drop_Http_UxDuoFaultResponseContentLength, "Drop_Http_UxDuoFaultResponseContentLength"},
	3426: {Drop_Http_UxDuoFaultResponseTransferEncoding, "Drop_Http_UxDuoFaultResponseTransferEncoding"},
	3427: {Drop_Http_UxDuoFaultResponseLine, "Drop_Http_UxDuoFaultResponseLine"},
	3428: {Drop_Http_UxDuoFaultResponseHeader, "Drop_Http_UxDuoFaultResponseHeader"},
	3429: {Drop_Http_UxDuoFaultConnect, "Drop_Http_UxDuoFaultConnect"},
	3430: {Drop_Http_UxDuoFaultChunkStart, "Drop_Http_UxDuoFaultChunkStart"},
	3431: {Drop_Http_UxDuoFaultChunkLength, "Drop_Http_UxDuoFaultChunkLength"},
	3432: {Drop_Http_UxDuoFaultChunkStop, "Drop_Http_UxDuoFaultChunkStop"},
	3433: {Drop_Http_UxDuoFaultHeadersAfterTrailers, "Drop_Http_UxDuoFaultHeadersAfterTrailers"},
	3434: {Drop_Http_UxDuoFaultHeadersAfterEnd, "Drop_Http_UxDuoFaultHeadersAfterEnd"},
	3435: {Drop_Http_UxDuoFaultEndlessTrailer, "Drop_Http_UxDuoFaultEndlessTrailer"},
	3436: {Drop_Http_UxDuoFaultTransferEncoding, "Drop_Http_UxDuoFaultTransferEncoding"},
	3437: {Drop_Http_UxDuoFaultMultipleTransferCodings, "Drop_Http_UxDuoFaultMultipleTransferCodings"},
	3438: {Drop_Http_UxDuoFaultPushBody, "Drop_Http_UxDuoFaultPushBody"},
	3439: {Drop_Http_UxDuoFaultStreamAbandoned, "Drop_Http_UxDuoFaultStreamAbandoned"},
	3440: {Drop_Http_UxDuoFaultMalformedHost, "Drop_Http_UxDuoFaultMalformedHost"},
	3441: {Drop_Http_UxDuoFaultDecompressionOverflow, "Drop_Http_UxDuoFaultDecompressionOverflow"},
	3442: {Drop_Http_UxDuoFaultIllegalHeaderName, "Drop_Http_UxDuoFaultIllegalHeaderName"},
	3443: {Drop_Http_UxDuoFaultIllegalHeaderValue, "Drop_Http_UxDuoFaultIllegalHeaderValue"},
	3444: {Drop_Http_UxDuoFaultConnHeaderDisallowed, "Drop_Http_UxDuoFaultConnHeaderDisallowed"},
	3445: {Drop_Http_UxDuoFaultConnHeaderMalformed, "Drop_Http_UxDuoFaultConnHeaderMalformed"},
	3446: {Drop_Http_UxDuoFaultCookieReassembly, "Drop_Http_UxDuoFaultCookieReassembly"},
	3447: {Drop_Http_UxDuoFaultStatusHeader, "Drop_Http_UxDuoFaultStatusHeader"},
	3448: {Drop_Http_UxDuoFaultSchemeDisallowed, "Drop_Http_UxDuoFaultSchemeDisallowed"},
	3449: {Drop_Http_UxDuoFaultPathDisallowed, "Drop_Http_UxDuoFaultPathDisallowed"},
	3450: {Drop_Http_UxDuoFaultPushHost, "Drop_Http_UxDuoFaultPushHost"},
	3451: {Drop_Http_UxDuoFaultGoawayReceived, "Drop_Http_UxDuoFaultGoawayReceived"},
	3452: {Drop_Http_UxDuoFaultAbortLegacyApp, "Drop_Http_UxDuoFaultAbortLegacyApp"},
	3453: {Drop_Http_UxDuoFaultUpgradeHeaderDisallowed, "Drop_Http_UxDuoFaultUpgradeHeaderDisallowed"},
	3454: {Drop_Http_UxDuoFaultResponseUpgradeHeader, "Drop_Http_UxDuoFaultResponseUpgradeHeader"},
	3455: {Drop_Http_UxDuoFaultKeepAliveHeaderDisallowed, "Drop_Http_UxDuoFaultKeepAliveHeaderDisallowed"},
	3456: {Drop_Http_UxDuoFaultResponseKeepAliveHeader, "Drop_Http_UxDuoFaultResponseKeepAliveHeader"},
	3457: {Drop_Http_UxDuoFaultProxyConnHeaderDisallowed, "Drop_Http_UxDuoFaultProxyConnHeaderDisallowed"},
	3458: {Drop_Http_UxDuoFaultResponseProxyConnHeader, "Drop_Http_UxDuoFaultResponseProxyConnHeader"},
	3459: {Drop_Http_UxDuoFaultConnectionGoingAway, "Drop_Http_UxDuoFaultConnectionGoingAway"},
	3460: {Drop_Http_UxDuoFaultTransferEncodingDisallowed, "Drop_Http_UxDuoFaultTransferEncodingDisallowed"},
	3461: {Drop_Http_UxDuoFaultContentLengthDisallowed, "Drop_Http_UxDuoFaultContentLengthDisallowed"},
	3462: {Drop_Http_UxDuoFaultTrailerDisallowed, "Drop_Http_UxDuoFaultTrailerDisallowed"},
	3463: {Drop_Http_UxDuoFaultEnd, "Drop_Http_UxDuoFaultEnd"},

	//
	// WSKlayerdrops
	//
	3600: {Drop_Http_ReceiveSuppressed, "Drop_Http_ReceiveSuppressed"},

	//
	// Http/SSLlayerdrops
	//
	3800: {Drop_Http_Generic, "Drop_Http_Generic"},
	3801: {Drop_Http_InvalidParameter, "Drop_Http_InvalidParameter"},
	3802: {Drop_Http_InsufficientResources, "Drop_Http_InsufficientResources"},
	3803: {Drop_Http_InvalidHandle, "Drop_Http_InvalidHandle"},
	3804: {Drop_Http_NotSupported, "Drop_Http_NotSupported"},
	3805: {Drop_Http_BadNetworkPath, "Drop_Http_BadNetworkPath"},
	3806: {Drop_Http_InternalError, "Drop_Http_InternalError"},
	3807: {Drop_Http_NoSuchPackage, "Drop_Http_NoSuchPackage"},
	3808: {Drop_Http_PrivilegeNotHeld, "Drop_Http_PrivilegeNotHeld"},
	3809: {Drop_Http_CannotImpersonate, "Drop_Http_CannotImpersonate"},
	3810: {Drop_Http_LogonFailure, "Drop_Http_LogonFailure"},
	3811: {Drop_Http_NoSuchLogonSession, "Drop_Http_NoSuchLogonSession"},
	3812: {Drop_Http_AccessDenied, "Drop_Http_AccessDenied"},
	3813: {Drop_Http_NoLogonServers, "Drop_Http_NoLogonServers"},
	3814: {Drop_Http_TimeDifferenceAtDc, "Drop_Http_TimeDifferenceAtDc"},

	4000: {Drop_Http_End, "Drop_Http_End"},
}

const (
	Drop_Unknown                     DropReasonType = 0
	Drop_InvalidData                 DropReasonType = 1
	Drop_InvalidPacket               DropReasonType = 2
	Drop_Resources                   DropReasonType = 3
	Drop_NotReady                    DropReasonType = 4
	Drop_Disconnected                DropReasonType = 5
	Drop_NotAccepted                 DropReasonType = 6
	Drop_Busy                        DropReasonType = 7
	Drop_Filtered                    DropReasonType = 8
	Drop_FilteredVLAN                DropReasonType = 9
	Drop_UnauthorizedVLAN            DropReasonType = 10
	Drop_UnauthorizedMAC             DropReasonType = 11
	Drop_FailedSecurityPolicy        DropReasonType = 12
	Drop_FailedPvlanSetting          DropReasonType = 13
	Drop_Qos                         DropReasonType = 14
	Drop_Ipsec                       DropReasonType = 15
	Drop_MacSpoofing                 DropReasonType = 16
	Drop_DhcpGuard                   DropReasonType = 17
	Drop_RouterGuard                 DropReasonType = 18
	Drop_BridgeReserved              DropReasonType = 19
	Drop_VirtualSubnetId             DropReasonType = 20
	Drop_RequiredExtensionMissing    DropReasonType = 21
	Drop_InvalidConfig               DropReasonType = 22
	Drop_MTUMismatch                 DropReasonType = 23
	Drop_NativeFwdingReq             DropReasonType = 24
	Drop_InvalidVlanFormat           DropReasonType = 25
	Drop_InvalidDestMac              DropReasonType = 26
	Drop_InvalidSourceMac            DropReasonType = 27
	Drop_InvalidFirstNBTooSmall      DropReasonType = 28
	Drop_Wnv                         DropReasonType = 29
	Drop_StormLimit                  DropReasonType = 30
	Drop_InjectedIcmp                DropReasonType = 31
	Drop_FailedDestinationListUpdate DropReasonType = 32
	Drop_NicDisabled                 DropReasonType = 33
	Drop_FailedPacketFilter          DropReasonType = 34
	Drop_SwitchDataFlowDisabled      DropReasonType = 35
	Drop_FilteredIsolationUntagged   DropReasonType = 36
	Drop_InvalidPDQueue              DropReasonType = 37
	Drop_LowPower                    DropReasonType = 38

	//
	// General errors
	//
	Drop_Pause              DropReasonType = 201
	Drop_Reset              DropReasonType = 202
	Drop_SendAborted        DropReasonType = 203
	Drop_ProtocolNotBound   DropReasonType = 204
	Drop_Failure            DropReasonType = 205
	Drop_InvalidLength      DropReasonType = 206
	Drop_HostOutOfMemory    DropReasonType = 207
	Drop_FrameTooLong       DropReasonType = 208
	Drop_FrameTooShort      DropReasonType = 209
	Drop_FrameLengthError   DropReasonType = 210
	Drop_CrcError           DropReasonType = 211
	Drop_BadFrameChecksum   DropReasonType = 212
	Drop_FcsError           DropReasonType = 213
	Drop_SymbolError        DropReasonType = 214
	Drop_HeadQTimeout       DropReasonType = 215
	Drop_StalledDiscard     DropReasonType = 216
	Drop_RxQFull            DropReasonType = 217
	Drop_PhysLayerError     DropReasonType = 218
	Drop_DmaError           DropReasonType = 219
	Drop_FirmwareError      DropReasonType = 220
	Drop_DecryptionFailed   DropReasonType = 221
	Drop_BadSignature       DropReasonType = 222
	Drop_CoalescingError    DropReasonType = 223
	Drop_VlanSpoofing       DropReasonType = 225
	Drop_UnallowedEtherType DropReasonType = 226
	Drop_VportDown          DropReasonType = 227
	Drop_SteeringMismatch   DropReasonType = 228

	//
	// NetVsc errors
	//
	Drop_MicroportError    DropReasonType = 401
	Drop_VfNotReady        DropReasonType = 402
	Drop_MicroportNotReady DropReasonType = 403
	Drop_VMBusError        DropReasonType = 404

	//
	// Tcpip FL errors
	//
	Drop_FL_LoopbackPacket         DropReasonType = 601
	Drop_FL_InvalidSnapHeader      DropReasonType = 602
	Drop_FL_InvalidEthernetType    DropReasonType = 603
	Drop_FL_InvalidPacketLength    DropReasonType = 604
	Drop_FL_HeaderNotContiguous    DropReasonType = 605
	Drop_FL_InvalidDestinationType DropReasonType = 606
	Drop_FL_InterfaceNotReady      DropReasonType = 607
	Drop_FL_ProviderNotReady       DropReasonType = 608
	Drop_FL_InvalidLsoInfo         DropReasonType = 609
	Drop_FL_InvalidUsoInfo         DropReasonType = 610
	Drop_FL_InvalidMedium          DropReasonType = 611
	Drop_FL_InvalidArpHeader       DropReasonType = 612
	Drop_FL_NoClientInterface      DropReasonType = 613
	Drop_FL_TooManyNetBuffers      DropReasonType = 614
	Drop_FL_FlsNpiClientDrop       DropReasonType = 615

	//
	// VFP errors
	//
	Drop_ArpGuard                DropReasonType = 701
	Drop_ArpLimiter              DropReasonType = 702
	Drop_DhcpLimiter             DropReasonType = 703
	Drop_BlockBroadcast          DropReasonType = 704
	Drop_BlockNonIp              DropReasonType = 705
	Drop_ArpFilter               DropReasonType = 706
	Drop_Ipv4Guard               DropReasonType = 707
	Drop_Ipv6Guard               DropReasonType = 708
	Drop_MacGuard                DropReasonType = 709
	Drop_BroadcastNoDestinations DropReasonType = 710
	Drop_UnicastNoDestination    DropReasonType = 711
	Drop_UnicastPortNotReady     DropReasonType = 712
	Drop_SwitchCallbackFailed    DropReasonType = 713
	Drop_Icmpv6Limiter           DropReasonType = 714
	Drop_Intercept               DropReasonType = 715
	Drop_InterceptBlock          DropReasonType = 716
	Drop_NDPGuard                DropReasonType = 717
	Drop_PortBlocked             DropReasonType = 718
	Drop_NicSuspended            DropReasonType = 719

	//
	// Tcpip NL errors
	//
	Drop_NL_BadSourceAddress               DropReasonType = 901
	Drop_NL_NotLocallyDestined             DropReasonType = 902
	Drop_NL_ProtocolUnreachable            DropReasonType = 903
	Drop_NL_PortUnreachable                DropReasonType = 904
	Drop_NL_BadLength                      DropReasonType = 905
	Drop_NL_MalformedHeader                DropReasonType = 906
	Drop_NL_NoRoute                        DropReasonType = 907
	Drop_NL_BeyondScope                    DropReasonType = 908
	Drop_NL_InspectionDrop                 DropReasonType = 909
	Drop_NL_TooManyDecapsulations          DropReasonType = 910
	Drop_NL_AdministrativelyProhibited     DropReasonType = 911
	Drop_NL_BadChecksum                    DropReasonType = 912
	Drop_NL_ReceivePathMax                 DropReasonType = 913
	Drop_NL_HopLimitExceeded               DropReasonType = 914
	Drop_NL_AddressUnreachable             DropReasonType = 915
	Drop_NL_RscPacket                      DropReasonType = 916
	Drop_NL_ForwardPathMax                 DropReasonType = 917
	Drop_NL_ArbitrationUnhandled           DropReasonType = 918
	Drop_NL_InspectionAbsorb               DropReasonType = 919
	Drop_NL_DontFragmentMtuExceeded        DropReasonType = 920
	Drop_NL_BufferLengthExceeded           DropReasonType = 921
	Drop_NL_AddressResolutionTimeout       DropReasonType = 922
	Drop_NL_AddressResolutionFailure       DropReasonType = 923
	Drop_NL_IpsecFailure                   DropReasonType = 924
	Drop_NL_ExtensionHeadersFailure        DropReasonType = 925
	Drop_NL_IpsnpiClientDrop               DropReasonType = 926
	Drop_NL_UnsupportedOffload             DropReasonType = 927
	Drop_NL_RoutingFailure                 DropReasonType = 928
	Drop_NL_AncillaryDataFailure           DropReasonType = 929
	Drop_NL_RawDataFailure                 DropReasonType = 930
	Drop_NL_SessionStateFailure            DropReasonType = 931
	Drop_NL_IpsnpiModifiedButNotForwarded  DropReasonType = 932
	Drop_NL_IpsnpiNoNextHop                DropReasonType = 933
	Drop_NL_IpsnpiNoCompartment            DropReasonType = 934
	Drop_NL_IpsnpiNoInterface              DropReasonType = 935
	Drop_NL_IpsnpiNoSubInterface           DropReasonType = 936
	Drop_NL_IpsnpiInterfaceDisabled        DropReasonType = 937
	Drop_NL_IpsnpiSegmentationFailed       DropReasonType = 938
	Drop_NL_IpsnpiNoEthernetHeader         DropReasonType = 939
	Drop_NL_IpsnpiUnexpectedFragment       DropReasonType = 940
	Drop_NL_IpsnpiUnsupportedInterfaceType DropReasonType = 941
	Drop_NL_IpsnpiInvalidLsoInfo           DropReasonType = 942
	Drop_NL_IpsnpiInvalidUsoInfo           DropReasonType = 943
	Drop_NL_InternalError                  DropReasonType = 944
	Drop_NL_AdministrativelyConfigured     DropReasonType = 945
	Drop_NL_BadOption                      DropReasonType = 946
	Drop_NL_LoopbackDisallowed             DropReasonType = 947
	Drop_NL_SmallerScope                   DropReasonType = 948
	Drop_NL_QueueFull                      DropReasonType = 949
	Drop_NL_InterfaceDisabled              DropReasonType = 950

	Drop_NL_IcmpGeneric                            DropReasonType = 951
	Drop_NL_IcmpTruncatedHeader                    DropReasonType = 952
	Drop_NL_IcmpInvalidChecksum                    DropReasonType = 953
	Drop_NL_IcmpInspection                         DropReasonType = 954
	Drop_NL_IcmpNeighborDiscoveryLoopback          DropReasonType = 955
	Drop_NL_IcmpUnknownType                        DropReasonType = 956
	Drop_NL_IcmpTruncatedIpHeader                  DropReasonType = 957
	Drop_NL_IcmpOversizedIpHeader                  DropReasonType = 958
	Drop_NL_IcmpNoHandler                          DropReasonType = 959
	Drop_NL_IcmpRespondingToError                  DropReasonType = 960
	Drop_NL_IcmpInvalidSource                      DropReasonType = 961
	Drop_NL_IcmpInterfaceRateLimit                 DropReasonType = 962
	Drop_NL_IcmpPathRateLimit                      DropReasonType = 963
	Drop_NL_IcmpNoRoute                            DropReasonType = 964
	Drop_NL_IcmpMatchingRequestNotFound            DropReasonType = 965
	Drop_NL_IcmpBufferTooSmall                     DropReasonType = 966
	Drop_NL_IcmpAncillaryDataQuery                 DropReasonType = 967
	Drop_NL_IcmpIncorrectHopLimit                  DropReasonType = 968
	Drop_NL_IcmpUnknownCode                        DropReasonType = 969
	Drop_NL_IcmpSourceNotLinkLocal                 DropReasonType = 970
	Drop_NL_IcmpTruncatedNdHeader                  DropReasonType = 971
	Drop_NL_IcmpInvalidNdOptSourceLinkAddr         DropReasonType = 972
	Drop_NL_IcmpInvalidNdOptMtu                    DropReasonType = 973
	Drop_NL_IcmpInvalidNdOptPrefixInformation      DropReasonType = 974
	Drop_NL_IcmpInvalidNdOptRouteInformation       DropReasonType = 975
	Drop_NL_IcmpInvalidNdOptRdnss                  DropReasonType = 976
	Drop_NL_IcmpInvalidNdOptDnssl                  DropReasonType = 977
	Drop_NL_IcmpPacketParsingFailure               DropReasonType = 978
	Drop_NL_IcmpDisallowed                         DropReasonType = 979
	Drop_NL_IcmpInvalidRouterAdvertisement         DropReasonType = 980
	Drop_NL_IcmpSourceFromDifferentLink            DropReasonType = 981
	Drop_NL_IcmpInvalidRedirectDestinationOrTarget DropReasonType = 982
	Drop_NL_IcmpInvalidNdTarget                    DropReasonType = 983
	Drop_NL_IcmpNaMulticastAndSolicited            DropReasonType = 984
	Drop_NL_IcmpNdLinkLayerAddressIsLocal          DropReasonType = 985
	Drop_NL_IcmpDuplicateEchoRequest               DropReasonType = 986
	Drop_NL_IcmpNotAPotentialRouter                DropReasonType = 987
	Drop_NL_IcmpInvalidMldQuery                    DropReasonType = 988
	Drop_NL_IcmpInvalidMldReport                   DropReasonType = 989
	Drop_NL_IcmpLocallySourcedMldReport            DropReasonType = 990
	Drop_NL_IcmpNotLocallyDestined                 DropReasonType = 991

	Drop_NL_ArpInvalidSource      DropReasonType = 992
	Drop_NL_ArpInvalidTarget      DropReasonType = 993
	Drop_NL_ArpDlSourceIsLocal    DropReasonType = 994
	Drop_NL_ArpNotLocallyDestined DropReasonType = 995

	Drop_NL_NlClientDiscard DropReasonType = 996

	Drop_NL_IpsnpiUroSegmentSizeExceedsMtu DropReasonType = 997

	Drop_NL_IcmpFragmentedPacket    DropReasonType = 998
	Drop_NL_FirstFragmentIncomplete DropReasonType = 999
	Drop_NL_SourceViolation         DropReasonType = 1000
	Drop_NL_IcmpJumbogram           DropReasonType = 1001
	Drop_NL_SwUsoFailure            DropReasonType = 1002

	//
	// INET discard reasons
	//
	Drop_INET_SourceUnspecified                   DropReasonType = 1200
	Drop_INET_DestinationMulticast                DropReasonType = 1201
	Drop_INET_HeaderInvalid                       DropReasonType = 1202
	Drop_INET_ChecksumInvalid                     DropReasonType = 1203
	Drop_INET_EndpointNotFound                    DropReasonType = 1204
	Drop_INET_ConnectedPath                       DropReasonType = 1205
	Drop_INET_SessionState                        DropReasonType = 1206
	Drop_INET_ReceiveInspection                   DropReasonType = 1207
	Drop_INET_AckInvalid                          DropReasonType = 1208
	Drop_INET_ExpectedSyn                         DropReasonType = 1209
	Drop_INET_Rst                                 DropReasonType = 1210
	Drop_INET_SynRcvdSyn                          DropReasonType = 1211
	Drop_INET_SimultaneousConnect                 DropReasonType = 1212
	Drop_INET_PawsFailed                          DropReasonType = 1213
	Drop_INET_LandAttack                          DropReasonType = 1214
	Drop_INET_MissedReset                         DropReasonType = 1215
	Drop_INET_OutsideWindow                       DropReasonType = 1216
	Drop_INET_DuplicateSegment                    DropReasonType = 1217
	Drop_INET_ClosedWindow                        DropReasonType = 1218
	Drop_INET_TcbRemoved                          DropReasonType = 1219
	Drop_INET_FinWait2                            DropReasonType = 1220
	Drop_INET_ReassemblyConflict                  DropReasonType = 1221
	Drop_INET_FinReceived                         DropReasonType = 1222
	Drop_INET_ListenerInvalidFlags                DropReasonType = 1223
	Drop_INET_TcbNotInTcbTable                    DropReasonType = 1224
	Drop_INET_TimeWaitTcbReceivedRstOutsideWindow DropReasonType = 1225
	Drop_INET_TimeWaitTcbSynAndOtherFlags         DropReasonType = 1226
	Drop_INET_TimeWaitTcb                         DropReasonType = 1227
	Drop_INET_SynAckWithFastopenCookieRequest     DropReasonType = 1228
	Drop_INET_PauseAccept                         DropReasonType = 1229
	Drop_INET_SynAttack                           DropReasonType = 1230
	Drop_INET_AcceptInspection                    DropReasonType = 1231
	Drop_INET_AcceptRedirection                   DropReasonType = 1232

	//
	// Slbmux Error
	//
	Drop_SlbMux_ParsingFailure                            DropReasonType = 1301
	Drop_SlbMux_FirstFragmentMiss                         DropReasonType = 1302
	Drop_SlbMux_ICMPErrorPayloadValidationFailure         DropReasonType = 1303
	Drop_SlbMux_ICMPErrorPacketMatchNoSession             DropReasonType = 1304
	Drop_SlbMux_ExternalHairpinNexthopLookupFailure       DropReasonType = 1305
	Drop_SlbMux_NoMatchingStaticMapping                   DropReasonType = 1306
	Drop_SlbMux_NexthopReferenceFailure                   DropReasonType = 1307
	Drop_SlbMux_CloningFailure                            DropReasonType = 1308
	Drop_SlbMux_TranslationFailure                        DropReasonType = 1309
	Drop_SlbMux_HopLimitExceeded                          DropReasonType = 1310
	Drop_SlbMux_PacketBiggerThanMTU                       DropReasonType = 1311
	Drop_SlbMux_UnexpectedRouteLookupFailure              DropReasonType = 1312
	Drop_SlbMux_NoRoute                                   DropReasonType = 1313
	Drop_SlbMux_SessionCreationFailure                    DropReasonType = 1314
	Drop_SlbMux_NexthopNotOverExternalInterface           DropReasonType = 1315
	Drop_SlbMux_NexthopExternalInterfaceMissNATInstance   DropReasonType = 1316
	Drop_SlbMux_NATItselfCantBeInternalNexthop            DropReasonType = 1317
	Drop_SlbMux_PacketRoutableInItsArrivalCompartment     DropReasonType = 1318
	Drop_SlbMux_PacketTransportProtocolNotSupported       DropReasonType = 1319
	Drop_SlbMux_PacketIsDestinedLocally                   DropReasonType = 1320
	Drop_SlbMux_PacketDestinationIPandPortNotSubjectToNAT DropReasonType = 1321
	Drop_SlbMux_MuxReject                                 DropReasonType = 1322
	Drop_SlbMux_DipLookupFailure                          DropReasonType = 1323
	Drop_SlbMux_MuxEncapsulationFailure                   DropReasonType = 1324
	Drop_SlbMux_InvalidDiagPacketEncapType                DropReasonType = 1325
	Drop_SlbMux_DiagPacketIsRedirect                      DropReasonType = 1326
	Drop_SlbMux_UnableToHandleRedirect                    DropReasonType = 1327

	//
	// Ipsec Errors
	//
	Drop_Ipsec_BadSpi                       DropReasonType = 1401
	Drop_Ipsec_SALifetimeExpired            DropReasonType = 1402
	Drop_Ipsec_WrongSA                      DropReasonType = 1403
	Drop_Ipsec_ReplayCheckFailed            DropReasonType = 1404
	Drop_Ipsec_InvalidPacket                DropReasonType = 1405
	Drop_Ipsec_IntegrityCheckFailed         DropReasonType = 1406
	Drop_Ipsec_ClearTextDrop                DropReasonType = 1407
	Drop_Ipsec_AuthFirewallDrop             DropReasonType = 1408
	Drop_Ipsec_ThrottleDrop                 DropReasonType = 1409
	Drop_Ipsec_Dosp_Block                   DropReasonType = 1410
	Drop_Ipsec_Dosp_ReceivedMulticast       DropReasonType = 1411
	Drop_Ipsec_Dosp_InvalidPacket           DropReasonType = 1412
	Drop_Ipsec_Dosp_StateLookupFailed       DropReasonType = 1413
	Drop_Ipsec_Dosp_MaxEntries              DropReasonType = 1414
	Drop_Ipsec_Dosp_KeymodNotAllowed        DropReasonType = 1415
	Drop_Ipsec_Dosp_MaxPerIpRateLimitQueues DropReasonType = 1416
	Drop_Ipsec_NoMemory                     DropReasonType = 1417
	Drop_Ipsec_Unsuccessful                 DropReasonType = 1418

	//
	// NetCx Drop Reasons
	//
	Drop_NetCx_NetPacketLayoutParseFailure        DropReasonType = 1501
	Drop_NetCx_SoftwareChecksumFailure            DropReasonType = 1502
	Drop_NetCx_NicQueueStop                       DropReasonType = 1503
	Drop_NetCx_InvalidNetBufferLength             DropReasonType = 1504
	Drop_NetCx_LSOFailure                         DropReasonType = 1505
	Drop_NetCx_USOFailure                         DropReasonType = 1506
	Drop_NetCx_BufferBounceFailureAndPacketIgnore DropReasonType = 1507

	//
	// Http errors  3000 - 4000.
	// These must be in sync with cmd\resource.h
	//
	Drop_Http_Begin DropReasonType = 3000

	//
	// UlErrors
	//
	Drop_Http_UlError_Begin        DropReasonType = 3001
	Drop_Http_UlError              DropReasonType = 3002
	Drop_Http_UlErrorVerb          DropReasonType = 3003
	Drop_Http_UlErrorUrl           DropReasonType = 3004
	Drop_Http_UlErrorHeader        DropReasonType = 3005
	Drop_Http_UlErrorHost          DropReasonType = 3006
	Drop_Http_UlErrorNum           DropReasonType = 3007
	Drop_Http_UlErrorFieldLength   DropReasonType = 3008
	Drop_Http_UlErrorRequestLength DropReasonType = 3009
	Drop_Http_UlErrorUnauthorized  DropReasonType = 3010

	Drop_Http_UlErrorForbiddenUrl        DropReasonType = 3011
	Drop_Http_UlErrorNotFound            DropReasonType = 3012
	Drop_Http_UlErrorContentLength       DropReasonType = 3013
	Drop_Http_UlErrorPreconditionFailed  DropReasonType = 3014
	Drop_Http_UlErrorEntityTooLarge      DropReasonType = 3015
	Drop_Http_UlErrorUrlLength           DropReasonType = 3016
	Drop_Http_UlErrorRangeNotSatisfiable DropReasonType = 3017
	Drop_Http_UlErrorMisdirectedRequest  DropReasonType = 3018

	Drop_Http_UlErrorInternalServer      DropReasonType = 3019
	Drop_Http_UlErrorNotImplemented      DropReasonType = 3020
	Drop_Http_UlErrorUnavailable         DropReasonType = 3021
	Drop_Http_UlErrorConnectionLimit     DropReasonType = 3022
	Drop_Http_UlErrorRapidFailProtection DropReasonType = 3023
	Drop_Http_UlErrorRequestQueueFull    DropReasonType = 3024
	Drop_Http_UlErrorDisabledByAdmin     DropReasonType = 3025
	Drop_Http_UlErrorDisabledByApp       DropReasonType = 3026
	Drop_Http_UlErrorJobObjectFired      DropReasonType = 3027
	Drop_Http_UlErrorAppPoolBusy         DropReasonType = 3028

	Drop_Http_UlErrorVersion DropReasonType = 3029
	Drop_Http_UlError_End    DropReasonType = 3030

	//
	// Stream-specific fault codes.
	//

	Drop_Http_UxDuoFaultBegin                      DropReasonType = 3400
	Drop_Http_UxDuoFaultUserAbort                  DropReasonType = 3401
	Drop_Http_UxDuoFaultCollection                 DropReasonType = 3402
	Drop_Http_UxDuoFaultClientResetStream          DropReasonType = 3403
	Drop_Http_UxDuoFaultMethodNotFound             DropReasonType = 3404
	Drop_Http_UxDuoFaultSchemeMismatch             DropReasonType = 3405
	Drop_Http_UxDuoFaultSchemeNotFound             DropReasonType = 3406
	Drop_Http_UxDuoFaultDataAfterEnd               DropReasonType = 3407
	Drop_Http_UxDuoFaultPathNotFound               DropReasonType = 3408
	Drop_Http_UxDuoFaultHalfClosedLocal            DropReasonType = 3409
	Drop_Http_UxDuoFaultIncompatibleAuth           DropReasonType = 3410
	Drop_Http_UxDuoFaultDeprecated3                DropReasonType = 3411
	Drop_Http_UxDuoFaultClientCertBlocked          DropReasonType = 3412
	Drop_Http_UxDuoFaultHeaderNameEmpty            DropReasonType = 3413
	Drop_Http_UxDuoFaultIllegalSend                DropReasonType = 3414
	Drop_Http_UxDuoFaultPushUpperAttach            DropReasonType = 3415
	Drop_Http_UxDuoFaultStreamUpperAttach          DropReasonType = 3416
	Drop_Http_UxDuoFaultActiveStreamLimit          DropReasonType = 3417
	Drop_Http_UxDuoFaultAuthorityNotFound          DropReasonType = 3418
	Drop_Http_UxDuoFaultUnexpectedTail             DropReasonType = 3419
	Drop_Http_UxDuoFaultTruncated                  DropReasonType = 3420
	Drop_Http_UxDuoFaultResponseHold               DropReasonType = 3421
	Drop_Http_UxDuoFaultRequestChunked             DropReasonType = 3422
	Drop_Http_UxDuoFaultRequestContentLength       DropReasonType = 3423
	Drop_Http_UxDuoFaultResponseChunked            DropReasonType = 3424
	Drop_Http_UxDuoFaultResponseContentLength      DropReasonType = 3425
	Drop_Http_UxDuoFaultResponseTransferEncoding   DropReasonType = 3426
	Drop_Http_UxDuoFaultResponseLine               DropReasonType = 3427
	Drop_Http_UxDuoFaultResponseHeader             DropReasonType = 3428
	Drop_Http_UxDuoFaultConnect                    DropReasonType = 3429
	Drop_Http_UxDuoFaultChunkStart                 DropReasonType = 3430
	Drop_Http_UxDuoFaultChunkLength                DropReasonType = 3431
	Drop_Http_UxDuoFaultChunkStop                  DropReasonType = 3432
	Drop_Http_UxDuoFaultHeadersAfterTrailers       DropReasonType = 3433
	Drop_Http_UxDuoFaultHeadersAfterEnd            DropReasonType = 3434
	Drop_Http_UxDuoFaultEndlessTrailer             DropReasonType = 3435
	Drop_Http_UxDuoFaultTransferEncoding           DropReasonType = 3436
	Drop_Http_UxDuoFaultMultipleTransferCodings    DropReasonType = 3437
	Drop_Http_UxDuoFaultPushBody                   DropReasonType = 3438
	Drop_Http_UxDuoFaultStreamAbandoned            DropReasonType = 3439
	Drop_Http_UxDuoFaultMalformedHost              DropReasonType = 3440
	Drop_Http_UxDuoFaultDecompressionOverflow      DropReasonType = 3441
	Drop_Http_UxDuoFaultIllegalHeaderName          DropReasonType = 3442
	Drop_Http_UxDuoFaultIllegalHeaderValue         DropReasonType = 3443
	Drop_Http_UxDuoFaultConnHeaderDisallowed       DropReasonType = 3444
	Drop_Http_UxDuoFaultConnHeaderMalformed        DropReasonType = 3445
	Drop_Http_UxDuoFaultCookieReassembly           DropReasonType = 3446
	Drop_Http_UxDuoFaultStatusHeader               DropReasonType = 3447
	Drop_Http_UxDuoFaultSchemeDisallowed           DropReasonType = 3448
	Drop_Http_UxDuoFaultPathDisallowed             DropReasonType = 3449
	Drop_Http_UxDuoFaultPushHost                   DropReasonType = 3450
	Drop_Http_UxDuoFaultGoawayReceived             DropReasonType = 3451
	Drop_Http_UxDuoFaultAbortLegacyApp             DropReasonType = 3452
	Drop_Http_UxDuoFaultUpgradeHeaderDisallowed    DropReasonType = 3453
	Drop_Http_UxDuoFaultResponseUpgradeHeader      DropReasonType = 3454
	Drop_Http_UxDuoFaultKeepAliveHeaderDisallowed  DropReasonType = 3455
	Drop_Http_UxDuoFaultResponseKeepAliveHeader    DropReasonType = 3456
	Drop_Http_UxDuoFaultProxyConnHeaderDisallowed  DropReasonType = 3457
	Drop_Http_UxDuoFaultResponseProxyConnHeader    DropReasonType = 3458
	Drop_Http_UxDuoFaultConnectionGoingAway        DropReasonType = 3459
	Drop_Http_UxDuoFaultTransferEncodingDisallowed DropReasonType = 3460
	Drop_Http_UxDuoFaultContentLengthDisallowed    DropReasonType = 3461
	Drop_Http_UxDuoFaultTrailerDisallowed          DropReasonType = 3462
	Drop_Http_UxDuoFaultEnd                        DropReasonType = 3463

	//
	//  WSK layer drops
	//
	Drop_Http_ReceiveSuppressed DropReasonType = 3600

	//
	//  Http/SSL layer drops
	//
	Drop_Http_Generic               DropReasonType = 3800
	Drop_Http_InvalidParameter      DropReasonType = 3801
	Drop_Http_InsufficientResources DropReasonType = 3802
	Drop_Http_InvalidHandle         DropReasonType = 3803
	Drop_Http_NotSupported          DropReasonType = 3804
	Drop_Http_BadNetworkPath        DropReasonType = 3805
	Drop_Http_InternalError         DropReasonType = 3806
	Drop_Http_NoSuchPackage         DropReasonType = 3807
	Drop_Http_PrivilegeNotHeld      DropReasonType = 3808
	Drop_Http_CannotImpersonate     DropReasonType = 3809
	Drop_Http_LogonFailure          DropReasonType = 3810
	Drop_Http_NoSuchLogonSession    DropReasonType = 3811
	Drop_Http_AccessDenied          DropReasonType = 3812
	Drop_Http_NoLogonServers        DropReasonType = 3813
	Drop_Http_TimeDifferenceAtDc    DropReasonType = 3814

	Drop_Http_End DropReasonType = 4000
)
