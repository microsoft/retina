// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package hubble

import (
	"fmt"
	"io"
	"math"
	"path/filepath"
	"strings"

	zaphook "github.com/Sytten/logrus-zap-hook"
	"github.com/cilium/cilium/pkg/defaults"
	"github.com/cilium/cilium/pkg/hubble/exporter/exporteroption"
	"github.com/cilium/cilium/pkg/hubble/observer/observeroption"
	"github.com/cilium/cilium/pkg/metrics"
	monitorAPI "github.com/cilium/cilium/pkg/monitor/api"
	"github.com/cilium/cilium/pkg/option"
	"github.com/cilium/cilium/pkg/time"
	"github.com/cilium/proxy/pkg/logging"
	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	sharedconfig "github.com/microsoft/retina/pkg/shared/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
)

const (
	configFileName string = "config.yaml"
	logFileName           = "retina.log"
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

	flags.String(option.ConfigDir, "/retina/config", `Configuration directory that contains a file for each option`)
	option.BindEnv(vp, option.ConfigDir)

	// // Env bindings
	// flags.Int(option.AgentHealthPort, defaults.AgentHealthPort, "TCP port for agent health status API")
	// option.BindEnv(vp, option.AgentHealthPort)

	// flags.Int(option.ClusterHealthPort, defaults.ClusterHealthPort, "TCP port for cluster-wide network connectivity health API")
	// option.BindEnv(vp, option.ClusterHealthPort)

	// flags.StringSlice(option.AgentLabels, []string{}, "Additional labels to identify this agent")
	// option.BindEnv(vp, option.AgentLabels)

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

	flags.StringSlice(option.HubbleTLSClientCAFiles, []string{}, "Paths to one or more public key files of client CA certificates to use for TLS with mutual authentication (mTLS). The files must contain PEM encoded data. When provided, this option effectively enables mTLS.")
	option.BindEnv(vp, option.HubbleTLSClientCAFiles)

	flags.Int(option.HubbleEventBufferCapacity, observeroption.Default.MaxFlows.AsInt(), "Capacity of Hubble events buffer. The provided value must be one less than an integer power of two and no larger than 65535 (ie: 1, 3, ..., 2047, 4095, ..., 65535)")
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

	// flags.String(option.AllowLocalhost, option.AllowLocalhostAuto, "Policy when to allow local stack to reach local endpoints { auto | always | policy }")
	// option.BindEnv(vp, option.AllowLocalhost)

	// flags.Bool(option.EnableIPv4Name, defaults.EnableIPv4, "Enable IPv4 support")
	// option.BindEnv(vp, option.EnableIPv4Name)

	// flags.Bool(option.EnableIPv6Name, false, "Enable IPv6 support")
	// option.BindEnv(vp, option.EnableIPv6Name)

	// flags.Duration(option.KVstoreLeaseTTL, defaults.KVstoreLeaseTTL, "Time-to-live for the KVstore lease.")
	// err := flags.MarkHidden(option.KVstoreLeaseTTL)
	// if err != nil {
	// 	logger.Fatalf("MarkHidden failed for %s: %s", option.KVstoreLeaseTTL, err)
	// }
	// option.BindEnv(vp, option.KVstoreLeaseTTL)

	if err := vp.BindPFlags(flags); err != nil {
		logger.Fatalf("BindPFlags failed: %s", err)
	}
}

func initLogging() {
	logger := setupDefaultLogger()
	config, _ := getRetinaConfig(logger)
	k8sCfg, _ := sharedconfig.GetK8sConfig()
	zapLogger := setupZapLogger(config, k8sCfg)
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

func setupZapLogger(config *config.Config, k8sCfg *rest.Config) *log.ZapLogger {
	logOpts := &log.LogOpts{
		Level:                 config.LogLevel,
		File:                  false,
		FileName:              logFileName,
		MaxFileSizeMB:         100,
		MaxBackups:            3,
		MaxAgeDays:            30,
		ApplicationInsightsID: applicationInsightsID,
		EnableTelemetry:       config.EnableTelemetry,
	}

	persistentFields := []zap.Field{
		zap.String("version", retinaVersion),
		zap.String("apiserver", k8sCfg.Host),
		zap.Strings("plugins", config.EnabledPlugin),
	}

	log.SetupZapLogger(logOpts, persistentFields...)

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
		option.Config.HubbleEventBufferCapacity = int(math.Pow(2, 14) - 1)
	}

	time.MaxInternalTimerDelay = vp.GetDuration(option.MaxInternalTimerDelay)
}
