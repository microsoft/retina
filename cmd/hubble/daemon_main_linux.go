// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
// Copyright Authors of Cilium.
// Modified by Authors of Retina.
// This bootstraps Hubble control plane.
package hubble

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	zaphook "github.com/Sytten/logrus-zap-hook"
	"github.com/cilium/cilium/pkg/hive"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	hubblecell "github.com/cilium/cilium/pkg/hubble/cell"
	"github.com/cilium/cilium/pkg/ipcache"
	"github.com/cilium/cilium/pkg/k8s"
	k8sClient "github.com/cilium/cilium/pkg/k8s/client"
	"github.com/cilium/cilium/pkg/k8s/watchers"
	"github.com/cilium/cilium/pkg/logging"
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
	"github.com/sirupsen/logrus"
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

	if err := vp.BindPFlags(flags); err != nil {
		logger.Fatalf("BindPFlags failed: %s", err)
	}
}

type daemonParams struct {
	cell.In

	Lifecycle     cell.Lifecycle
	Clientset     k8sClient.Clientset
	MonitorAgent  monitorAgent.Agent
	PluginManager *pluginmanager.PluginManager
	HTTPServer    *servermanager.HTTPServer
	Log           logrus.FieldLogger
	Client        client.Client
	EventChan     chan *v1.Event
	K8sWatcher    *watchers.K8sWatcher
	Lnds          *node.LocalNodeStore
	IPC           *ipcache.IPCache
	SvcCache      k8s.ServiceCache
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

			d.log.Info("starting Retina Enterprise version: ", buildinfo.Version)
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
	logger := setupDefaultLogger()
	retinaConfig, _ := getRetinaConfig(logger)
	k8sCfg, _ := sharedconfig.GetK8sConfig()
	zapLogger := setupZapLogger(retinaConfig, k8sCfg)
	setupLoggingHooks(logger, zapLogger)
	bootstrapLogging(retinaConfig, logger)
}

func setupDefaultLogger() *logrus.Logger {
	logger := logging.DefaultLogger
	logger.ReportCaller = true
	logger.SetOutput(io.Discard)
	return logger
}

func getRetinaConfig(logger *logrus.Logger) (*config.Config, error) {
	retinaConfigFile := filepath.Join(option.Config.ConfigDir, configFileName)
	conf, err := config.GetConfig(retinaConfigFile)
	if err != nil {
		logger.WithError(err).Error("Failed to get config file")
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
		logger.Fatalf("Failed to setup zap logger: %v", err)
	}

	namedLogger := log.Logger().Named("retina-with-hubble")
	namedLogger.Info("Traces telemetry initialized with zapai", zap.String("version", buildinfo.Version), zap.String("appInsightsID", buildinfo.ApplicationInsightsID))

	return namedLogger
}

func setupLoggingHooks(logger *logrus.Logger, zapLogger *log.ZapLogger) {
	logger.Hooks.Add(metrics.NewLoggingHook())

	zapHook, err := zaphook.NewZapHook(zapLogger.Logger)
	if err != nil {
		logger.WithError(err).Error("Failed to create zap hook")
	} else {
		logger.Hooks.Add(zapHook)
	}
}

func bootstrapLogging(retinaConfig *config.Config, logger *logrus.Logger) {
	if err := logging.SetupLogging(option.Config.LogDriver, logging.LogOptions(option.Config.LogOpt), "retina-agent", option.Config.Debug); err != nil {
		logger.Fatal(err)
	}

	logLevel, err := logrus.ParseLevel(retinaConfig.LogLevel)
	if err != nil {
		logLevel = logrus.InfoLevel
	}
	logger.SetLevel(logLevel)
}

func initDaemonConfig(vp *viper.Viper) {
	option.Config.Populate(vp)

	time.MaxInternalTimerDelay = vp.GetDuration(option.MaxInternalTimerDelay)
}

func Execute(cobraCmd *cobra.Command, h *hive.Hive) {
	fn := option.InitConfig(cobraCmd, "retina-agent", "retina", h.Viper())
	fn()
	initDaemonConfig(h.Viper())
	initLogging()

	hiveLogger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Allow the current process to lock memory for eBPF resources.
	if err := rlimit.RemoveMemlock(); err != nil {
		logger.Fatal("failed to remove memlock", zap.Error(err))
	}

	//nolint:gocritic // without granular commits this commented-out code may be lost
	// initEnv(h.Viper())

	if err := h.Run(hiveLogger); err != nil {
		logger.Fatal(err)
	}
}
