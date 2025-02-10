// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package legacy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"time"

	"go.uber.org/zap/zapcore"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	"go.uber.org/zap"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	crzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	deploy "github.com/microsoft/retina/deploy/legacy"
	"github.com/microsoft/retina/internal/buildinfo"
	"github.com/microsoft/retina/operator/cache"
	config "github.com/microsoft/retina/operator/config"
	captureUtils "github.com/microsoft/retina/pkg/capture/utils"
	captureController "github.com/microsoft/retina/pkg/controllers/operator/capture"
	metricsconfiguration "github.com/microsoft/retina/pkg/controllers/operator/metricsconfiguration"
	podcontroller "github.com/microsoft/retina/pkg/controllers/operator/pod"
	retinaendpointcontroller "github.com/microsoft/retina/pkg/controllers/operator/retinaendpoint"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/telemetry"
	"github.com/pkg/errors"
)

var (
	scheme     = k8sruntime.NewScheme()
	mainLogger *log.ZapLogger
	oconfig    *config.OperatorConfig

	MaxPodChannelBuffer                  = 250
	MaxMetricsConfigurationChannelBuffer = 50
	MaxTracesConfigurationChannelBuffer  = 50
	MaxRetinaEndpointChannelBuffer       = 250

	MaxFileSizeMB = 100
	MaxBackups    = 3
	MaxAgeDays    = 30

	HeartbeatFrequency = 5 * time.Minute
)

func init() {
	//+kubebuilder:scaffold:scheme
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(retinav1alpha1.AddToScheme(scheme))
}

type Operator struct {
	metricsAddr          string
	probeAddr            string
	enableLeaderElection bool
	configFile           string
}

func NewOperator(metricsAddr, probeAddr, configFile string, enableLeaderElection bool) *Operator {
	return &Operator{
		metricsAddr:          metricsAddr,
		probeAddr:            probeAddr,
		enableLeaderElection: enableLeaderElection,
		configFile:           configFile,
	}
}

