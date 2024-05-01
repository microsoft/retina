// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// This variable is used by the "version" command and is set during build.
// Defaults to a safe value if not set.
var Version = "v0.0.5"

var version = &cobra.Command{
	Use:   "version",
	Short: "Show version",
	Run: func(*cobra.Command, []string) {
		fmt.Println(Version)
	},
}

func init() {
	Retina.AddCommand(version)
}
