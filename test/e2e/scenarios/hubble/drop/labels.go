package drop

import (
	"github.com/microsoft/retina/test/e2e/framework/constants"
)

var (
	podName     = "agnhost-drop-0"
	agnhostName = "agnhost-drop"

	validRetinaDropMetricLabels = map[string]string{
		constants.RetinaReasonLabel:    constants.IPTableRuleDrop,
		constants.RetinaDirectionLabel: "unknown",
	}

	// Note: When the agnhost pod (with deny-all network policy) tries to curl bing.com,
	// it triggers a DNS lookup to CoreDNS. The network policy blocks this egress traffic,
	// but Cilium/Hubble records the drop at the destination (CoreDNS) ingress side rather
	// than the source (agnhost) egress side.
	// 'source:kube-system/agnhost-drop-0' is not recorded in Hubble drop metrics.
	// We partially validate this metric.
	validHubbleDropMetricLabels = map[string]string{
		constants.HubbleSourceLabel:   "",
		constants.HubbleProtocolLabel: constants.UDP,
		constants.HubbleReasonLabel:   "POLICY_DENIED",
	}
)
