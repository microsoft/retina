// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package linuxutil

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/cakturk/go-netstat/netstat"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/utils"
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

	// The netstat proc file (typically found at /proc/net/netstat) is composed
	// of pairs of lines describing various statistics. The reference
	// implementation for this file is found at
	// https://sourceforge.net/p/net-tools/code/ci/master/tree/statistics.c.
	// Given that these statistics are separated across lines the file must first
	// be divided into lines in order to be processed:
	lines := strings.Split(string(data), "\n")

	// files often end with a trailing newline. After splitting, this would
	// present itself as a single empty string at the end. If this is the case,
	// we want to omit this case from the logic that follows
	if last := len(lines) - 1; lines[last] == "" {
		lines = lines[0:last]
	}

	if len(lines) == 1 {
		return fmt.Errorf("invalid netstat file")
	}

	// Each pair of lines must then be considered together to properly extract
	// statistics:
	for i := 0; i < len(lines); i += 2 {
		// the format of each stat line pair begins with some signifier like
		// "TcpExt:" followed by one or more statistics. The first line contains
		// the headers for these statistics and the second line contains the
		// corresponding value in the same position. In order to access each
		// statistic, both of these lines must be processed into sets of
		// whitespace-delineated fields:
		headers := strings.Fields(lines[i])
		if len(headers) < 2 {
			continue
		}

		values := strings.Fields(lines[i+1])
		if len(values) < 2 {
			continue
		}

		// The signifiers for each pair of headers and values must match in order
		// to be properly considered together.
		if headers[0] != values[0] {
			continue
		}

		// Also, the set of statistics is malformed if there is not a corresponding
		// header for each value:
		if len(headers) != len(values) {
			continue
		}

		// knowing that there are two well-formed sets of statistics, it's now
		// possible to examine the signifier and process the statistics into a
		// semantic collection:
		if strings.HasPrefix(headers[0], "TcpExt") && strings.HasPrefix(values[0], "TcpExt") {
			nr.l.Debug("TcpExt found for netstat ")
			nr.connStats.TcpExt = nr.processConnFields(headers, values)
		} else if strings.HasPrefix(headers[0], "IpExt") && strings.HasPrefix(values[0], "IpExt") {
			nr.l.Debug("IpExt found for netstat ")
			nr.connStats.IpExt = nr.processConnFields(headers, values)
		} else if strings.HasPrefix(headers[0], "MPTcpExt") && strings.HasPrefix(values[0], "MPTcpExt") {
			nr.l.Debug("MPTcpExt found for netstat ")
			nr.connStats.MPTcpExt = nr.processConnFields(headers, values)
		} else {
			nr.l.Info("Unknown field found for netstat ", zap.Any("F1", headers[0]), zap.Any("F2", values[0]))
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
