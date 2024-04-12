// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package cmd

import (
	"github.com/spf13/cobra"
)

var trace = &cobra.Command{
	Use:   "trace",
	Short: "retrieve status or results from Retina",
}

var getTrace = &cobra.Command{
	Use:   "get",
	Short: "Retrieve network trace results with operation ID",
	RunE: func(cmd *cobra.Command, args []string) error {
		operationID, _ := cmd.Flags().GetString("operationID")
		return RetinaClient.GetTrace(operationID)
	},
}

func init() {
	getTrace.Flags().String("operationID", "", "Network Trace Operation ID")
	_ = getTrace.MarkFlagRequired("operationID")
	trace.AddCommand(getTrace)
	Retina.AddCommand(trace)
}
