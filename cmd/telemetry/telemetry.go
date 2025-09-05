// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package telemetry

import (
	"fmt"
	"strings"

	"github.com/microsoft/retina/internal/buildinfo"
	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/telemetry"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
)

func InitializeTelemetryClient(restCfg *rest.Config, dcfg *config.Config, ml *zap.SugaredLogger) (telemetry.Telemetry, error) {
	if dcfg.EnableTelemetry {
		if buildinfo.ApplicationInsightsID == "" {
			panic("telemetry enabled, but ApplicationInsightsID is empty")
		}
		ml.Info("telemetry enabled", zap.String("applicationInsightsID", buildinfo.ApplicationInsightsID))

		var tel telemetry.Telemetry
		var err error
		if restCfg != nil {
			tel, err = telemetry.NewAppInsightsTelemetryClient("retina-agent", map[string]string{
				"version":   buildinfo.Version,
				"apiserver": restCfg.Host,
				"plugins":   strings.Join(dcfg.EnabledPlugin, `,`),
			})
		} else {
			tel, err = telemetry.NewAppInsightsTelemetryClient("standalone-retina-agent", map[string]string{
				"version": buildinfo.Version,
				"plugins": strings.Join(dcfg.EnabledPlugin, `,`),
			})
		}
		if err != nil {
			ml.Error("failed to create telemetry client", zap.Error(err))
			return tel, fmt.Errorf("error when creating telemetry client: %w", err)
		}
		return tel, nil
	}

	ml.Info("telemetry disabled")
	tel := telemetry.NewNoopTelemetry()
	return tel, nil
}
