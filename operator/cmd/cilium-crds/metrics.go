// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium and Retina

// NOTE: changed to say networkobservability_operator

package ciliumcrds

import (
	"github.com/spf13/cobra"
)

const RetinaOperatorMetricsNamespace = "networkobservability_operator"

// MetricsCmd represents the metrics command for the operator.
var MetricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Access metric status of the operator",
}
