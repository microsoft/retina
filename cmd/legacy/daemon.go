// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package legacy

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-logr/zapr"
	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/config"
	controllercache "github.com/microsoft/retina/pkg/controllers/cache"
	mcc "github.com/microsoft/retina/pkg/controllers/daemon/metricsconfiguration"
	namespacecontroller "github.com/microsoft/retina/pkg/controllers/daemon/namespace"
	nc "github.com/microsoft/retina/pkg/controllers/daemon/node"
	pc "github.com/microsoft/retina/pkg/controllers/daemon/pod"
	kec "github.com/microsoft/retina/pkg/controllers/daemon/retinaendpoint"
	sc "github.com/microsoft/retina/pkg/controllers/daemon/service"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	crcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kcfg "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	crmgr "sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	cm "github.com/microsoft/retina/pkg/managers/controllermanager"
	"github.com/microsoft/retina/pkg/managers/filtermanager"
	"github.com/microsoft/retina/pkg/metrics"
	mm "github.com/microsoft/retina/pkg/module/metrics"
	"github.com/microsoft/retina/pkg/pubsub"
	"github.com/microsoft/retina/pkg/telemetry"
)

const (
	logFileName       = "retina.log"
	heartbeatInterval = 5 * time.Minute

	nodeNameEnvKey = "NODE_NAME"
	nodeIPEnvKey   = "NODE_IP"
)

var (
	scheme = k8sruntime.NewScheme()

	// applicationInsightsID is the instrumentation key for Azure Application Insights
	// It is set during the build process using the -ldflags flag
	// If it is set, the application will send telemetry to the corresponding Application Insights resource.
	applicationInsightsID string
	version               string
)

func init() {
	//+kubebuilder:scaffold:scheme
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(retinav1alpha1.AddToScheme(scheme))
}

type Daemon struct {
	metricsAddr          string
	probeAddr            string
	enableLeaderElection bool
	configFile           string
}

func NewDaemon(metricsAddr, probeAddr, configFile string, enableLeaderElection bool) *Daemon {
	return &Daemon{
		metricsAddr:          metricsAddr,
		probeAddr:            probeAddr,
		enableLeaderElection: enableLeaderElection,
		configFile:           configFile,
	}
}

