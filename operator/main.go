/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	"go.uber.org/zap/zapcore"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	"github.com/spf13/viper"
	"go.uber.org/zap"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	crzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	deploy "github.com/microsoft/retina/deploy"
	"github.com/microsoft/retina/operator/cache"
	config "github.com/microsoft/retina/operator/config"
	captureUtils "github.com/microsoft/retina/pkg/capture/utils"
	captureController "github.com/microsoft/retina/pkg/controllers/operator/capture"
	metricsconfiguration "github.com/microsoft/retina/pkg/controllers/operator/metricsconfiguration"
	podcontroller "github.com/microsoft/retina/pkg/controllers/operator/pod"
	retinaendpointcontroller "github.com/microsoft/retina/pkg/controllers/operator/retinaendpoint"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/telemetry"
)

var (
	scheme     = k8sruntime.NewScheme()
	mainLogger *log.ZapLogger
	oconfig    *config.OperatorConfig

	MAX_POD_CHANNEL_BUFFER                   = 250
	MAX_METRICS_CONFIGURATION_CHANNEL_BUFFER = 50
	MAX_TRACES_CONFIGURATION_CHANNEL_BUFFER  = 50
	MAX_RETINA_ENDPOINT_CHANNEL_BUFFER       = 250

	version = "undefined"

	applicationInsightsID string //nolint // aiMetadata is set in Makefile
)

func init() {
	//+kubebuilder:scaffold:scheme
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(retinav1alpha1.AddToScheme(scheme))

	var err error
	oconfig, err = LoadConfig()
	if err != nil {
		fmt.Printf("failed to load config with err %s", err.Error())
		os.Exit(1)
	}

	err = initLogging(oconfig, applicationInsightsID)
	if err != nil {
		fmt.Printf("failed to initialize logging with err %s", err.Error())
		os.Exit(1)
	}
	mainLogger = log.Logger().Named("main")
}

func main() {
	go EnablePProf()
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	mainLogger.Sugar().Infof("Operator configuration", zap.Any("configuration", oconfig))

	opts := &crzap.Options{
		Development: false,
	}

	ctrl.SetLogger(crzap.New(crzap.UseFlagOptions(opts), crzap.Encoder(zapcore.NewConsoleEncoder(log.EncoderConfig()))))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
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
		mainLogger.Error("Unable to start manager", zap.Error(err))
		os.Exit(1)
	}

	ctx := context.Background()
	clientset, err := apiextv1.NewForConfig(mgr.GetConfig())
	if err != nil {
		mainLogger.Error("Failed to get apiextension clientset", zap.Error(err))
		os.Exit(1)
	}

	if oconfig.InstallCRDs {
		mainLogger.Sugar().Infof("Installing CRDs")
		crds, err := deploy.InstallOrUpdateCRDs(ctx, oconfig.EnableRetinaEndpoint, clientset)
		if err != nil {
			mainLogger.Error("unable to register CRDs", zap.Error(err))
			os.Exit(1)
		}
		for name := range crds {
			mainLogger.Info("CRD registered", zap.String("name", name))
		}
	}

	apiserverURL, err := telemetry.GetK8SApiserverURLFromKubeConfig()
	if err != nil {
		mainLogger.Error("Apiserver URL is cannot be found", zap.Error(err))
		os.Exit(1)
	}

	var tel telemetry.Telemetry
	if oconfig.EnableTelemetry {
		mainLogger.Info("telemetry enabled", zap.String("applicationInsightsID", applicationInsightsID))
		properties := map[string]string{
			"version":                   version,
			telemetry.PropertyApiserver: apiserverURL,
		}
		tel = telemetry.NewAppInsightsTelemetryClient("retina-agent", properties)
	} else {
		mainLogger.Info("telemetry disabled for:", zap.String("apiserver", apiserverURL))
		tel = telemetry.NewNoopTelemetry()
	}

	kubeClient, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		mainLogger.Error("Failed to get clientset", zap.Error(err))
		os.Exit(1)
	}

	if err = captureController.NewCaptureReconciler(
		mgr.GetClient(), mgr.GetScheme(), kubeClient, oconfig.CaptureConfig,
	).SetupWithManager(mgr); err != nil {
		mainLogger.Error("Unable to setup retina capture controller with manager", zap.Error(err))
		os.Exit(1)
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

			retinaendpointchannel := make(chan cache.PodCacheObject, MAX_RETINA_ENDPOINT_CHANNEL_BUFFER)
			ke := retinaendpointcontroller.New(mgr.GetClient(), retinaendpointchannel)
			// start reconcile the cached Pod before manager starts to not miss any events
			go ke.ReconcilePod(ctrlCtx)

			pc := podcontroller.New(mgr.GetClient(), mgr.GetScheme(), retinaendpointchannel)
			if err = (pc).SetupWithManager(mgr); err != nil {
				mainLogger.Error("Unable to create controller", zap.String("controller", "podcontroller"), zap.Error(err))
				os.Exit(1)
			}
		}
	}

	mc := metricsconfiguration.New(mgr.GetClient(), mgr.GetScheme())
	if err = (mc).SetupWithManager(mgr); err != nil {
		mainLogger.Error("Unable to create controller", zap.String("controller", "metricsconfiguration"), zap.Error(err))
		os.Exit(1)
	}

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		mainLogger.Error("Unable to set up health check", zap.Error(err))
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		mainLogger.Error("Unable to set up ready check", zap.Error(err))
		os.Exit(1)
	}

	mainLogger.Info("Starting manager")
	if err := mgr.Start(ctrlCtx); err != nil {
		mainLogger.Error("Problem running manager", zap.Error(err))
		os.Exit(1)
	}

	// start heartbeat goroutine for application insights
	go tel.Heartbeat(ctx, 5*time.Minute)
}

func LoadConfig() (*config.OperatorConfig, error) {
	viper.SetConfigType("yaml")
	viper.SetConfigFile("retina/operator-config.yaml")
	err := viper.ReadInConfig()
	if err != nil {
		return nil, err
	}

	viper.AutomaticEnv()

	var config config.OperatorConfig

	// Check pkg/config/config.go for the explanation of setting EnableRetinaEndpoint defaults to true.
	viper.SetDefault("EnableRetinaEndpoint", true)
	err = viper.Unmarshal(&config)

	// Set Capture config
	config.CaptureConfig.CaptureImageVersion = version
	config.CaptureConfig.CaptureImageVersionSource = captureUtils.VersionSourceOperatorImageVersion

	return &config, err
}

func EnablePProf() {
	pprofmux := http.NewServeMux()
	pprofmux.HandleFunc("/debug/pprof/", pprof.Index)
	pprofmux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	pprofmux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	pprofmux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	pprofmux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	pprofmux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))

	if err := http.ListenAndServe(":8082", pprofmux); err != nil {
		panic(err)
	}
}

func initLogging(config *config.OperatorConfig, applicationInsightsID string) error {
	logOpts := &log.LogOpts{
		Level:                 config.LogLevel,
		File:                  false,
		MaxFileSizeMB:         100,
		MaxBackups:            3,
		MaxAgeDays:            30,
		ApplicationInsightsID: applicationInsightsID,
		EnableTelemetry:       config.EnableTelemetry,
	}

	log.SetupZapLogger(logOpts)

	return nil
}
