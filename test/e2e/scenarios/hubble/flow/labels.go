package flow

import (
	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/e2e/framework/constants"
)

var (
	podName                      = "agnhost-flow-0"
	validHubbleFlowLabelsToStack = map[string]string{
		constants.HubbleSourceLabel:      common.TestPodNamespace + "/" + podName,
		constants.HubbleDestinationLabel: "",
		constants.HubbleProtocolLabel:    constants.UDP,
		constants.HubbleSubtypeLabel:     "to-stack",
		constants.HubbleTypeLabel:        "Trace",
		constants.HubbleVerdictLabel:     "FORWARDED",
	}
	validHubbleFlowLabelsToEndpoint = map[string]string{
		constants.HubbleSourceLabel:      "",
		constants.HubbleDestinationLabel: common.TestPodNamespace + "/" + podName,
		constants.HubbleProtocolLabel:    constants.TCP,
		constants.HubbleSubtypeLabel:     "to-endpoint",
		constants.HubbleTypeLabel:        "Trace",
		constants.HubbleVerdictLabel:     "FORWARDED",
	}

	validHubbleFlowMetricsLabels = []map[string]string{
		validHubbleFlowLabelsToStack,
		// TODO: Needs to further investigate why these labels are not being generated
		// validHubbleFlowLabelsToNetwork,
		// validHubbleFlowLabelsFromNetwork,
		validHubbleFlowLabelsToEndpoint,
	}

	// validHubbleFlowLabelsToNetwork = map[string]string{
	// 	constants.HubbleSourceLabel:      common.TestPodNamespace + "/" + podName,
	// 	constants.HubbleDestinationLabel: "",
	// 	constants.HubbleProtocolLabel:    constants.UDP,
	// 	constants.HubbleSubtypeLabel:     "to-network",
	// 	constants.HubbleTypeLabel:        "Trace",
	// 	constants.HubbleVerdictLabel:     "FORWARDED",
	// }
	// validHubbleFlowLabelsFromNetwork = map[string]string{
	// 	constants.HubbleSourceLabel:      "",
	// 	constants.HubbleDestinationLabel: common.TestPodNamespace + "/" + podName,
	// 	constants.HubbleProtocolLabel:    constants.UDP,
	// 	constants.HubbleSubtypeLabel:     "from-network",
	// 	constants.HubbleTypeLabel:        "Trace",
	// 	constants.HubbleVerdictLabel:     "FORWARDED",
	// }
)
