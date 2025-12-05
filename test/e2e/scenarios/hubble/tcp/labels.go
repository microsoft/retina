package tcp

import (
	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/e2e/framework/constants"
)

var (
	podName               = "agnhost-tcp-0"
	validHubbleTCPSYNFlag = map[string]string{
		constants.HubbleSourceLabel:      common.TestPodNamespace + "/" + podName,
		constants.HubbleDestinationLabel: "",
		constants.HubbleFamilyLabel:      constants.IPV4,
		constants.HubbleFlagLabel:        constants.SYN,
	}
	validHubbleTCPFINFlag = map[string]string{
		constants.HubbleSourceLabel:      common.TestPodNamespace + "/" + podName,
		constants.HubbleDestinationLabel: "",
		constants.HubbleFamilyLabel:      constants.IPV4,
		constants.HubbleFlagLabel:        constants.FIN,
	}

	validHubbleTCPMetricsLabels = []map[string]string{
		validHubbleTCPSYNFlag,
		validHubbleTCPFINFlag,
	}
)
