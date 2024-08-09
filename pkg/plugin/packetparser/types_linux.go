// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package packetparser

import (
	"sync"
	"time"

	kcfg "github.com/microsoft/retina/pkg/config"

	"github.com/cilium/cilium/api/v1/flow"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/perf"
	"github.com/florianl/go-tc"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/plugin/api"
	"github.com/vishvananda/netlink"
)

const (
	Name                  api.PluginName = "packetparser"
	toEndpoint            string         = "toEndpoint"
	fromEndpoint          string         = "fromEndpoint"
	Veth                  string         = "veth"
	Device                string         = "device"
	workers               int            = 2
	buffer                int            = 10000
	flowCacheSize         int            = 1000
	flowCacheTTL          time.Duration  = 6 * time.Minute
	bpfSourceDir          string         = "_cprog"
	bpfSourceFileName     string         = "packetparser.c"
	bpfObjectFileName     string         = "packetparser_bpf.o"
	dynamicHeaderFileName string         = "dynamic.h"
)

var (
	getQdisc = func(tcnl ITc) IQdisc {
		return tcnl.Qdisc()
	}
	getFilter = func(tcnl ITc) IFilter {
		return tcnl.Filter()
	}
	tcOpen = func(config *tc.Config) (ITc, error) {
		return tc.Open(config)
	}
	getFD = func(e *ebpf.Program) int {
		return e.FD()
	}
	// Determined via testing on a large cluster.
	// Actual buffer size will be 32 * pagesize.
	perCPUBuffer = 32
)

type key struct {
	name         string
	hardwareAddr string
	netNs        int
}

type flowCacheKey struct {
	srcIP   uint32
	dstIP   uint32
	srcPort uint32
	dstPort uint32
	proto   uint8
	dir     uint32
}

//go:generate go run go.uber.org/mock/mockgen@v0.4.0 -source=types_linux.go -destination=mocks/mock_types.go -package=mocks

// Define the interfaces.
type IQdisc interface {
	Add(info *tc.Object) error
	Delete(info *tc.Object) error
}

type IFilter interface {
	Add(info *tc.Object) error
}

type ITc interface {
	Qdisc() *tc.Qdisc
	Filter() *tc.Filter
	Close() error
}

type IPerf interface {
	Read() (perf.Record, error)
	Close() error
}

type val struct {
	tcnl         ITc
	tcIngressObj *tc.Object
	tcEgressObj  *tc.Object
}

type packetParser struct {
	cfg        *kcfg.Config
	l          *log.ZapLogger
	callbackID string
	flowCache  *expirable.LRU[flowCacheKey, *flow.Flow]
	objs       *packetparserObjects //nolint:typecheck
	// tcMap is a map of key to *val.
	tcMap    *sync.Map
	reader   IPerf
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

func ifaceToKey(iface netlink.LinkAttrs) key {
	return key{
		name:         iface.Name,
		hardwareAddr: iface.HardwareAddr.String(),
		netNs:        iface.NetNsID,
	}
}

const (
	handleMajMask uint32 = 0xFFFF0000
	handleMinMask uint32 = 0x0000FFFF
)

func TC_H_MAKE(maj, min uint32) uint32 {
	return (((maj) & handleMajMask) | (min & handleMinMask))
}
