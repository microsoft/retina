// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package observability

import (
	"fmt"
	"strings"

	"github.com/microsoft/retina/internal/buildinfo"
	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/telemetry"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
)

const (
	logFileName = "retina.log"
)

func InitializeTelemetryClient(restCfg *rest.Config, enabledPlugin []string, enableTelemetry bool, l *zap.SugaredLogger) (telemetry.Telemetry, error) {
	if enableTelemetry {
		if buildinfo.ApplicationInsightsID == "" {
			panic("telemetry enabled, but ApplicationInsightsID is empty")
		}
		l.Info("telemetry enabled", zap.String("applicationInsightsID", buildinfo.ApplicationInsightsID))

		var tel telemetry.Telemetry
		var err error
		if restCfg != nil {
			tel, err = telemetry.NewAppInsightsTelemetryClient("retina-agent", map[string]string{
				"version":   buildinfo.Version,
				"apiserver": restCfg.Host,
				"plugins":   strings.Join(enabledPlugin, `,`),
			})
		} else {
			tel, err = telemetry.NewAppInsightsTelemetryClient("standalone-retina-agent", map[string]string{
				"version": buildinfo.Version,
				"plugins": strings.Join(enabledPlugin, `,`),
			})
		}
		if err != nil {
			l.Error("failed to create telemetry client", zap.Error(err))
			return tel, fmt.Errorf("error when creating telemetry client: %w", err)
		}
		return tel, nil
	}

	l.Info("telemetry disabled")
	tel := telemetry.NewNoopTelemetry()
	return tel, nil
}

func InitializeLogger(logLevel string, enableTelemetry bool, enabledPlugin []string, dataAggregationLevel config.Level) *log.ZapLogger {
	if buildinfo.ApplicationInsightsID != "" {
		telemetry.InitAppInsights(buildinfo.ApplicationInsightsID, buildinfo.Version)
		defer telemetry.ShutdownAppInsights()
		defer telemetry.TrackPanic()
	}

	fmt.Println("init logger")
	zl, err := log.SetupZapLogger(&log.LogOpts{
		Level:                 logLevel,
		File:                  false,
		FileName:              logFileName,
		MaxFileSizeMB:         100, //nolint:gomnd // defaults
		MaxBackups:            3,   //nolint:gomnd // defaults
		MaxAgeDays:            30,  //nolint:gomnd // defaults
		ApplicationInsightsID: buildinfo.ApplicationInsightsID,
		EnableTelemetry:       enableTelemetry,
	},
		zap.String("version", buildinfo.Version),
		zap.String("plugins", strings.Join(enabledPlugin, `,`)),
		zap.String("data aggregation level", dataAggregationLevel.String()),
	)
	if err != nil {
		panic(err)
	}
	defer zl.Close()

	return zl
}
