// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package steps

import (
	"context"
	"log"

	k8s "github.com/microsoft/retina/test/e2ev3/framework/kubernetes"
)

// StopPortForwardStep stops a previously started port forward.
type StopPortForwardStep struct {
	PF *k8s.PortForward
}

func (s *StopPortForwardStep) Do(_ context.Context) error {
	log.Printf("stopping port forward %s -> %s", s.PF.LocalPort, s.PF.RemotePort)
	return s.PF.Stop()
}
