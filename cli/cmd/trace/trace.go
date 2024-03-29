// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package trace

import (
	retinacmd "github.com/microsoft/retina/cli/cmd"
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
		RunE: func(cmd *cobra.Command, _ []string) error {
			operationID, _ := cmd.Flags().GetString("operationID")
			err := retinacmd.RetinaClient.GetTrace(operationID)
			if err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().String("operationID", "", "Network Trace Operation ID")
	_ = cmd.MarkFlagRequired("operationID")
	return cmd
}
