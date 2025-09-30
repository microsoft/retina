// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build windows

package ctrinfo

func runGetPods(c *Ctrinfo) (string, error) {
	return c.runCommand("cmd", "/c", "crictl", "pods", "-q")
}

func runPodInspect(c *Ctrinfo, id string) (string, error) {
	return c.runCommand("cmd", "/c", "crictl", "inspectp", id)
}
