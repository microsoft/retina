// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package infiniband

import (
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	pathInfiniband            = "/sys/class/infiniband"
	pathDebugStatusParameters = "/sys/class/net"
)

const (
	InfinibandDevicePrefix = "mlx"
	InfinibandIfacePrefix  = "ib"
)

func NewInfinibandReader() *InfinibandReader {
	return &InfinibandReader{
		l:                log.Logger().Named(string("InfinibandReader")),
		counterStats:     make(map[CounterStat]uint64),
		statusParamStats: make(map[StatusParam]uint64),
	}
}

type InfinibandReader struct { // nolint // clearer naming
	l                *log.ZapLogger
	counterStats     map[CounterStat]uint64
	statusParamStats map[StatusParam]uint64
}

func (ir *InfinibandReader) readAndUpdate() error {
	g := errgroup.Group{}
	ibFS := os.DirFS(pathInfiniband)
	g.Go(func() error { return ir.readCounterStats(ibFS) })

	netFS := os.DirFS(pathDebugStatusParameters)
	g.Go(func() error { return ir.readStatusParamStats(netFS) })

	err := g.Wait()
	ir.updateMetrics()
	ir.l.Debug("Done reading and updating stats")
	return errors.Wrap(err, "error reading and updating stats")
}

func (ir *InfinibandReader) readCounterStats(fsys fs.FS) error {
	devices, err := fs.ReadDir(fsys, ".")
	if err != nil {
		ir.l.Error("error reading dir:", zap.Error(err))
		return err // nolint std. fmt.
	}
	for _, device := range devices {
		portsPath := filepath.Join(".", device.Name(), "ports")
		ports, err := fs.ReadDir(fsys, portsPath) // does the real filesystem c
		if err != nil {
			ir.l.Error("error reading dir:", zap.Error(err))
			continue
		}
		for _, port := range ports {
			countersPath := filepath.Join(portsPath, port.Name(), "counters")
			counters, err := fs.ReadDir(fsys, countersPath)
			if err != nil {
				ir.l.Error("error reading dir:", zap.Error(err))
				continue
			}
			for _, counter := range counters {
				counterPath := filepath.Join(countersPath, counter.Name())
				val, err := fs.ReadFile(fsys, counterPath)
				if err != nil {
					ir.l.Error("Error while reading infiniband file: \n", zap.Error(err))
					continue
				}
				num, err := strconv.ParseUint(strings.TrimSpace(string(val)), 10, 64)
				if err != nil {
					ir.l.Error("error parsing string:", zap.Error(err))
					continue // nolint std. fmt.
				}
				ir.counterStats[CounterStat{Name: counter.Name(), Device: device.Name(), Port: port.Name()}] = num
			}

		}
	}
	return nil
}

func (ir *InfinibandReader) readStatusParamStats(fsys fs.FS) error {
	ifaces, err := fs.ReadDir(fsys, ".")
	if err != nil {
		ir.l.Error("error reading dir:", zap.Error(err))
		return err // nolint std. fmt.
	}
	ir.statusParamStats = make(map[StatusParam]uint64)
	for _, iface := range ifaces {
		if !strings.HasPrefix(iface.Name(), InfinibandIfacePrefix) {
			continue
		}
		statusParamsPath := filepath.Join(".", iface.Name(), "debug")
		statusParams, err := fs.ReadDir(fsys, statusParamsPath)
		if err != nil {
			ir.l.Error("error parsing string:", zap.Error(err))
			continue
		}
		for _, statusParam := range statusParams {
			statusParamPath := filepath.Join(statusParamsPath, statusParam.Name())
			val, err := fs.ReadFile(fsys, statusParamPath)
			if err != nil {
				ir.l.Error("Error while reading infiniband path file: \n", zap.Error(err))
				continue
			}
			num, err := strconv.ParseUint(string(val), 10, 64)
			if err != nil {
				ir.l.Error("Error while reading infiniband file: \n", zap.Error(err))
				return err // nolint std. fmt.
			}
			ir.statusParamStats[StatusParam{Name: statusParam.Name(), Iface: iface.Name()}] = num

		}
	}
	return nil
}

func (ir *InfinibandReader) updateMetrics() {
	// Adding counter stats
	for counter, val := range ir.counterStats {
		metrics.InfinibandCounterStats.WithLabelValues(counter.Name, counter.Device, counter.Port).Set(float64(val))
	}

	// Adding status params
	for statusParam, val := range ir.statusParamStats {
		metrics.InfinibandStatusParams.WithLabelValues(statusParam.Name, statusParam.Iface).Set(float64(val))
	}
}
