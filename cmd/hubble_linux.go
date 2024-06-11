// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package cmd

import (
	"github.com/cilium/cilium/pkg/hive"
	"github.com/microsoft/retina/cmd/hubble"
)

func init() {
	h := hive.New(hubble.Agent)
	rootCmd.AddCommand(hubble.Cmd(h))
}
