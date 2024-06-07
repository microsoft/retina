package hubble

import (
	"context"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/managers/pluginmanager"
	"github.com/microsoft/retina/pkg/managers/servermanager"

	retinak8s "github.com/microsoft/retina/pkg/k8s"

	"github.com/cilium/cilium/pkg/hive/cell"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/cilium/pkg/ipcache"
	"github.com/cilium/cilium/pkg/k8s"
	k8sClient "github.com/cilium/cilium/pkg/k8s/client"
	"github.com/cilium/cilium/pkg/k8s/watchers"
	monitoragent "github.com/cilium/cilium/pkg/monitor/agent"
	"github.com/cilium/cilium/pkg/node"
	"github.com/cilium/workerpool"

	corev1 "k8s.io/api/core/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	zapf "sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var (
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
				Scheme: scheme,
				Metrics: metricsserver.Options{
					BindAddress: rcfg.MetricsBindAddress,
				},
				HealthProbeBindAddress: rcfg.HealthProbeBindAddress,
				LeaderElection:         rcfg.LeaderElection,
				LeaderElectionID:       "ecaf1259.retina.io",
			}

			logf.SetLogger(zapf.New())
			ctrlManager, err := ctrl.NewManager(k8sCfg, mgrOption)
			if err != nil {
				logger.Error("failed to create manager")
				return nil, nil, err
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

type Daemon struct {
	clientset k8sClient.Clientset

	log            logrus.FieldLogger
	monitorAgent   monitoragent.Agent
	pluginManager  *pluginmanager.PluginManager
	HTTPServer     *servermanager.HTTPServer
	client         client.Client
	eventChan      chan *v1.Event
	k8swatcher     *watchers.K8sWatcher
	localNodeStore *node.LocalNodeStore
	ipc            *ipcache.IPCache
	svcCache       *k8s.ServiceCache
}

func newDaemon(params *daemonParams) (*Daemon, error) {
	return &Daemon{
		monitorAgent:   params.MonitorAgent,
		pluginManager:  params.PluginManager,
		HTTPServer:     params.HTTPServer,
		clientset:      params.Clientset,
		log:            params.Log,
		client:         params.Client,
		eventChan:      params.EventChan,
		k8swatcher:     params.K8sWatcher,
		localNodeStore: params.Lnds,
		ipc:            params.IPC,
		svcCache:       params.SvcCache,
	}, nil
}

func (d *Daemon) Run(ctx context.Context) error {
	// Start K8s watcher
	d.log.WithField("localNodeStore", d.localNodeStore).Info("Starting local node store")

	// Start K8s watcher. Will block till sync is complete or timeout.
	// If sync doesn't complete within timeout (3 minutes), causes fatal error.
	retinak8s.Start(ctx, d.k8swatcher)

	go d.generateEvents(ctx)
	return nil
}

func (d *Daemon) generateEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-d.eventChan:
			d.log.WithField("event", event).Debug("Sending event to monitor agent")
			err := d.monitorAgent.SendEvent(0, event)
			if err != nil {
				d.log.WithError(err).Error("Unable to send event to monitor agent")
			}
		}
	}
}
