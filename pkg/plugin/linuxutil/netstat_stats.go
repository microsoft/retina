// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package linuxutil

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/cakturk/go-netstat/netstat"
	"go.uber.org/zap"
)

const (
	pathNetNetstat = "/proc/net/netstat"
	pathNetSnmp    = "/proc/net/snmp"
)

type NetstatReader struct {
	l          *log.ZapLogger
	connStats  *ConnectionStats
	ifaceStats *IfaceStats
	opts       *NetstatOpts
	ns         NetstatInterface
}

func NewNetstatReader(opts *NetstatOpts, ns NetstatInterface) *NetstatReader {
	return &NetstatReader{
		l:          log.Logger().Named(string("NetstatReader")),
		opts:       opts,
		connStats:  &ConnectionStats{},
		ifaceStats: &IfaceStats{},
		ns:         ns,
	}
}

func (nr *NetstatReader) readAndUpdate() error {
	if err := nr.readConnectionStats(pathNetNetstat); err != nil {
		return err
	}

	// Get Socket stats
	if err := nr.readSockStats(); err != nil {
		return err
	}

	nr.updateMetrics()
	nr.l.Debug("Done reading and updating connections stats")

	return nil
}

func (nr *NetstatReader) readConnectionStats(path string) error {
	// Read the contents of the file into a string
	data, err := os.ReadFile(path)
	if err != nil {
		nr.l.Error("Error while reading netstat path file: \n", zap.Error(err))
		return err
	}

	// Split the string into lines
	lines := strings.Split(string(data), "\n")

	if len(lines) < 2 && len(lines)%2 != 0 {
		return fmt.Errorf("invalid netstat file")
	}

	for i := 0; i < len(lines); i += 2 {
		fields1 := strings.Fields(lines[i])
		if len(fields1) < 2 {
			continue
		}

		fields2 := strings.Fields(lines[i+1])
		if len(fields2) < 2 {
			continue
		}

		if fields1[0] != fields2[0] {
			continue
		}

		if len(fields1) != len(fields2) {
			continue
		}

		if strings.HasPrefix(fields1[0], "TcpExt") && strings.HasPrefix(fields2[0], "TcpExt") {
			nr.l.Debug("TcpExt found for netstat ")
			nr.connStats.TcpExt = nr.processConnFields(fields1, fields2)
		} else if strings.HasPrefix(fields1[0], "IpExt") && strings.HasPrefix(fields2[0], "IpExt") {
			nr.l.Debug("IpExt found for netstat ")
			nr.connStats.IpExt = nr.processConnFields(fields1, fields2)
		} else if strings.HasPrefix(fields1[0], "MPTcpExt") && strings.HasPrefix(fields2[0], "MPTcpExt") {
			nr.l.Debug("MPTcpExt found for netstat ")
			nr.connStats.MPTcpExt = nr.processConnFields(fields1, fields2)
		} else {
			nr.l.Info("Unknown field found for netstat ", zap.Any("F1", fields1[0]), zap.Any("F2", fields2[0]))
			continue
		}

	}

	return nil
}

func (nr *NetstatReader) processConnFields(f1, f2 []string) map[string]uint64 {
	stats := make(map[string]uint64)

	for i := 1; i < len(f1); i++ {
		num, err := strconv.ParseUint(f2[i], 10, 64)
		if err != nil {
			continue
		}

		if _, ok := netstatCuratedKeys[f1[i]]; nr.opts.CuratedKeys && !ok {
			continue
		}

		if num == 0 && !nr.opts.AddZeroVal {
			continue
		}

		stats[f1[i]] = num
	}

	return stats
}

func (nr *NetstatReader) readSockStats() error {
	filter := netstat.NoopFilter
	if nr.opts.ListenSock {
		filter = func(s *netstat.SockTabEntry) bool {
			return s.State == netstat.Listen
		}
	}

	// UDP sockets
	socks, err := nr.ns.UDPSocks(filter)
	if err != nil {
		nr.l.Error("netstat 1: \n", zap.Error(err))
		return err
	} else {
		sockStats := processSocks(socks)
		nr.connStats.UdpSockets = *sockStats
	}

	// TCP sockets
	socks, err = nr.ns.TCPSocks(filter)
	if err != nil {
		nr.l.Error("netstat 2: \n", zap.Error(err))
		return err
	} else {
		sockStats := processSocks(socks)
		nr.connStats.TcpSockets = *sockStats
	}

	return nil
}

func processSocks(socks []netstat.SockTabEntry) *SocketStats {
	stats := &SocketStats{
		totalActiveSockets: len(socks),
		socketByState:      make(map[string]int),
		socketByRemoteAddr: make(map[string]int),
	}

	for _, e := range socks {
		stats.socketByState[e.State.String()]++
		stats.socketByRemoteAddr[e.RemoteAddr.String()]++
	}

	return stats
}

func (nr *NetstatReader) updateMetrics() {
	if nr.connStats == nil {
		nr.l.Info("No connection stats found")
		return
	}
	// Adding TCP Connection Stats
	for statName, val := range nr.connStats.TcpExt {
		metrics.TCPConnectionStats.WithLabelValues(statName).Set(float64(val))
	}

	// Adding IP Stats
	for statName, val := range nr.connStats.IpExt {
		metrics.IPConnectionStats.WithLabelValues(statName).Set(float64(val))
	}

	// Adding MPTCP Stats
	for statName, val := range nr.connStats.MPTcpExt {
		metrics.TCPConnectionStats.WithLabelValues(statName).Set(float64(val))
	}

	// TCP COnnection State and remote addr metrics
	for state, v := range nr.connStats.TcpSockets.socketByState {
		metrics.TCPStateGauge.WithLabelValues(state).Set(float64(v))
	}

	for remoteAddr, v := range nr.connStats.TcpSockets.socketByRemoteAddr {
		addr := ""
		port := ""
		splitAddr := strings.Split(remoteAddr, ":")
		if len(splitAddr) == 2 {
			addr = splitAddr[0]
			port = splitAddr[1]
		} else {
			addr = remoteAddr
		}
		if !validateRemoteAddr(addr) {
			continue
		}

		metrics.TCPConnectionRemoteGauge.WithLabelValues(addr, port).Set(float64(v))
	}

	// UDP COnnection State metrics
	metrics.UDPConnectionStats.WithLabelValues(utils.Active).Set(float64(nr.connStats.UdpSockets.totalActiveSockets))
}

func validateRemoteAddr(addr string) bool {
	if addr == "" {
		return false
	}

	// ignore localhost addresses.
	if strings.Contains(addr, "127.0.0") {
		return false
	}

	return true
}
