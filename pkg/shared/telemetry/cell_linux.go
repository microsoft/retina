package telemetry

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cilium/hive/cell"
	"github.com/microsoft/retina/pkg/telemetry"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

const heartbeatInterval = 15 * time.Minute

type Config struct {
	Component             string
	EnableTelemetry       bool
	ApplicationInsightsID string
	RetinaVersion         string
	// EnabledPlugins is optional
	EnabledPlugins []string
}

type params struct {
	cell.In

	Config Config
	K8sCfg *rest.Config
}

var (
	Constructor = cell.Module(
		"telemetry",
		"provides telemetry",
		cell.Provide(func(p params, l logrus.FieldLogger) (telemetry.Telemetry, error) {
			l.WithFields(logrus.Fields{
				"app-insights-id": p.Config.ApplicationInsightsID,
				"retina-version":  p.Config.RetinaVersion,
			}).Info("configuring telemetry")

			if p.Config.EnableTelemetry {
				if p.Config.ApplicationInsightsID == "" {
					l.Info("cannot enable telemetry: empty app insights id")
					return telemetry.NewNoopTelemetry(), nil
				}

				l.Info("telemetry enabled")

				// initialize Application Insights
				telemetry.InitAppInsights(p.Config.ApplicationInsightsID, p.Config.RetinaVersion)

				properties := map[string]string{
					"version":   p.Config.RetinaVersion,
					"apiserver": p.K8sCfg.Host,
				}
				if len(p.Config.EnabledPlugins) > 0 {
					properties["plugins"] = strings.Join(p.Config.EnabledPlugins, `,`)
				}

				tel, err := telemetry.NewAppInsightsTelemetryClient(p.Config.Component, properties)
				if err != nil {
					return nil, fmt.Errorf("failed to create telemetry client: %w", err)
				}
				return tel, nil
			}

			l.Info("telemetry disabled")
			return telemetry.NewNoopTelemetry(), nil
		}),
	)

	Heartbeat = cell.Module(
		"heartbeat",
		"sends periodic telemetry heartbeat",
		cell.Invoke(
			func(tel telemetry.Telemetry, lifecycle cell.Lifecycle, l logrus.FieldLogger) {
				ctx, cancelCtx := context.WithCancel(context.Background())
				lifecycle.Append(cell.Hook{
					OnStart: func(cell.HookContext) error {
						l.Info("starting periodic heartbeat")
						go tel.Heartbeat(ctx, heartbeatInterval)
						return nil
					},
					OnStop: func(cell.HookContext) error {
						cancelCtx()
						return nil
					},
				})
			},
		),
	)
)
