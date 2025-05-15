// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package dropreason

import (
	"sync"

	hubblev1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/utils"
)

const (
	name                  string = "dropreason"
	bpfSourceDir          string = "_cprog"
	bpfSourceFileName     string = "drop_reason.c"
	bpfObjectFileName     string = "kprobe_bpf.o"
	dynamicHeaderFileName string = "dynamic.h"
	buffer                int    = 10000
	workers               int    = 2
)

// Determined via testing on a large cluster.
// Actual buffer size will be 16 * pagesize.
var perCPUBuffer = 16

type dropReason struct {
	cfg             *kcfg.Config
	l               *log.ZapLogger
	hooks           []link.Link
	metricsMapData  IMap
	isRunning       bool
	reader          IPerfReader
	enricher        enricher.EnricherInterface
	recordsChannel  chan perf.Record
	wg              sync.WaitGroup
	externalChannel chan *hubblev1.Event
}

type fexitObjects struct {
	fexitPrograms
	kprobeMaps
}

type fexitPrograms struct {
	InetCskAcceptFexit      *ebpf.Program `ebpf:"inet_csk_accept_fexit"`
	NfConntrackConfirmFexit *ebpf.Program `ebpf:"nf_conntrack_confirm_fexit"`
	NfHookSlowFexit         *ebpf.Program `ebpf:"nf_hook_slow_fexit"`
	NfNatInetFnFexit        *ebpf.Program `ebpf:"nf_nat_inet_fn_fexit"`
	TcpV4ConnectFexit       *ebpf.Program `ebpf:"tcp_v4_connect_fexit"` // nolint:revive // needs to match generated code
}

type marinerObjects struct {
	marinerPrograms
	kprobeMaps
}

type marinerPrograms struct {
	InetCskAccept      *ebpf.Program `ebpf:"inet_csk_accept"`
	InetCskAcceptRet   *ebpf.Program `ebpf:"inet_csk_accept_ret"`
	InetCskAcceptFexit *ebpf.Program `ebpf:"inet_csk_accept_fexit"`
	NfHookSlow         *ebpf.Program `ebpf:"nf_hook_slow"`
	NfHookSlowRet      *ebpf.Program `ebpf:"nf_hook_slow_ret"`
	NfHookSlowFexit    *ebpf.Program `ebpf:"nf_hook_slow_fexit"`
	TcpV4ConnectRet    *ebpf.Program `ebpf:"tcp_v4_connect_ret"`   // nolint:revive // needs to match generated code
	TcpV4ConnectFexit  *ebpf.Program `ebpf:"tcp_v4_connect_fexit"` // nolint:revive // needs to match generated code
}

type kprobeObjectsOld struct {
	kprobeProgramsOld
	kprobeMaps
}

type kprobeProgramsOld struct {
	InetCskAccept         *ebpf.Program `ebpf:"inet_csk_accept"`
	InetCskAcceptRet      *ebpf.Program `ebpf:"inet_csk_accept_ret"`
	NfConntrackConfirm    *ebpf.Program `ebpf:"nf_conntrack_confirm"`
	NfConntrackConfirmRet *ebpf.Program `ebpf:"nf_conntrack_confirm_ret"`
	NfHookSlow            *ebpf.Program `ebpf:"nf_hook_slow"`
	NfHookSlowRet         *ebpf.Program `ebpf:"nf_hook_slow_ret"`
	NfNatInetFn           *ebpf.Program `ebpf:"nf_nat_inet_fn"`
	NfNatInetFnRet        *ebpf.Program `ebpf:"nf_nat_inet_fn_ret"`
	TcpV4ConnectRet       *ebpf.Program `ebpf:"tcp_v4_connect_ret"` // nolint:revive // needs to match generated code
}

type (
	returnValue uint32
)

// Interface to https://pkg.go.dev/github.com/cilium/ebpf#Map.
// Added for unit tests.
//
//go:generate go run go.uber.org/mock/mockgen@v0.4.0 -source=types_linux.go -destination=mocks/mock_types.go -package=dropreason . IMap IMapIterator IPerfReader
type IMapIterator interface {
	Next(keyOut interface{}, valueOut interface{}) bool
	Err() error
}

type IMap interface {
	Iterate() *ebpf.MapIterator
	Close() error
}

type IPerfReader interface {
	Read() (perf.Record, error)
	Close() error
}

//lint:ignore
type dropMetricKey kprobeMetricsMapKey //nolint:typecheck

//lint:ignore
type dropMetricValues []kprobeMetricsMapValue //nolint:typecheck

func (dk *dropMetricKey) getType() string {
	//nolint:typecheck
	return utils.DropReason(dk.DropType).String()
}

func (dk *dropMetricKey) getDirection() string {
	switch dk.getType() {
	case utils.DropReason_TCP_CONNECT_BASIC.String():
		return "egress"
	case utils.DropReason_TCP_ACCEPT_BASIC.String():
		return "ingress"
	}
	return "unknown"
}

func (dv *dropMetricValues) getPktCountAndBytes() (float64, float64) {
	count := uint64(0)
	bytes := uint64(0)
	for _, v := range *dv {
		count += v.Count
		bytes += v.Bytes
	}
	return float64(count), float64(bytes)
}
