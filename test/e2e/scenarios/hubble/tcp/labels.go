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
	validHubbleTCPSYNACKFlag = map[string]string{
		constants.HubbleSourceLabel:      "",
		constants.HubbleDestinationLabel: common.TestPodNamespace + "/" + podName,
		constants.HubbleFamilyLabel:      constants.IPV4,
		constants.HubbleFlagLabel:        constants.SYNACK,
	}
	validHubbleTCPFINFlag = map[string]string{
		constants.HubbleSourceLabel:      "",
		constants.HubbleDestinationLabel: common.TestPodNamespace + "/" + podName,
		constants.HubbleFamilyLabel:      constants.IPV4,
		constants.HubbleFlagLabel:        constants.FIN,
	}

	validHubbleTCPMetricsLabels = []map[string]string{
		validHubbleTCPSYNFlag,
		validHubbleTCPSYNACKFlag, //TODO: Needs to further investigate why these labels are not being generated
		validHubbleTCPFINFlag,
	}
)
