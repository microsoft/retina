// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package debug

import (
	retinacmd "github.com/microsoft/retina/cli/cmd"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var opts = struct {
	genericclioptions.ConfigFlags
}{
	ConfigFlags: *genericclioptions.NewConfigFlags(true),
}

var debug = &cobra.Command{
	Use:   "debug",
	Short: "Debug network issues",
	Long:  "Debug network issues using various Retina debugging tools",
}

func init() {
	retinacmd.Retina.AddCommand(debug)
	opts.AddFlags(debug.PersistentFlags())
}