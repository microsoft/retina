// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package main

import (
	"github.com/cilium/cilium/pkg/hive"
	"github.com/microsoft/retina/cmd"
	"github.com/microsoft/retina/cmd/hubble"
)

func main() {
	h := hive.New(hubble.Agent)
	cmd.Execute(h)
}
