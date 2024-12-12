package drop

import (
	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/e2e/framework/constants"
)

var (
	podName                     = "agnhost-drop-0"
	validHubbleDropMetricLabels = map[string]string{
		constants.HubbleSourceLabel:      common.TestPodNamespace + "/" + podName,
		constants.HubbleDestinationLabel: "",
		constants.HubbleProtocolLabel:    constants.UDP,
		constants.HubbleReasonLabel:      "POLICY_DENIED",
	}
)
