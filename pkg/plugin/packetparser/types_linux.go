// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package packetparser

import (
	"context"
	"sync"
	"time"
	"unsafe"

	kcfg "github.com/microsoft/retina/pkg/config"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	tc "github.com/florianl/go-tc"
	nl "github.com/mdlayher/netlink"
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
	name                  string = "packetparser"
	toEndpoint            string = "toEndpoint"
	fromEndpoint          string = "fromEndpoint"
	workers               int    = 2
	buffer                int    = 10000
	bpfSourceDir          string = "_cprog"
	bpfSourceFileName     string = "packetparser.c"
	bpfObjectFileName     string = "packetparser_bpf.o"
	dynamicHeaderFileName string = "dynamic.h"
	tcFilterPriority      uint16 = 0x1
)

type interfaceType string

const (
	Veth   interfaceType = "veth"
	Device interfaceType = "device"
)

const (
	// eventFlushInterval is the maximum time that an event waits in the kernel buffer.
	// The kernel does not wake the reader for each event, so the reader also reads the
	// buffer each time this interval expires.
	eventFlushInterval = 100 * time.Millisecond
	// wakeupDivisor sets the quantity of unread data at which the kernel wakes the reader:
	// 1/wakeupDivisor of the buffer size. For a perf buffer, the size is that of one CPU.
	wakeupDivisor = 4
	// maxPendingRecords is the maximum number of records that the kernel holds before it
	// wakes the reader, and the maximum size of a batch. The two together fit in the
	// records channel. Thus the reader does not drop the records of one flush interval
	// when the workers have the capacity for the event rate.
	maxPendingRecords = buffer / 2

	eventSize = int(unsafe.Sizeof(packetparserPacket{}))
	// A perf buffer record has an 8-byte header and a 4-byte size before the event.
	// A ring buffer record has an 8-byte header. The kernel pads the two to 8 bytes.
	perfRecordSize    = 8 + (4+eventSize+7)/8*8
	ringBufRecordSize = (8 + eventSize + 7) / 8 * 8
)

const (
	ringBufMinKernelMajor = 5
	ringBufMinKernelMinor = 8
	ringBufMinKernelPatch = 0
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

// attachmentKey uniquely identifies a network interface for BPF program attachment.
type attachmentKey struct {
	name         string
	hardwareAddr string
	netNs        int
}

// attachmentType represents the method used to attach BPF programs.
type attachmentType int

const (
	attachmentTypeTC  attachmentType = iota // Traditional TC with clsact qdisc
	attachmentTypeTCX                       // TCX (TC eXpress) - kernel 6.6+
)

// attachmentValue stores the attachment details for a network interface.
type attachmentValue struct {
	// TC-specific fields
	tc    nltc
	qdisc *tc.Object
	// TCX-specific fields
	attachmentType attachmentType
	tcxIngressLink link.Link
	tcxEgressLink  link.Link
}

//go:generate go run go.uber.org/mock/mockgen@v0.4.0 -source=types_linux.go -destination=mocks/mock_types_linux.go -package=mocks -exclude_interfaces=perfReader

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
	Read() (perfRecord, error)
	Close() error
}

type perfRecord struct {
	CPU         int
	LostSamples uint64
	RawSample   []byte
	Remaining   int
}

type packetParser struct {
	cfg        *kcfg.Config
	l          *log.ZapLogger
	callbackID string
	objs       *packetparserObjects //nolint:typecheck
	// attachmentMap is a map of interface key to attachment details (TC or TCX).
	attachmentMap *sync.Map
	reader        perfReader
	enricher      enricher.EnricherInterface
	// interfaceLockMap is a map of key to *sync.Mutex.
	interfaceLockMap    *sync.Map
	endpointIngressInfo *ebpf.ProgramInfo
	endpointEgressInfo  *ebpf.ProgramInfo
	hostIngressInfo     *ebpf.ProgramInfo
	hostEgressInfo      *ebpf.ProgramInfo
	wg                  sync.WaitGroup
	recordsChannel      chan perfRecord
	// batch holds the records that readData read in the current flush interval, and
	// batchStart is the time of its first record. Only the reader goroutine uses them.
	batch      []perfRecord
	batchStart time.Time
	// stopReporter cancels reportLostEvents and reporterWg waits for it to exit. Stop uses
	// both before closing objs, since the reporter reads a map from objs.
	stopReporter    context.CancelFunc
	reporterWg      sync.WaitGroup
	externalChannel chan *v1.Event
	tcxSupported    bool // Whether TCX is supported on this system
}

func ifaceToKey(iface netlink.LinkAttrs) attachmentKey {
	return attachmentKey{
		name:         iface.Name,
		hardwareAddr: iface.HardwareAddr.String(),
		netNs:        iface.NetNsID,
	}
}
