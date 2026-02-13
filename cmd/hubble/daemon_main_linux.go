// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
// Copyright Authors of Cilium.
// Modified by Authors of Retina.
// This bootstraps Hubble control plane.
package hubble

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/cilium/cilium/pkg/hive"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	hubblecell "github.com/cilium/cilium/pkg/hubble/cell"
	"github.com/cilium/cilium/pkg/ipcache"
	k8sClient "github.com/cilium/cilium/pkg/k8s/client"
	"github.com/cilium/cilium/pkg/k8s/watchers"
	"github.com/cilium/cilium/pkg/logging"
	"github.com/cilium/cilium/pkg/logging/logfields"
	"github.com/cilium/cilium/pkg/metrics"
	monitorAgent "github.com/cilium/cilium/pkg/monitor/agent"
	"github.com/cilium/cilium/pkg/node"
	"github.com/cilium/cilium/pkg/option"
	"github.com/cilium/cilium/pkg/promise"
	"github.com/cilium/cilium/pkg/time"
	"github.com/cilium/ebpf/rlimit"
	"github.com/cilium/hive/cell"

	"github.com/microsoft/retina/internal/buildinfo"
	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/managers/pluginmanager"
	"github.com/microsoft/retina/pkg/managers/servermanager"
	sharedconfig "github.com/microsoft/retina/pkg/shared/config"
	"github.com/microsoft/retina/pkg/telemetry"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	configFileName string = "config.yaml"
	logFileName    string = "retina.log"
)

func InitGlobalFlags(cmd *cobra.Command, vp *viper.Viper) {
	flags := cmd.Flags()

	flags.String(option.IdentityAllocationMode, option.IdentityAllocationModeCRD, "Identity allocation mode")

	// Add all the flags Hubble supports currently.
	flags.String(option.ConfigDir, "/retina/config", `Configuration directory that contains a file for each option`)
	option.BindEnv(vp, option.ConfigDir)

	// Set the default value for the endpoint GC interval to 0, which disables it.
	vp.Set(option.EndpointGCInterval, 0)

	if err := vp.BindPFlags(flags); err != nil {
		logging.Fatal(logger, fmt.Sprintf("BindPFlags failed: %s", err))
	}
}

type daemonParams struct {
	cell.In

	Lifecycle     cell.Lifecycle
	Clientset     k8sClient.Clientset
	MonitorAgent  monitorAgent.Agent
	PluginManager *pluginmanager.PluginManager
	HTTPServer    *servermanager.HTTPServer
	Log           *slog.Logger
	Client        client.Client
	EventChan     chan *v1.Event
	K8sWatcher    *watchers.K8sWatcher
	Lnds          *node.LocalNodeStore
	IPC           *ipcache.IPCache
	Telemetry     telemetry.Telemetry
	Hubble        hubblecell.HubbleIntegration
	Config        config.Config
}

func newDaemonPromise(params daemonParams) promise.Promise[*Daemon] {
	daemonResolver, daemonPromise := promise.New[*Daemon]()

	// daemonCtx is the daemon-wide context cancelled when stopping.
	daemonCtx, cancelDaemonCtx := context.WithCancel(context.Background())

	var daemon *Daemon
	params.Lifecycle.Append(cell.Hook{
		OnStart: func(cell.HookContext) error {
			d := newDaemon(&params)
			daemon = d
			daemonResolver.Resolve(daemon)

			d.log.Info("starting Retina version: ", "version", buildinfo.Version)
			err := d.Run(daemonCtx)
			if err != nil {
				return fmt.Errorf("daemon run failed: %w", err)
			}

			return nil
		},
		OnStop: func(cell.HookContext) error {
			cancelDaemonCtx()
			return nil
		},
	})
	return daemonPromise
}

func initLogging() {
	retinaConfig, _ := getRetinaConfig()
	k8sCfg, _ := sharedconfig.GetK8sConfig()
	zapLogger := setupZapLogger(retinaConfig, k8sCfg)
	setupLoggingHooks(zapLogger)
	bootstrapLogging()
}

func getRetinaConfig() (*config.Config, error) {
	retinaConfigFile := filepath.Join(option.Config.ConfigDir, configFileName)
	conf, err := config.GetConfig(retinaConfigFile)
	if err != nil {
		logging.DefaultSlogLogger.Error("Failed to get config file", "error", err)
		return nil, fmt.Errorf("getting config from file %q: %w", configFileName, err)
	}
	return conf, nil
}

func setupZapLogger(retinaConfig *config.Config, k8sCfg *rest.Config) *log.ZapLogger {
	logOpts := &log.LogOpts{
		Level:                 retinaConfig.LogLevel,
		File:                  false,
		FileName:              logFileName,
		MaxFileSizeMB:         100, //nolint:gomnd // this is obvious from usage
		MaxBackups:            3,   //nolint:gomnd // this is obvious from usage
		MaxAgeDays:            30,  //nolint:gomnd // this is obvious from usage
		ApplicationInsightsID: buildinfo.ApplicationInsightsID,
		EnableTelemetry:       retinaConfig.EnableTelemetry,
	}

	persistentFields := []zap.Field{
		zap.String("version", buildinfo.Version),
		zap.String("apiserver", k8sCfg.Host),
		zap.Strings("plugins", retinaConfig.EnabledPlugin),
		zap.String("data aggregation level", retinaConfig.DataAggregationLevel.String()),
	}

	_, err := log.SetupZapLogger(logOpts, persistentFields...)
	if err != nil {
		logging.Fatal(logger, fmt.Sprintf("Failed to setup zap logger: %v", err))
	}

	namedLogger := log.Logger().Named("retina-with-hubble")
	namedLogger.Info("Traces telemetry initialized with zapai", zap.String("version", buildinfo.Version), zap.String("appInsightsID", buildinfo.ApplicationInsightsID))

	return namedLogger
}

func setupLoggingHooks(zapLogger *log.ZapLogger) {
	// Add metrics logging hook to slog
	logging.AddHandlers(metrics.NewLoggingHook())

	// Note: The zap hook for logrus is no longer supported since Cilium moved to slog
	_ = zapLogger
}

func bootstrapLogging() {
	if err := logging.SetupLogging(option.Config.LogDriver, logging.LogOptions(option.Config.LogOpt), "retina-agent", option.Config.Debug); err != nil {
		logging.Fatal(logging.DefaultSlogLogger, err.Error())
	}
}

func initDaemonConfig(vp *viper.Viper) {
	// slogloggercheck: using default logger for configuration initialization
	option.Config.Populate(logging.DefaultSlogLogger, vp)

	time.MaxInternalTimerDelay = vp.GetDuration(option.MaxInternalTimerDelay)
}

func Execute(cobraCmd *cobra.Command, h *hive.Hive) {
	// slogloggercheck: using default logger for configuration initialization
	fn := option.InitConfig(logging.DefaultSlogLogger, cobraCmd, "retina-agent", "retina", h.Viper())
	fn()
	initDaemonConfig(h.Viper())
	initLogging()

	// Set up unified slog backed by zap (routes to stdout + Application Insights)
	log.SetDefaultSlog()
	hiveLogger := log.SlogLogger()

	// Allow the current process to lock memory for eBPF resources.
	if err := rlimit.RemoveMemlock(); err != nil {
		logging.Fatal(logger, "failed to remove memlock", logfields.Error, err)
	}

	//nolint:gocritic // without granular commits this commented-out code may be lost
	// initEnv(h.Viper())

	if err := h.Run(hiveLogger); err != nil {
		logging.Fatal(logger, "Hive Run failed", logfields.Error, err)
	}
}
