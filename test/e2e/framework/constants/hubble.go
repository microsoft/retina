package constants

const (
	// Metrics Port
	HubbleMetricsPort = "9965"

	// MetricsName
	HubbleDNSQueryMetricName    = "hubble_dns_queries_total"
	HubbleDNSResponseMetricName = "hubble_dns_responses_total"
	HubbleFlowMetricName        = "hubble_flows_processed_total"
	HubbleDropMetricName        = "hubble_drop_total"
	HubbleTCPFlagsMetricName    = "hubble_tcp_flags_total"

	// Labels
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
