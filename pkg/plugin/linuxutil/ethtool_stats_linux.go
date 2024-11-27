// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package linuxutil

import (
	"errors"
	"net"
	"strings"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/utils"
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
		stats: make(map[string]uint64),
	}

	for _, i := range ifaces {
		// exclude lo (loopback interface), cbr0 (bridge network interface), lxc (Linux containers interface), and azv (virtual interface)
		if strings.Contains(i.Name, "lo") ||
			strings.Contains(i.Name, "cbr0") ||
			strings.Contains(i.Name, "lxc") ||
			strings.Contains(i.Name, "azv") {
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

		tempMap := er.processStats(ifaceStats)
		for key, value := range tempMap {
			er.data.stats[key] += value
		}

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
	// retrive statname from ethStats
	for statName, statVal := range er.data.stats {
		metrics.InterfaceStatsGauge.WithLabelValues(utils.InterfaceNameConstant, statName).Set(float64(statVal))
	}
}
