// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package config

const (
	// Retina Metrics Port
	RetinaMetricsPort = "10093"

	// Retina MetricsName
	RetinaDropMetricName    = "networkobservability_drop_count"
	RetinaForwardMetricName = "networkobservability_forward_count"

	// Retina Labels
	RetinaSourceLabel      = "source"
	RetinaDestinationLabel = "destination"
	RetinaProtocolLabel    = "protocol"
	RetinaReasonLabel      = "reason"
	RetinaDirectionLabel   = "direction"

	// Hubble Metrics Port
	HubbleMetricsPort = "9965"

	// Hubble MetricsName
	HubbleDNSQueryMetricName    = "hubble_dns_queries_total"
	HubbleDNSResponseMetricName = "hubble_dns_responses_total"
	HubbleFlowMetricName        = "hubble_flows_processed_total"
	HubbleDropMetricName        = "hubble_drop_total"
	HubbleTCPFlagsMetricName    = "hubble_tcp_flags_total"

	// Hubble Labels
	HubbleDestinationLabel = "destination"
	HubbleSourceLabel      = "source"
	HubbleIPsRetunedLabel  = "ips_returned"
	HubbleQTypesLabel      = "qtypes"
	HubbleRCodeLabel       = "rcode"
	HubbleQueryLabel       = "query"

	HubbleProtocolLabel = "protocol"
	HubbleReasonLabel   = "reason"

	HubbleSubtypeLabel = "subtype"
	HubbleTypeLabel    = "type"
	HubbleVerdictLabel = "verdict"

	HubbleFamilyLabel = "family"
	HubbleFlagLabel   = "flag"
)
