// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package packetparser

import (
	"sync"

	kcfg "github.com/microsoft/retina/pkg/config"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/perf"
	tc "github.com/florianl/go-tc"
	nl "github.com/mdlayher/netlink"
	"github.com/vishvananda/netlink"

	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/plugin/api"
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
)

const (
	Name                  api.PluginName = "packetparser"
	toEndpoint            string         = "toEndpoint"
	fromEndpoint          string         = "fromEndpoint"
	workers               int            = 2
	buffer                int            = 10000
	bpfSourceDir          string         = "_cprog"
	bpfSourceFileName     string         = "packetparser.c"
	bpfObjectFileName     string         = "packetparser_bpf.o"
	dynamicHeaderFileName string         = "dynamic.h"
	tcFilterPriority      uint16         = 0x1
)

type interfaceType string

const (
	Veth   interfaceType = "veth"
	Device interfaceType = "device"
)

var (
	getQdisc = func(tcnl nltc) qdisc {
		return tcnl.Qdisc()
	}
	getFilter = func(tcnl nltc) filter {
		return tcnl.Filter()
	}
	tcOpen = func(config *tc.Config) (nltc, error) {
		return tc.Open(config)
	}
	getFD = func(e *ebpf.Program) int {
		return e.FD()
	}
	// Determined via testing on a large cluster.
	// Actual buffer size will be 32 * pagesize.
	perCPUBuffer = 32
)

type tcKey struct {
	name         string
	hardwareAddr string
	netNs        int
}

type tcValue struct {
	tc    nltc
	qdisc *tc.Object
}

//go:generate mockgen -source=types_linux.go -destination=mocks/mock_types.go -package=mocks

// tc qdisc interface
type qdisc interface {
	Add(info *tc.Object) error
	Delete(info *tc.Object) error
}

// tc filter interface
type filter interface {
	Add(info *tc.Object) error
}

// netlink tc interface
type nltc interface {
	Qdisc() *tc.Qdisc
	Filter() *tc.Filter
	SetOption(nl.ConnOption, bool) error
	Close() error
}

type perfReader interface {
	Read() (perf.Record, error)
	Close() error
}

type packetParser struct {
	cfg        *kcfg.Config
	l          *log.ZapLogger
	callbackID string
	objs       *packetparserObjects //nolint:typecheck
	// tcMap is a map of key to *val.
	tcMap    *sync.Map
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

func ifaceToKey(iface netlink.LinkAttrs) tcKey {
	return tcKey{
		name:         iface.Name,
		hardwareAddr: iface.HardwareAddr.String(),
		netNs:        iface.NetNsID,
	}
}