func (o *Operator) Start() error {
	mainLogger = log.Logger().Named("main")

	mainLogger.Sugar().Infof("Starting legacy operator")

	opts := &crzap.Options{
		Development: false,
	}

	var err error
	oconfig, err = config.GetConfig(o.configFile)
	if err != nil {
		return errors.Wrap(err, "failed to load config")
	}

	mainLogger.Sugar().Infof("Operator configuration", zap.Any("configuration", oconfig))

	// Set Capture config
	oconfig.CaptureConfig.CaptureImageVersion = buildinfo.Version
	oconfig.CaptureConfig.CaptureImageVersionSource = captureUtils.VersionSourceOperatorImageVersion

	err = initLogging(oconfig, buildinfo.ApplicationInsightsID)
	if err != nil {
		return errors.Wrap(err, "failed to initialize logging")
	}

	ctrl.SetLogger(crzap.New(crzap.UseFlagOptions(opts), crzap.Encoder(zapcore.NewConsoleEncoder(log.EncoderConfig()))))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: o.metricsAddr,
		},
		HealthProbeBindAddress: o.probeAddr,
		LeaderElection:         o.enableLeaderElection,
		LeaderElectionID:       "16937e39.retina.sh",

		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		return errors.Wrap(err, "unable to start manager")
	}

	ctx := context.Background()
	clientset, err := apiextv1.NewForConfig(mgr.GetConfig())
	if err != nil {
		return errors.Wrap(err, "failed to get apiextension clientset")
	}

	if oconfig.InstallCRDs {
		mainLogger.Sugar().Infof("Installing CRDs")

		var crds map[string]*v1.CustomResourceDefinition
		crds, err = deploy.InstallOrUpdateCRDs(ctx, oconfig.EnableRetinaEndpoint, clientset)
		if err != nil {
			return errors.Wrap(err, "unable to register CRDs")
		}
		for name := range crds {
			mainLogger.Info("CRD registered", zap.String("name", name))
		}
	}

	apiserverURL, err := telemetry.GetK8SApiserverURLFromKubeConfig()
	if err != nil {
		return errors.Wrap(err, "apiserver URL cannot be found")
	}

	var tel telemetry.Telemetry
	if oconfig.EnableTelemetry && buildinfo.ApplicationInsightsID != "" {
		mainLogger.Info("telemetry enabled", zap.String("applicationInsightsID", buildinfo.ApplicationInsightsID))
		properties := map[string]string{
			"version":                   buildinfo.Version,
			telemetry.PropertyApiserver: apiserverURL,
		}

		telemetry.InitAppInsights(buildinfo.ApplicationInsightsID, buildinfo.Version)
		defer telemetry.ShutdownAppInsights()
		defer telemetry.TrackPanic()

		tel, err = telemetry.NewAppInsightsTelemetryClient("retina-operator", properties)
		if err != nil {
			return errors.Wrap(err, "failed to create telemetry client")
		}
	} else {
		mainLogger.Info("telemetry disabled", zap.String("apiserver", apiserverURL))
		tel = telemetry.NewNoopTelemetry()
	}

	kubeClient, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return errors.Wrap(err, "failed to get clientset")
	}

	captureReconciler, err := captureController.NewCaptureReconciler(
		mgr.GetClient(), mgr.GetScheme(), kubeClient, oconfig.CaptureConfig,
	)
	if err != nil {
		return errors.Wrap(err, "unable to create capture reconciler")
	}
	if err = captureReconciler.SetupWithManager(mgr); err != nil {
		return errors.Wrap(err, "unable to setup retina capture controller with manager")
	}

	ctrlCtx := ctrl.SetupSignalHandler()

	//+kubebuilder:scaffold:builder

	// TODO(mainred): retina-operater is responsible for recycling created retinaendpoints if remotecontext is switched off.
	// Tracked by https://github.com/microsoft/retina/issues/522
	if oconfig.RemoteContext {
		// Create RetinaEndpoint out of Pod to extract only the necessary fields of Pods to reduce the pressure of APIServer
		// when RetinaEndpoint is enabled.
		// TODO(mainred): An alternative of RetinaEndpoint, and possible long term solution, is to use CiliumEndpoint
		// created for Cilium unmanged Pods.
		if oconfig.EnableRetinaEndpoint {
			mainLogger.Info("RetinaEndpoint is enabled")

			retinaendpointchannel := make(chan cache.PodCacheObject, MaxRetinaEndpointChannelBuffer)
			ke := retinaendpointcontroller.New(mgr.GetClient(), retinaendpointchannel)
			// start reconcile the cached Pod before manager starts to not miss any events
			go ke.ReconcilePod(ctrlCtx)

			pc := podcontroller.New(mgr.GetClient(), mgr.GetScheme(), retinaendpointchannel)
			if err = (pc).SetupWithManager(mgr); err != nil {
				return errors.Wrap(err, "unable to create controller - podcontroller")
			}
		}
	}

	mc := metricsconfiguration.New(mgr.GetClient(), mgr.GetScheme())
	if err = (mc).SetupWithManager(mgr); err != nil {
		return errors.Wrap(err, "unable to create controller - metricsconfiguration")
	}

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return errors.Wrap(err, "unable to set up health check")
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return errors.Wrap(err, "unable to set up ready check")
	}

	// start heartbeat goroutine for application insights
	go tel.Heartbeat(ctx, HeartbeatFrequency)

	mainLogger.Info("Starting manager")
	if err := mgr.Start(ctrlCtx); err != nil {
		return errors.Wrap(err, "problem running manager")
	}

	return nil
}

func EnablePProf() {
	pprofmux := http.NewServeMux()
	pprofmux.HandleFunc("/debug/pprof/", pprof.Index)
	pprofmux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	pprofmux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	pprofmux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	pprofmux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	pprofmux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))

	if err := http.ListenAndServe(":8082", pprofmux); err != nil { //nolint:gosec // TODO replace with secure server that supports timeout
		panic(err)
	}
}

func initLogging(cfg *config.OperatorConfig, applicationInsightsID string) error {
	logOpts := &log.LogOpts{
		Level:                 cfg.LogLevel,
		File:                  false,
		MaxFileSizeMB:         MaxFileSizeMB,
		MaxBackups:            MaxBackups,
		MaxAgeDays:            MaxAgeDays,
		ApplicationInsightsID: applicationInsightsID,
		EnableTelemetry:       cfg.EnableTelemetry,
	}

	_, err := log.SetupZapLogger(logOpts)
	if err != nil {
		mainLogger.Error("Failed to setup zap logger", zap.Error(err))
		return fmt.Errorf("failed to setup zap logger: %w", err)
	}

	return nil
}
