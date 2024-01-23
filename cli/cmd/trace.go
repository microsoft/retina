// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package cmd

import (
	"github.com/spf13/cobra"
)

func TraceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trace",
		Short: "retrieve status or results from Retina",
	}
	cmd.AddCommand(getTrace())
	return cmd
}

func getTrace() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Retrieve network trace results with operation ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			operationID, _ := cmd.Flags().GetString("operationID")
			return RetinaClient.GetTrace(operationID)
		},
	}
	cmd.Flags().String("operationID", "", "Network Trace Operation ID")
	_ = cmd.MarkFlagRequired("operationID")
	return cmd
}
