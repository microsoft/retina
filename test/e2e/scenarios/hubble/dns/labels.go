package dns

import (
	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/e2e/framework/constants"
)

var (
	podName                   = "agnhost-dns-0"
	validDNSQueryMetricLabels = map[string]string{
		constants.HubbleDestinationLabel: "",
		constants.HubbleSourceLabel:      common.TestPodNamespace + "/" + podName,
		constants.HubbleIPsRetunedLabel:  "0",
		constants.HubbleQTypesLabel:      "A",
		constants.HubbleRCodeLabel:       "",
		constants.HubbleQueryLabel:       "one.one.one.one.",
	}
	validDNSResponseMetricLabels = map[string]string{
		constants.HubbleDestinationLabel: common.TestPodNamespace + "/" + podName,
		constants.HubbleSourceLabel:      "",
		constants.HubbleIPsRetunedLabel:  "2",
		constants.HubbleQTypesLabel:      "A",
		constants.HubbleRCodeLabel:       "No Error",
		constants.HubbleQueryLabel:       "one.one.one.one.",
	}
)