func (d *Daemon) Start() error {
	fmt.Printf("starting Retina daemon with legacy control plane %v\n", version)

	if applicationInsightsID != "" {
		telemetry.InitAppInsights(applicationInsightsID, version)
		defer telemetry.ShutdownAppInsights()
		defer telemetry.TrackPanic()
	}

	daemonConfig, err := config.GetConfig(d.configFile)
	if err != nil {
		panic(err)
	}

	fmt.Println("init client-go")
	cfg, err := kcfg.GetConfig()
	if err != nil {
		panic(err)
	}

	fmt.Println("init logger")
	zl, err := log.SetupZapLogger(&log.LogOpts{
		Level:                 daemonConfig.LogLevel,
		File:                  false,
		FileName:              logFileName,
		MaxFileSizeMB:         100, //nolint:gomnd // defaults
		MaxBackups:            3,   //nolint:gomnd // defaults
		MaxAgeDays:            30,  //nolint:gomnd // defaults
		ApplicationInsightsID: applicationInsightsID,
		EnableTelemetry:       daemonConfig.EnableTelemetry,
	},
		zap.String("version", version),
		zap.String("apiserver", cfg.Host),
		zap.String("plugins", strings.Join(daemonConfig.EnabledPlugin, `,`)),
	)
	if err != nil {
		panic(err)
	}
	defer zl.Close()
	mainLogger := zl.Named("main").Sugar()

	metrics.InitializeMetrics()

	var tel telemetry.Telemetry
	if daemonConfig.EnableTelemetry && applicationInsightsID != "" {
		mainLogger.Info("telemetry enabled", zap.String("applicationInsightsID", applicationInsightsID))
		tel = telemetry.NewAppInsightsTelemetryClient("retina-agent", map[string]string{
			"version":   version,
			"apiserver": cfg.Host,
			"plugins":   strings.Join(daemonConfig.EnabledPlugin, `,`),
		})
	} else {
		mainLogger.Info("telemetry disabled")
		tel = telemetry.NewNoopTelemetry()
	}

	// Create a manager for controller-runtime

	mgrOption := crmgr.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: d.metricsAddr,
		},
		HealthProbeBindAddress: d.probeAddr,
		LeaderElection:         d.enableLeaderElection,
		LeaderElectionID:       "ecaf1259.retina.sh",
	}

	// Local context has its meaning only when pod level(advanced) metrics is enabled.
	if daemonConfig.EnablePodLevel && !daemonConfig.RemoteContext {
		mainLogger.Info("Remote context is disabled, only pods deployed on the same node as retina-agent will be monitored")
		// the new cache sets Selector options on the Manager cache which are used
		// to perform *server-side* filtering of the cached objects. This is very important
		// for high node/pod count clusters, as it keeps us from watching objects at the
		// whole cluster scope when we are only interested in the Node's scope.
		nodeName := os.Getenv(nodeNameEnvKey)
		if nodeName == "" {
			mainLogger.Fatal("failed to get node name from environment variable", zap.String("node name env key", nodeNameEnvKey))
		}
		podNodeNameSelector := fields.SelectorFromSet(fields.Set{"spec.nodeName": nodeName})
		// Ignore hostnetwork pods which share the same IP with the node and pods on the same node.
		// Unlike spec.nodeName, field label "spec.hostNetwork" is not supported, and as a workaround,
		// We use status.podIP to filter out hostnetwork pods.
		// https://github.com/kubernetes/kubernetes/blob/41da26dbe15207cbe5b6c36b48a31d2cd3344123/pkg/apis/core/v1/conversion.go#L36
		nodeIP := os.Getenv(nodeIPEnvKey)
		if nodeIP == "" {
			mainLogger.Fatal("failed to get node IP from environment variable", zap.String("node IP env key", nodeIPEnvKey))
		}
		podNodeIPNotMatchSelector := fields.OneTermNotEqualSelector("status.podIP", nodeIP)
		podSelector := fields.AndSelectors(podNodeNameSelector, podNodeIPNotMatchSelector)

		mainLogger.Info("pod selector when remote context is disabled", zap.String("pod selector", podSelector.String()))
		mgrOption.Cache = crcache.Options{
			ByObject: map[client.Object]crcache.ByObject{
				&corev1.Pod{}: {
					Field: podSelector,
				},
			},
		}
	}

	mgr, err := crmgr.New(cfg, mgrOption)
	if err != nil {
		mainLogger.Error("Unable to start manager", zap.Error(err))
		return fmt.Errorf("creating controller-runtime manager: %w", err)
	}

	//+kubebuilder:scaffold:builder

	if healthCheckErr := mgr.AddHealthzCheck("healthz", healthz.Ping); healthCheckErr != nil {
		mainLogger.Fatal("Unable to set up health check", zap.Error(healthCheckErr))
	}
	if addReadyCheckErr := mgr.AddReadyzCheck("readyz", healthz.Ping); addReadyCheckErr != nil {
		mainLogger.Fatal("Unable to set up ready check", zap.Error(addReadyCheckErr))
	}

	// k8s Client used for informers
	cl := kubernetes.NewForConfigOrDie(mgr.GetConfig())

	serverVersion, err := cl.Discovery().ServerVersion()
	if err != nil {
		mainLogger.Error("failed to get Kubernetes server version: ", zap.Error(err))
	} else {
		mainLogger.Infof("Kubernetes server version: %v", serverVersion)
	}

	// Setup RetinaEndpoint controller.
	// TODO(mainred): This is to temporarily create a cache and pubsub for RetinaEndpoint, need to refactor this.
	ctx := ctrl.SetupSignalHandler()
	ctrl.SetLogger(zapr.NewLogger(zl.Logger.Named("controller-runtime")))

	if daemonConfig.EnablePodLevel {
		pubSub := pubsub.New()
		controllerCache := controllercache.New(pubSub)
		enrich := enricher.New(ctx, controllerCache)
		//nolint:govet // shadowing this err is fine
		fm, err := filtermanager.Init(5) //nolint:gomnd // defaults
		if err != nil {
			mainLogger.Fatal("unable to create filter manager", zap.Error(err))
		}
		defer fm.Stop() //nolint:errcheck // best effort
		enrich.Run()
		metricsModule := mm.InitModule(ctx, daemonConfig, pubSub, enrich, fm, controllerCache)

		if !daemonConfig.RemoteContext {
			mainLogger.Info("Initializing Pod controller")

			podController := pc.New(mgr.GetClient(), controllerCache)
			if err := podController.SetupWithManager(mgr); err != nil {
				mainLogger.Fatal("unable to create PodController", zap.Error(err))
			}
		} else if daemonConfig.EnableRetinaEndpoint {
			mainLogger.Info("RetinaEndpoint is enabled")
			mainLogger.Info("Initializing RetinaEndpoint controller")

			retinaEndpointController := kec.New(mgr.GetClient(), controllerCache)
			if err := retinaEndpointController.SetupWithManager(mgr); err != nil {
				mainLogger.Fatal("unable to create retinaEndpointController", zap.Error(err))
			}
		}

		mainLogger.Info("Initializing Node controller")
		nodeController := nc.New(mgr.GetClient(), controllerCache)
		if err := nodeController.SetupWithManager(mgr); err != nil {
			mainLogger.Fatal("unable to create nodeController", zap.Error(err))
		}

		mainLogger.Info("Initializing Service controller")
		svcController := sc.New(mgr.GetClient(), controllerCache)
		if err := svcController.SetupWithManager(mgr); err != nil {
			mainLogger.Fatal("unable to create svcController", zap.Error(err))
		}

		if daemonConfig.EnableAnnotations {
			mainLogger.Info("Initializing MetricsConfig namespaceController")
			namespaceController := namespacecontroller.New(mgr.GetClient(), controllerCache, metricsModule)
			if err := namespaceController.SetupWithManager(mgr); err != nil {
				mainLogger.Fatal("unable to create namespaceController", zap.Error(err))
			}
			go namespaceController.Start(ctx)
		} else {
			mainLogger.Info("Initializing MetricsConfig controller")
			metricsConfigController := mcc.New(mgr.GetClient(), mgr.GetScheme(), metricsModule)
			if err := metricsConfigController.SetupWithManager(mgr); err != nil {
				mainLogger.Fatal("unable to create metricsConfigController", zap.Error(err))
			}
		}
	}

	controllerMgr, err := cm.NewControllerManager(daemonConfig, cl, tel)
	if err != nil {
		mainLogger.Fatal("Failed to create controller manager", zap.Error(err))
	}
	if err := controllerMgr.Init(ctx); err != nil {
		mainLogger.Fatal("Failed to initialize controller manager", zap.Error(err))
	}
	// Stop is best effort. If it fails, we still want to stop the main process.
	// This is needed for graceful shutdown of Retina plugins.
	// Do it in the main thread as graceful shutdown is important.
	defer controllerMgr.Stop(ctx)

	// start heartbeat goroutine for application insights
	go tel.Heartbeat(ctx, heartbeatInterval)

	// Start controller manager, which will start http server and plugin manager.
	go controllerMgr.Start(ctx)
	mainLogger.Info("Started controller manager")

	// Start all registered controllers. This will block until container receives SIGTERM.
	if err := mgr.Start(ctx); err != nil {
		mainLogger.Fatal("unable to start manager", zap.Error(err))
	}

	mainLogger.Info("Network observability exiting. Till next time!")
	return nil
}
