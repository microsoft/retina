// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build linux

package ctrinfo

func runGetPods(c *Ctrinfo) (string, error) {
	return c.runCommand("crictl", "pods", "-q")
}

func runPodInspect(c *Ctrinfo, id string) (string, error) {
	return c.runCommand("crictl", "inspectp", id)
}
