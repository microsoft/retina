// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package packetparsertcx

import (
	"sync"

	kcfg "github.com/microsoft/retina/pkg/config"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/vishvananda/netlink"

	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
)

const (
	TCPFlagFIN = 1 << iota
	TCPFlagSYN
	TCPFlagRST
	TCPFlagPSH
	TCPFlagACK
	TCPFlagURG
	TCPFlagECE
	TCPFlagCWR
	TCPFlagNS
)

const (
	name                  string = "packetparsertcx"
	workers               int    = 2
	buffer                int    = 10000
	bpfSourceDir          string = "_cprog"
	bpfSourceFileName     string = "packetparser_tcx.c"
	bpfObjectFileName     string = "packetparser_tcx_bpf.o"
	dynamicHeaderFileName string = "dynamic.h"
)

type interfaceType string

const (
	Veth   interfaceType = "veth"
	Device interfaceType = "device"
)

// Determined via testing on a large cluster.
// Actual buffer size will be 32 * pagesize.
var perCPUBuffer = 32

type tcxKey struct {
	name         string
	hardwareAddr string
	netNs        int
}

type tcxValue struct {
	ingressLink link.Link
	egressLink  link.Link
}

type perfReader interface {
	Read() (perf.Record, error)
	Close() error
}

type packetParserTCX struct {
	cfg        *kcfg.Config
	l          *log.ZapLogger
	callbackID string
	objs       *packetparsertcxObjects //nolint:typecheck // type from bpf2go-generated code, not present until build
	// tcxMap tracks TCX link attachments per interface.
	tcxMap   *sync.Map
	reader   perfReader
	enricher enricher.EnricherInterface
	// interfaceLockMap is a map of key to *sync.Mutex.
	interfaceLockMap    *sync.Map
	endpointIngressInfo *ebpf.ProgramInfo
	endpointEgressInfo  *ebpf.ProgramInfo
	hostIngressInfo     *ebpf.ProgramInfo
	hostEgressInfo      *ebpf.ProgramInfo
	wg                  sync.WaitGroup
	recordsChannel      chan perf.Record
	externalChannel     chan *v1.Event
}

func ifaceToKey(iface netlink.LinkAttrs) tcxKey {
	return tcxKey{
		name:         iface.Name,
		hardwareAddr: iface.HardwareAddr.String(),
		netNs:        iface.NetNsID,
	}
}
