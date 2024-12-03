// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package packetforward

import (
	"fmt"

	kcfg "github.com/microsoft/retina/pkg/config"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/plugin/api"
)

const (
	PacketForwardSocketAttach int            = 50
	Name                      api.PluginName = "packetforward"
	socketIndex               int            = 0
	ingressKey                uint32         = 0
	egressKey                 uint32         = 1
	ingressLabel              string         = "ingress"
	egressLabel               string         = "egress"
	bpfObjectFileName         string         = "packetforward_bpf.o"
	bpfSourceDir              string         = "_cprog"
	bpfSourceFileName         string         = "packetforward.c"
	dynamicHeaderFileName     string         = "dynamic.h"
)

// Interface to https://pkg.go.dev/github.com/cilium/ebpf#Map.
// Added for unit tests.
//
//go:generate mockgen -destination=mocks/mock_types.go -package=mocks . IMap
type IMap interface {
	Lookup(key, valueOut interface{}) error
	Close() error
}

type packetForward struct {
	cfg         *kcfg.Config
	l           *log.ZapLogger
	hashmapData IMap
	sock        int
	isRunning   bool
}

type PacketForwardData struct {
	ingressBytesTotal uint64
	ingressCountTotal uint64
	egressBytesTotal  uint64
	egressCountTotal  uint64
}

func (p PacketForwardData) String() string {
	return fmt.Sprintf("IngressBytes:%d IngressPackets:%d EgressBytes:%d EgressPackets:%d",
		p.ingressBytesTotal, p.ingressCountTotal, p.egressBytesTotal, p.egressCountTotal)
}
