// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package cmd

import (
	"fmt"
	"runtime/debug"

	"github.com/microsoft/retina/internal/buildinfo"
	"github.com/spf13/cobra"
)

var version = &cobra.Command{
	Use:   "version",
	Short: "Show version",
	Run: func(cmd *cobra.Command, args []string) {
		// Version is fetched from buildinfo package.
		if buildinfo.Version != "" {
			fmt.Println(buildinfo.Version)
			return
		}

		// Fetch version from Build Settings.
		const baseLine = "buildinfo.Version is not set successfully"

		info, ok := debug.ReadBuildInfo()
		if !ok {
			fmt.Printf("%s, and BuildInfo is not available\n", baseLine)
			return
		}

		var revision string
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				revision = setting.Value
				break
			}
		}

		// If revision is available, show it.
		if revision != "" {
			fmt.Printf("%s, showing vcs.revision: %s\n", baseLine, revision)
			return
		}

		// Revision is not available, raise error.
		fmt.Printf("%s, and vcs.revision is not available\n", baseLine)
	},
}

func init() {
	Retina.AddCommand(version)
}
