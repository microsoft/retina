// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"github.com/microsoft/retina/cli/cmd"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var name string

const defaultName = "retina-capture"

var capture = &cobra.Command{
	Use:   "capture",
	Short: "Capture network traffic",
}

func init() {
	cmd.Retina.AddCommand(capture)
	configFlags = genericclioptions.NewConfigFlags(true)
	configFlags.AddFlags(capture.PersistentFlags())
	capture.PersistentFlags().StringVar(&name, "name", defaultName, "The name of the Retina Capture")
}
