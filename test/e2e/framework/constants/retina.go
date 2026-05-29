package constants

const (
	// Metrics Port
	RetinaMetricsPort = "10093"

	// MetricsName
	RetinaDropMetricName    = "networkobservability_drop_count"
	RetinaForwardMetricName = "networkobservability_forward_count"

	// Labels
	RetinaSourceLabel      = "source"
	RetinaDestinationLabel = "destination"
	RetinaProtocolLabel    = "protocol"
	RetinaReasonLabel      = "reason"
	RetinaDirectionLabel   = "direction"
)
