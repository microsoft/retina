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
)
