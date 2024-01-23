// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"github.com/spf13/cobra"
)

func CaptureCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "capture",
		Short: "Retina Capture - capture network traffic",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help() //nolint:errcheck
		},
	}

	cmd.AddCommand(CaptureCmdCreate())
	cmd.AddCommand(CaptureCmdList())
	cmd.AddCommand(CaptureCmdDelete())

	return cmd
}
