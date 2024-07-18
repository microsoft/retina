// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
// Copyright Authors of Cilium.
// Modified by Authors of Retina.
package dns_notify_forwarding

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	zaphook "github.com/Sytten/logrus-zap-hook"
	"github.com/cilium/cilium/pkg/hive"
	"github.com/cilium/cilium/pkg/hive/cell"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/cilium/pkg/ipcache"
	"github.com/cilium/cilium/pkg/k8s"
	k8sClient "github.com/cilium/cilium/pkg/k8s/client"
	"github.com/cilium/cilium/pkg/k8s/watchers"
	"github.com/cilium/cilium/pkg/metrics"
	"github.com/cilium/cilium/pkg/node"
	"github.com/cilium/cilium/pkg/option"
	"github.com/cilium/cilium/pkg/promise"
	"github.com/cilium/cilium/pkg/time"
	"github.com/cilium/proxy/pkg/logging"
	"github.com/cilium/workerpool"
	"github.com/microsoft/retina/pkg/config"
	retinak8s "github.com/microsoft/retina/pkg/k8s"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/managers/pluginmanager"
	sharedconfig "github.com/microsoft/retina/pkg/shared/config"
	"github.com/microsoft/retina/pkg/telemetry"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	zapf "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// Connect to Cilium agent
// ...

// Create a channel for DNS plugin to send events to
// ...

// On events received, create mappings and send to Cilium agent
// ...

// Port 40046 for DNS proxy
// Port 40045 default port for Cilium agent
// ...

const (
	configFileName  string = "config.yaml"
	logFileName     string = "retina.log"
	grpcForwardPort        = 40046
)

var (
	// Below two fields are set while building the binary
	// they are passed in as ldflags
	// see dockerfile
	applicationInsightsID string
	retinaVersion         string

	scheme     = k8sruntime.NewScheme()
	daemonCell = cell.Module(
		"daemon",
		"Retina-Agent Daemon",
		// Create the controller manager, provides the hive with the controller manager and its client
		cell.Provide(func(k8sCfg *rest.Config, logger logrus.FieldLogger, rcfg config.RetinaHubbleConfig) (ctrl.Manager, client.Client, error) {
			if err := corev1.AddToScheme(scheme); err != nil { //nolint:govet // intentional shadow
				logger.Error("failed to add corev1 to scheme")
				return nil, nil, errors.Wrap(err, "failed to add corev1 to scheme")
			}

			mgrOption := ctrl.Options{
				Scheme:                 scheme,
				HealthProbeBindAddress: rcfg.HealthProbeBindAddress,
				LeaderElection:         rcfg.LeaderElection,
				LeaderElectionID:       "ecaf1259.retina.io",
			}

			logf.SetLogger(zapf.New())
			ctrlManager, err := ctrl.NewManager(k8sCfg, mgrOption)
			if err != nil {
				logger.Error("failed to create manager")
				return nil, nil, fmt.Errorf("creating new controller-runtime manager: %w", err)
			}

			return ctrlManager, ctrlManager.GetClient(), nil
		}),

		// Start the controller manager
		cell.Invoke(func(l logrus.FieldLogger, lifecycle cell.Lifecycle, ctrlManager ctrl.Manager) {
			var wp *workerpool.WorkerPool
			lifecycle.Append(
				cell.Hook{
					OnStart: func(cell.HookContext) error {
						wp = workerpool.New(1)
						l.Info("starting controller-runtime manager")
						if err := wp.Submit("controller-runtime manager", ctrlManager.Start); err != nil {
							return errors.Wrap(err, "failed to submit controller-runtime manager to workerpool")
						}
						return nil
					},
					OnStop: func(cell.HookContext) error {
						if err := wp.Close(); err != nil {
							return errors.Wrap(err, "failed to close controller-runtime workerpool")
						}
						return nil
					},
				},
			)
		}),
		cell.Invoke(newDaemonPromise),
	)
)

func InitGlobalFlags(cmd *cobra.Command, vp *viper.Viper) {
	flags := cmd.Flags()

	if err := vp.BindPFlags(flags); err != nil {
		logger.Fatalf("BindPFlags failed: %s", err)
	}
}

type daemonParams struct {
	cell.In

	Lifecycle     cell.Lifecycle
	Clientset     k8sClient.Clientset
	PluginManager *pluginmanager.PluginManager
	Log           logrus.FieldLogger
	Client        client.Client
	K8sWatcher    *watchers.K8sWatcher
	Lnds          *node.LocalNodeStore
	IPC           *ipcache.IPCache
	SvcCache      *k8s.ServiceCache
	Telemetry     telemetry.Telemetry
	Config        config.Config
	EventChannel  chan *v1.Event
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
		ApplicationInsightsID: applicationInsightsID,
		EnableTelemetry:       retinaConfig.EnableTelemetry,
	}

	persistentFields := []zap.Field{
		zap.String("version", retinaVersion),
		zap.String("apiserver", k8sCfg.Host),
		// In this case, only dns should be enabled
		zap.Strings("plugins", retinaConfig.EnabledPlugin),
	}

	_, err := log.SetupZapLogger(logOpts, persistentFields...)
	if err != nil {
		logger.Fatalf("Failed to setup zap logger: %v", err)
	}

	namedLogger := log.Logger().Named("retina-dns-notify-forwarding")
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
	time.MaxInternalTimerDelay = vp.GetDuration(option.MaxInternalTimerDelay)
}

func Execute(cobraCmd *cobra.Command, h *hive.Hive) {
	fn := option.InitConfig(cobraCmd, "retina-agent", "retina", h.Viper())
	fn()
	initDaemonConfig(h.Viper())
	initLogging()

	if err := h.Run(); err != nil {
		logger.Fatal(err)
	}
}

type Daemon struct {
	clientset k8sClient.Clientset

	log            logrus.FieldLogger
	pluginManager  *pluginmanager.PluginManager
	client         client.Client
	k8swatcher     *watchers.K8sWatcher
	localNodeStore *node.LocalNodeStore
	ipc            *ipcache.IPCache
	svcCache       *k8s.ServiceCache
	eventChannel   chan *v1.Event
}

func newDaemon(params *daemonParams) *Daemon {
	return &Daemon{
		pluginManager:  params.PluginManager,
		clientset:      params.Clientset,
		log:            params.Log,
		client:         params.Client,
		k8swatcher:     params.K8sWatcher,
		localNodeStore: params.Lnds,
		ipc:            params.IPC,
		svcCache:       params.SvcCache,
		eventChannel:   params.EventChannel,
	}
}

func (d *Daemon) Run(ctx context.Context) error {
	// Start K8s watcher
	d.log.WithField("localNodeStore", d.localNodeStore).Info("Starting local node store")

	// Start K8s watcher. Will block till sync is complete or timeout.
	// If sync doesn't complete within timeout (3 minutes), causes fatal error.
	retinak8s.Start(ctx, d.k8swatcher)

	return nil
}
