// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package standard

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	crcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kcfg "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	crmgr "sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/go-logr/zapr"
	"github.com/microsoft/retina/cmd/observability"
	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/internal/buildinfo"
	"github.com/microsoft/retina/pkg/config"
	controllercache "github.com/microsoft/retina/pkg/controllers/cache"
	mcc "github.com/microsoft/retina/pkg/controllers/daemon/metricsconfiguration"
	namespacecontroller "github.com/microsoft/retina/pkg/controllers/daemon/namespace"
	nc "github.com/microsoft/retina/pkg/controllers/daemon/node"
	pc "github.com/microsoft/retina/pkg/controllers/daemon/pod"
	kec "github.com/microsoft/retina/pkg/controllers/daemon/retinaendpoint"
	sc "github.com/microsoft/retina/pkg/controllers/daemon/service"

	"github.com/microsoft/retina/pkg/enricher"
	cm "github.com/microsoft/retina/pkg/managers/controllermanager"
	"github.com/microsoft/retina/pkg/managers/filtermanager"
	"github.com/microsoft/retina/pkg/metrics"
	mm "github.com/microsoft/retina/pkg/module/metrics"
	"github.com/microsoft/retina/pkg/pubsub"
)

const (
	nodeNameEnvKey = "NODE_NAME"
	nodeIPEnvKey   = "NODE_IP"
)

var scheme = k8sruntime.NewScheme()

func init() {
	//+kubebuilder:scaffold:scheme
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(retinav1alpha1.AddToScheme(scheme))
}

type Daemon struct {
	configFile           string
	metricsAddr          string
	probeAddr            string
	enableLeaderElection bool
}

func NewDaemon(configFile, metricsAddr, probeAddr string, enableLeaderElection bool) *Daemon {
	return &Daemon{
		configFile:           configFile,
		metricsAddr:          metricsAddr,
		probeAddr:            probeAddr,
		enableLeaderElection: enableLeaderElection,
	}
}

func (d *Daemon) Start() error {
	fmt.Printf("Starting Retina daemon with legacy control plane %v\n", buildinfo.Version)
	fmt.Println("init client-go")

	daemonCfg, err := config.GetConfig(d.configFile)
	if err != nil {
		panic(err)
	}
	zl := observability.InitializeLogger(daemonCfg.LogLevel, daemonCfg.EnableTelemetry, daemonCfg.EnabledPlugin, daemonCfg.DataAggregationLevel)

	var restCfg *rest.Config
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		fmt.Println("KUBECONFIG detected, using kubeconfig: ", kubeconfig)
		restCfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return fmt.Errorf("creating controller-runtime manager: %w", err)
		}
	} else {
		restCfg, err = kcfg.GetConfig()
		if err != nil {
			panic(err)
		}
	}

	fmt.Println("api server: ", restCfg.Host)

	mainLogger := zl.Named("main").Sugar().With(
		"apiserver", restCfg.Host,
	)

	// Allow the current process to lock memory for eBPF resources.
	// OS specific implementation.
	// This is a no-op on Windows.
	if err = d.RemoveMemlock(); err != nil {
		mainLogger.Fatal("failed to remove memlock", zap.Error(err))
	}

	metrics.InitializeMetrics()
	mainLogger.Info(zap.String("data aggregation level", daemonCfg.DataAggregationLevel.String()))

	tel, err := observability.InitializeTelemetryClient(restCfg, daemonCfg.EnabledPlugin, daemonCfg.EnableTelemetry, mainLogger)
	if err != nil {
		return fmt.Errorf("failed to initialize telemetry client: %w", err)
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
	if daemonCfg.EnablePodLevel && !daemonCfg.RemoteContext {
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

	mgr, err := crmgr.New(restCfg, mgrOption)
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

	if daemonCfg.EnablePodLevel {
		pubSub := pubsub.New()
		controllerCache := controllercache.New(pubSub)
		enrich := enricher.NewStandard(ctx, controllerCache)
		//nolint:govet // shadowing this err is fine
		fm, err := filtermanager.Init(5) //nolint:gomnd // defaults
		if err != nil {
			mainLogger.Fatal("unable to create filter manager", zap.Error(err))
		}
		defer fm.Stop() //nolint:errcheck // best effort
		enrich.Run()
		metricsModule := mm.InitModule(ctx, daemonCfg, pubSub, enrich, fm, controllerCache)

		if !daemonCfg.RemoteContext {
			mainLogger.Info("Initializing Pod controller")

			podController := pc.New(mgr.GetClient(), controllerCache)
			if err := podController.SetupWithManager(mgr); err != nil {
				mainLogger.Fatal("unable to create PodController", zap.Error(err))
			}
		} else if daemonCfg.EnableRetinaEndpoint {
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

		if daemonCfg.EnableAnnotations {
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

	controllerMgr, err := cm.NewStandardControllerManager(daemonCfg, cl, tel)
	if err != nil {
		mainLogger.Fatal("Failed to create controller manager", zap.Error(err))
	}
	if err := controllerMgr.Init(ctx); err != nil {
		mainLogger.Fatal("Failed to initialize controller manager", zap.Error(err))
	}
	// Stop is best effort. If it fails, we still want to stop the main process.
	// This is needed for graceful shutdown of Retina plugins.
	// Do it in the main thread as graceful shutdown is important.
	defer controllerMgr.Stop()

	// start heartbeat goroutine for application insights
	go tel.Heartbeat(ctx, daemonCfg.TelemetryInterval)

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
