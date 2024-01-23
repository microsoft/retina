// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// This variable is used by the "version" command and is set during build.
var Version = "undefined"

func VersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(Version)
		},
	}
	return cmd
}
