// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package linuxutil

import (
	"github.com/cakturk/go-netstat/netstat"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/plugin/api"
)

const (
	Name api.PluginName = "linuxutil"
)

//go:generate mockgen -source=types_linux.go -destination=linuxutil_mock_generated.go -package=linuxutil
type linuxUtil struct {
	cfg              *kcfg.Config
	l                *log.ZapLogger
	isRunning        bool
	prevTCPSockStats *SocketStats
}

var netstatCuratedKeys = map[string]struct{}{
	"ListenDrops":        {},
	"LockDroppedIcmps":   {},
	"PFMemallocDrop":     {},
	"TCPBacklogDrop":     {},
	"TCPDeferAcceptDrop": {},
	"TCPMinTTLDrop":      {},
	"TCPRcvQDrop":        {},
	"TCPReqQFullDrop":    {},
	"TCPZeroWindowDrop":  {},
	"InCsumErrors":       {},
	"DataCsumErr":        {},
	"AddAddrDrop":        {},
	"RmAddrDrop":         {},
	"TCPTimeouts": 	      {},
	"TCPLossProbes":      {},
	"TCPLostRetransmit":  {},
}

type ConnectionStats struct {
	// https://github.com/ecki/net-tools/blob/master/statistics.c#L206
	TcpExt   map[string]uint64 `json:"tcp_ext"`
	IpExt    map[string]uint64 `json:"ip_ext"`
	MPTcpExt map[string]uint64 `json:"mptcp_ext"`
	// Socket stats
	UdpSockets SocketStats `json:"udp_sockets"`
	TcpSockets SocketStats `json:"tcp_sockets"`
}

type IfaceStats struct {
	Name string
	// Inbound stats
	RxBytes      uint64
	RxPackets    uint64
	RxErrs       uint64
	RxDrop       uint64
	RxFIFO       uint64
	RxFrame      uint64
	RxCompressed uint64
	RxMulticast  uint64
	// Outbound stats
	TxBytes      uint64
	TxPackets    uint64
	TxErrs       uint64
	TxDrop       uint64
	TxFIFO       uint64
	TxColls      uint64
	TxCarrier    uint64
	TxCompressed uint64
}

type SocketStats struct {
	totalActiveSockets int
	// count of sockets opened by state
	socketByState map[string]int
	// count of sockets opened by remote address
	socketByRemoteAddr map[string]int
}

type NetstatOpts struct {
	// when true only includes curated list of keys
	CuratedKeys bool

	// when true will include all keys with value 0
	AddZeroVal bool

	// get only listening sockets
	ListenSock bool

	// previous TCP socket stats
	PrevTCPSockStats *SocketStats
}

type EthtoolStats struct {
	// Stats by interface name and stat name
	stats map[string]map[string]uint64
}

type EthtoolOpts struct {
	// when true will only include keys with err or drop in its name
	errOrDropKeysOnly bool

	// when true will include all keys with value 0
	addZeroVal bool

	// Configurable limit for unsupported interfaces cache
	limit uint
}

type EthtoolInterface interface {
	Stats(intf string) (map[string]uint64, error)
	Close()
}

type NetstatInterface interface {
	UDPSocks(accept netstat.AcceptFn) ([]netstat.SockTabEntry, error)
	TCPSocks(accept netstat.AcceptFn) ([]netstat.SockTabEntry, error)
}

type Netstat struct{}

func (n *Netstat) UDPSocks(accept netstat.AcceptFn) ([]netstat.SockTabEntry, error) {
	return netstat.UDPSocks(accept)
}

func (n *Netstat) TCPSocks(accept netstat.AcceptFn) ([]netstat.SockTabEntry, error) {
	return netstat.TCPSocks(accept)
}
