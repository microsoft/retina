// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// This bootstraps Hubble control plane.
// Inspired by Cilium's daemon_main.go.

package hubble

import (
	"context"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"strings"

	zaphook "github.com/Sytten/logrus-zap-hook"
	"github.com/cilium/cilium/pkg/defaults"
	"github.com/cilium/cilium/pkg/hive"
	"github.com/cilium/cilium/pkg/hive/cell"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/cilium/pkg/hubble/exporter/exporteroption"
	"github.com/cilium/cilium/pkg/hubble/observer/observeroption"
	"github.com/cilium/cilium/pkg/ipcache"
	"github.com/cilium/cilium/pkg/k8s"
	k8sClient "github.com/cilium/cilium/pkg/k8s/client"
	"github.com/cilium/cilium/pkg/k8s/watchers"
	"github.com/cilium/cilium/pkg/metrics"
	monitorAgent "github.com/cilium/cilium/pkg/monitor/agent"
	monitorAPI "github.com/cilium/cilium/pkg/monitor/api"
	"github.com/cilium/cilium/pkg/node"
	"github.com/cilium/cilium/pkg/option"
	"github.com/cilium/cilium/pkg/promise"
	"github.com/cilium/cilium/pkg/time"
	"github.com/cilium/proxy/pkg/logging"
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

var (
	// Below two fields are set while building the binary
	// they are passed in as ldflags
	// see dockerfile
	applicationInsightsID string
	retinaVersion         string
)

func InitGlobalFlags(cmd *cobra.Command, vp *viper.Viper) {
	flags := cmd.Flags()

	flags.String(option.IdentityAllocationMode, option.IdentityAllocationModeCRD, "Identity allocation mode")

	// Add all the flags Hubble supports currently.
	flags.String(option.ConfigDir, "/retina/config", `Configuration directory that contains a file for each option`)
	option.BindEnv(vp, option.ConfigDir)

	flags.Bool(option.EnableHubble, false, "Enable hubble server")
	option.BindEnv(vp, option.EnableHubble)

	flags.String(option.HubbleSocketPath, defaults.HubbleSockPath, "Set hubble's socket path to listen for connections")
	option.BindEnv(vp, option.HubbleSocketPath)

	flags.String(option.HubbleListenAddress, "", `An additional address for Hubble server to listen to, e.g. ":4244"`)
	option.BindEnv(vp, option.HubbleListenAddress)

	flags.Bool(option.HubblePreferIpv6, false, "Prefer IPv6 addresses for announcing nodes when both address types are available.")
	option.BindEnv(vp, option.HubblePreferIpv6)

	flags.Bool(option.HubbleTLSDisabled, false, "Allow Hubble server to run on the given listen address without TLS.")
	option.BindEnv(vp, option.HubbleTLSDisabled)

	flags.String(option.HubbleTLSCertFile, "", "Path to the public key file for the Hubble server. The file must contain PEM encoded data.")
	option.BindEnv(vp, option.HubbleTLSCertFile)

	flags.String(option.HubbleTLSKeyFile, "", "Path to the private key file for the Hubble server. The file must contain PEM encoded data.")
	option.BindEnv(vp, option.HubbleTLSKeyFile)

	flags.StringSlice(option.HubbleTLSClientCAFiles, []string{}, "Paths to one or more public key files of client CA certificates to use for TLS with mutual authentication (mTLS). The files must contain PEM encoded data. When provided, this option effectively enables mTLS.") //nolint:lll // long line (over 80 characters).
	option.BindEnv(vp, option.HubbleTLSClientCAFiles)

	flags.Int(option.HubbleEventBufferCapacity, observeroption.Default.MaxFlows.AsInt(), "Capacity of Hubble events buffer. The provided value must be one less than an integer power of two and no larger than 65535 (ie: 1, 3, ..., 2047, 4095, ..., 65535)") //nolint:lll // long line.
	option.BindEnv(vp, option.HubbleEventBufferCapacity)

	flags.Int(option.HubbleEventQueueSize, 0, "Buffer size of the channel to receive monitor events.")
	option.BindEnv(vp, option.HubbleEventQueueSize)

	flags.String(option.HubbleMetricsServer, "", "Address to serve Hubble metrics on.")
	option.BindEnv(vp, option.HubbleMetricsServer)

	flags.StringSlice(option.HubbleMetrics, []string{}, "List of Hubble metrics to enable.")
	option.BindEnv(vp, option.HubbleMetrics)

	flags.String(option.HubbleFlowlogsConfigFilePath, "", "Filepath with configuration of hubble flowlogs")
	option.BindEnv(vp, option.HubbleFlowlogsConfigFilePath)

	flags.String(option.HubbleExportFilePath, exporteroption.Default.Path, "Filepath to write Hubble events to.")
	option.BindEnv(vp, option.HubbleExportFilePath)

	flags.Int(option.HubbleExportFileMaxSizeMB, exporteroption.Default.MaxSizeMB, "Size in MB at which to rotate Hubble export file.")
	option.BindEnv(vp, option.HubbleExportFileMaxSizeMB)

	flags.Int(option.HubbleExportFileMaxBackups, exporteroption.Default.MaxBackups, "Number of rotated Hubble export files to keep.")
	option.BindEnv(vp, option.HubbleExportFileMaxBackups)

	flags.Bool(option.HubbleExportFileCompress, exporteroption.Default.Compress, "Compress rotated Hubble export files.")
	option.BindEnv(vp, option.HubbleExportFileCompress)

	flags.StringSlice(option.HubbleExportAllowlist, []string{}, "Specify allowlist as JSON encoded FlowFilters to Hubble exporter.")
	option.BindEnv(vp, option.HubbleExportAllowlist)

	flags.StringSlice(option.HubbleExportDenylist, []string{}, "Specify denylist as JSON encoded FlowFilters to Hubble exporter.")
	option.BindEnv(vp, option.HubbleExportDenylist)

	flags.StringSlice(option.HubbleExportFieldmask, []string{}, "Specify list of fields to use for field mask in Hubble exporter.")
	option.BindEnv(vp, option.HubbleExportFieldmask)

	flags.Bool(option.EnableHubbleRecorderAPI, true, "Enable the Hubble recorder API")
	option.BindEnv(vp, option.EnableHubbleRecorderAPI)

	flags.String(option.HubbleRecorderStoragePath, defaults.HubbleRecorderStoragePath, "Directory in which pcap files created via the Hubble Recorder API are stored")
	option.BindEnv(vp, option.HubbleRecorderStoragePath)

	flags.Int(option.HubbleRecorderSinkQueueSize, defaults.HubbleRecorderSinkQueueSize, "Queue size of each Hubble recorder sink")
	option.BindEnv(vp, option.HubbleRecorderSinkQueueSize)

	flags.Bool(option.HubbleSkipUnknownCGroupIDs, true, "Skip Hubble events with unknown cgroup ids")
	option.BindEnv(vp, option.HubbleSkipUnknownCGroupIDs)

	flags.StringSlice(option.HubbleMonitorEvents, []string{},
		fmt.Sprintf(
			"Cilium monitor events for Hubble to observe: [%s]. By default, Hubble observes all monitor events.",
			strings.Join(monitorAPI.AllMessageTypeNames(), " "),
		),
	)
	option.BindEnv(vp, option.HubbleMonitorEvents)

	flags.Bool(option.HubbleRedactEnabled, defaults.HubbleRedactEnabled, "Hubble redact sensitive information from flows")
	option.BindEnv(vp, option.HubbleRedactEnabled)

	flags.Bool(option.HubbleRedactHttpURLQuery, defaults.HubbleRedactHttpURLQuery, "Hubble redact http URL query from flows")
	option.BindEnv(vp, option.HubbleRedactHttpURLQuery)

	flags.Bool(option.HubbleRedactHttpUserInfo, defaults.HubbleRedactHttpUserInfo, "Hubble redact http user info from flows")
	option.BindEnv(vp, option.HubbleRedactHttpUserInfo)

	flags.Bool(option.HubbleRedactKafkaApiKey, defaults.HubbleRedactKafkaApiKey, "Hubble redact Kafka API key from flows")
	option.BindEnv(vp, option.HubbleRedactKafkaApiKey)

	flags.StringSlice(option.HubbleRedactHttpHeadersAllow, []string{}, "HTTP headers to keep visible in flows")
	option.BindEnv(vp, option.HubbleRedactHttpHeadersAllow)

	flags.StringSlice(option.HubbleRedactHttpHeadersDeny, []string{}, "HTTP headers to redact from flows")
	option.BindEnv(vp, option.HubbleRedactHttpHeadersDeny)

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
	SvcCache      *k8s.ServiceCache
	Telemetry     telemetry.Telemetry
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

			d.log.Info("starting Retina Enterprise version: ", retinaVersion)
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
	bootstrapLogging(logger)
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
		return nil, err
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
		ApplicationInsightsID: applicationInsightsID,
		EnableTelemetry:       retinaConfig.EnableTelemetry,
	}

	persistentFields := []zap.Field{
		zap.String("version", retinaVersion),
		zap.String("apiserver", k8sCfg.Host),
		zap.Strings("plugins", retinaConfig.EnabledPlugin),
	}

	_, err := log.SetupZapLogger(logOpts, persistentFields...)
	if err != nil {
		logger.Fatalf("Failed to setup zap logger: %v", err)
	}

	namedLogger := log.Logger().Named("retina-enterprise")
	namedLogger.Info("Traces telemetry initialized with zapai", zap.String("version", retinaVersion), zap.String("appInsightsID", applicationInsightsID))

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

func bootstrapLogging(logger *logrus.Logger) {
	if err := logging.SetupLogging(option.Config.LogDriver, logging.LogOptions(option.Config.LogOpt), "retina-agent", option.Config.Debug); err != nil {
		logger.Fatal(err)
	}
}

func initDaemonConfig(vp *viper.Viper) {
	option.Config.Populate(vp)
	if option.Config.HubbleEventBufferCapacity == 0 {
		option.Config.HubbleEventBufferCapacity = int(math.Pow(2, 14) - 1) //nolint:gomnd // this is just math
	}

	time.MaxInternalTimerDelay = vp.GetDuration(option.MaxInternalTimerDelay)
}

func Execute(cobraCmd *cobra.Command, h *hive.Hive) {
	fn := option.InitConfig(cobraCmd, "retina-agent", "retina", h.Viper())
	fn()
	initDaemonConfig(h.Viper())
	initLogging()

	//nolint:gocritic // without granular commits this commented-out code may be lost
	// initEnv(h.Viper())

	if err := h.Run(); err != nil {
		logger.Fatal(err)
	}
}
