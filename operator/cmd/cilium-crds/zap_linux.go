// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package ciliumcrds

import (
	"log/slog"

	"github.com/cilium/cilium/pkg/option"
	"github.com/cilium/hive/cell"
	"github.com/microsoft/retina/internal/buildinfo"
	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"

	"github.com/microsoft/retina/operator/cilium-crds/config"
)

type params struct {
	cell.In

	Logger      *slog.Logger
	K8sCfg      *rest.Config
	DaemonCfg   *option.DaemonConfig
	OperatorCfg config.Config
}

func setupZapHook(p params) {
	// Note: Zap logger is now initialized in initEnv() before hive starts.
	// This hook only logs startup info with operator-specific fields from the hive DI context.
	namedLogger := log.Logger().Named("retina-operator-v2")
	namedLogger.Info("Traces telemetry initialized with zapai",
		zap.String("version", buildinfo.Version),
		zap.String("appInsightsID", buildinfo.ApplicationInsightsID),
		zap.String("apiserver", p.K8sCfg.Host),
	)
}
