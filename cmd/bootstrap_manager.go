// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package cmd

import (
	"fmt"
	"strings"

	"github.com/microsoft/retina/cmd/standalone"
	"github.com/microsoft/retina/cmd/standard"
	"github.com/microsoft/retina/internal/buildinfo"
	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/telemetry"
	"go.uber.org/zap"
)

const (
	logFileName = "retina.log"
)

type BootstrapManager struct {
	metricsAddr          string
	probeAddr            string
	enableLeaderElection bool
	configFile           string
}

func NewBootstrapManager(metricsAddr, probeAddr, configFile string, enableLeaderElection bool) *BootstrapManager {
	return &BootstrapManager{
		metricsAddr:          metricsAddr,
		probeAddr:            probeAddr,
		enableLeaderElection: enableLeaderElection,
		configFile:           configFile,
	}
}

func (b *BootstrapManager) Start() error {
	if buildinfo.ApplicationInsightsID != "" {
		telemetry.InitAppInsights(buildinfo.ApplicationInsightsID, buildinfo.Version)
		defer telemetry.ShutdownAppInsights()
		defer telemetry.TrackPanic()
	}

	daemonConfig, err := config.GetConfig(b.configFile)
	if err != nil {
		panic(err)
	}

	fmt.Println("init logger")
	zl, err := log.SetupZapLogger(&log.LogOpts{
		Level:                 daemonConfig.LogLevel,
		File:                  false,
		FileName:              logFileName,
		MaxFileSizeMB:         100, //nolint:gomnd // defaults
		MaxBackups:            3,   //nolint:gomnd // defaults
		MaxAgeDays:            30,  //nolint:gomnd // defaults
		ApplicationInsightsID: buildinfo.ApplicationInsightsID,
		EnableTelemetry:       daemonConfig.EnableTelemetry,
	},
		zap.String("version", buildinfo.Version),
		zap.String("plugins", strings.Join(daemonConfig.EnabledPlugin, `,`)),
		zap.String("data aggregation level", daemonConfig.DataAggregationLevel.String()),
	)
	if err != nil {
		panic(err)
	}
	defer zl.Close()

	if daemonConfig.EnableStandalone {
		sd := standalone.NewDaemon(daemonConfig)
		if err = sd.Start(zl); err != nil {
			return fmt.Errorf("starting standalone daemon: %w", err)
		}
		return nil
	}

	d := standard.NewDaemon(daemonConfig, b.metricsAddr, b.probeAddr, b.enableLeaderElection)
	if err := d.Start(zl); err != nil {
		return fmt.Errorf("starting daemon: %w", err)
	}
	return nil
}
