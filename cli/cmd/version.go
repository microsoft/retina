// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/microsoft/retina/internal/buildinfo"
)

var version = &cobra.Command{
	Use:   "version",
	Short: "Show version",
	Run: func(*cobra.Command, []string) {
		fmt.Println(buildinfo.Version)
	},
}

func init() {
	Retina.AddCommand(version)
}
