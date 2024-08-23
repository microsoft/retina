// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package linuxutil

import (
	"errors"
	"net"
	"strings"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/safchain/ethtool"
	"go.uber.org/zap"
)

type EthtoolReader struct {
	l         *log.ZapLogger
	opts      *EthtoolOpts
	data      *EthtoolStats
	ethHandle EthtoolInterface
}

func NewEthtoolReader(opts *EthtoolOpts, ethHandle EthtoolInterface) *EthtoolReader {
	if ethHandle == nil {
		var err error
		ethHandle, err = ethtool.NewEthtool()
		if err != nil {
			log.Logger().Error("Error while creating ethtool handle: %v\n", zap.Error(err))
			return nil
		}
	}
	// Construct a cached ethtool handle
	CachedEthHandle := NewCachedEthtool(ethHandle, opts)
	return &EthtoolReader{
		l:         log.Logger().Named(string("EthtoolReader")),
		opts:      opts,
		data:      &EthtoolStats{},
		ethHandle: CachedEthHandle,
	}
}

func (er *EthtoolReader) readAndUpdate() error {
	if err := er.readInterfaceStats(); err != nil {
		return err
	}

	er.updateMetrics()
	er.l.Debug("Done reading and updating interface stats")

	return nil
}

func (er *EthtoolReader) readInterfaceStats() error {

	ifaces, err := net.Interfaces()
	if err != nil {
		er.l.Error("Error while getting all interfaces: %v\n", zap.Error(err))
		return err
	}

	defer er.ethHandle.Close()

	er.data = &EthtoolStats{
		stats: make(map[string]map[string]uint64),
	}

	for _, i := range ifaces {
		// exclude loopback interface and bridge network interface
		if strings.Contains(i.Name, "lo") || strings.Contains(i.Name, "cbr0") {
			continue
		}

		// Retrieve tx from eth0
		ifaceStats, err := er.ethHandle.Stats(i.Name)

		if err != nil {
			if errors.Is(err, errskip) {
				er.l.Debug("Skipping unsupported interface", zap.String("ifacename", i.Name))
			} else {
				er.l.Error("Error while getting ethtool:", zap.String("ifacename", i.Name), zap.Error(err))
			}
			continue
		}

		er.data.stats[i.Name] = make(map[string]uint64)
		tempMap := er.processStats(ifaceStats)
		er.data.stats[i.Name] = tempMap

		er.l.Debug("Processed ethtool Stats ", zap.String("ifacename", i.Name))

	}

	return nil
}

func (er *EthtoolReader) processStats(ifaceStats map[string]uint64) map[string]uint64 {
	// process stats section
	newStats := make(map[string]uint64)
	for k, v := range ifaceStats {
		if !er.opts.addZeroVal && v == 0 {
			continue
		}

		if er.opts.errOrDropKeysOnly && !strings.Contains(k, "err") && !strings.Contains(k, "drop") {
			continue
		}

		newStats[k] = v
	}

	return newStats
}

func (er *EthtoolReader) updateMetrics() {
	// update metrics section
	// retrive interfacename and statname from ethStats
	for ifName, stats := range er.data.stats {
		for statName, statVal := range stats {
			metrics.InterfaceStats.WithLabelValues(ifName, statName).Set(float64(statVal))
		}
	}
}
