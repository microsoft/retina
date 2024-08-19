// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package utils

import (
	"go.opentelemetry.io/otel/attribute"
)

const (
	unknown = "__uknown__"
)

var (
	pluginKey    = attribute.Key("plugin")
	eventTypeKey = attribute.Key("event_type")
	timestampKey = attribute.Key("timestamp")

	localIPKey        = attribute.Key("local_ip")
	localPortKey      = attribute.Key("local_port")
	localKindKey      = attribute.Key("local_kind")
	localNameKey      = attribute.Key("local_name")
	localNamespaceKey = attribute.Key("local_namespace")
	localOwnerNameKey = attribute.Key("local_owner_name")
	localOwnerKindKey = attribute.Key("local_owner_kind")
	localNodeKey      = attribute.Key("local_node")

	remoteIPKey        = attribute.Key("remote_ip")
	remotePortKey      = attribute.Key("remote_port")
	remoteKindKey      = attribute.Key("remote_kind")
	remoteNameKey      = attribute.Key("remote_name")
	remoteNamespaceKey = attribute.Key("remote_namespace")
	remoteOwnerNameKey = attribute.Key("remote_owner_name")
	remoteOwnerKindKey = attribute.Key("remote_owner_kind")
	remoteNodeKey      = attribute.Key("remote_node")

	// todo move to attributes pkg?
	Type           = "type"
	Reason         = "reason"
	Direction      = "direction"
	SourceNodeName = "source_node_name"
	TargetNodeName = "target_node_name"
	State          = "state"
	Address        = "address"
	Port           = "port"
	StatName       = "statistic_name"
	InterfaceName  = "interface_name"
	Flag           = "flag"
	Endpoint       = "endpoint"
	AclRule        = "aclrule"
	Active         = "ACTIVE"
	Device         = "device"

	// TCP Connection Statistic Names
	ResetCount           = "ResetCount"
	ClosedFin            = "ClosedFin"
	ResetSyn             = "ResetSyn"
	TcpHalfOpenTimeouts  = "TcpHalfOpenTimeouts"
	Verified             = "Verified"
	TimedOutCount        = "TimedOutCount"
	TimeWaitExpiredCount = "TimeWaitExpiredCount"

	// Events types
	Kernel          = "kernel"
	EnricherRing    = "enricher_ring"
	BufferedChannel = "buffered_channel"
	ExternalChannel = "external_channel"

	// TCP Flags
	SYN    = "SYN"
	SYNACK = "SYNACK"
	ACK    = "ACK"
	FIN    = "FIN"
	RST    = "RST"
	PSH    = "PSH"
	ECE    = "ECE"
	CWR    = "CWR"
	NS     = "NS"
	URG    = "URG"

	DataPlane = "dataplane"
	Linux     = "linux"
	Windows   = "windows"

	// DNS labels.
	DNSRequestLabels  = []string{"query_type", "query"}
	DNSResponseLabels = []string{"return_code", "query_type", "query", "response", "num_response"}

	FlowsGaugeLabels = []string{
		"source_ip",
		"source_port",
		"destination_ip",
		"destination_port",
		"protocol",
		"flow_direction",
	}
)

func GetPluginEventAttributes(attrs []attribute.KeyValue, pluginName, eventName, timestamp string) []attribute.KeyValue {
	return append(attrs,
		pluginKey.String(pluginName),
		eventTypeKey.String(eventName),
		timestampKey.String(timestamp),
	)
}
