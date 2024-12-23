// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"github.com/microsoft/retina/cli/cmd"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var opts = struct {
	genericclioptions.ConfigFlags
	Name *string
}{
	Name: new(string),
}

const defaultName = "retina-capture"

var capture = &cobra.Command{
	Use:   "capture",
	Short: "Capture network traffic",
}

func init() {
	cmd.Retina.AddCommand(capture)
	opts.ConfigFlags = *genericclioptions.NewConfigFlags(true)
	opts.AddFlags(capture.PersistentFlags())
	capture.PersistentFlags().StringVar(opts.Name, "name", defaultName, "The name of the Retina Capture")
}
