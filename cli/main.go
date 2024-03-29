// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package main

import (
	"github.com/microsoft/retina/cli/cmd"
	"github.com/microsoft/retina/cli/cmd/capture"
)

func main() {
	rootCmd := cmd.NewRootCmd()
	// rootCmd.AddCommand(trace.TraceCmd())
	rootCmd.AddCommand(capture.CaptureCmd())
	rootCmd.AddCommand(cmd.VersionCmd())
	rootCmd.Execute() //nolint:errcheck
}
