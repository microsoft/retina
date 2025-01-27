package hubble

import (
	"context"
	"fmt"

	"github.com/cilium/cilium/api/v1/flow"
	"github.com/cilium/cilium/pkg/crypto/certloader"
	"github.com/cilium/cilium/pkg/hubble/container"
	"github.com/cilium/cilium/pkg/hubble/metrics"
	"github.com/cilium/cilium/pkg/hubble/monitor"
	"github.com/cilium/cilium/pkg/hubble/observer"
	"github.com/cilium/cilium/pkg/hubble/observer/observeroption"
	"github.com/cilium/cilium/pkg/hubble/peer"
	"github.com/cilium/cilium/pkg/hubble/peer/serviceoption"
	"github.com/cilium/cilium/pkg/hubble/server"
	"github.com/cilium/cilium/pkg/hubble/server/serveroption"
	"github.com/cilium/cilium/pkg/ipcache"
	"github.com/cilium/cilium/pkg/k8s"
	"github.com/cilium/cilium/pkg/logging/logfields"
	monitoragent "github.com/cilium/cilium/pkg/monitor/agent"
	"github.com/cilium/cilium/pkg/option"
	"github.com/cilium/hive/cell"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	rnode "github.com/microsoft/retina/pkg/controllers/daemon/nodereconciler"
	"github.com/microsoft/retina/pkg/hubble/parser"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
)

type RetinaHubble struct {
	log            *logrus.Entry
	client         client.Client
	monitorAgent   monitoragent.Agent
	svc            *k8s.ServiceCache
	ipc            *ipcache.IPCache
	nodeReconciler *rnode.NodeReconciler
}

type hubbleParams struct {
	cell.In

	Client         client.Client
	MonitorAgent   monitoragent.Agent
	ServiceCache   *k8s.ServiceCache
	IPCache        *ipcache.IPCache
	NodeReconciler *rnode.NodeReconciler
	Log            logrus.FieldLogger
}

func newRetinaHubble(params hubbleParams) *RetinaHubble {
	rh := &RetinaHubble{
		log:            params.Log.WithField(logfields.LogSubsys, "retina-hubble"),
		client:         params.Client,
		monitorAgent:   params.MonitorAgent,
		svc:            params.ServiceCache,
		ipc:            params.IPCache,
		nodeReconciler: params.NodeReconciler,
	}
	rh.log.Logger.SetLevel(logrus.InfoLevel)

	return rh
}

func (rh *RetinaHubble) defaultOptions() {
	// Not final, will be updated later.
	option.Config.HubblePreferIpv6 = false
	option.Config.EnableHighScaleIPcache = false
	option.Config.EnableHubbleOpenMetrics = false

	rh.log.Info("Starting Hubble with configuration", zap.Any("config", option.Config))
}

func (rh *RetinaHubble) getHubbleEventBufferCapacity() (container.Capacity, error) {
	kap, err := container.NewCapacity(option.Config.HubbleEventBufferCapacity)
	if err != nil {
		return nil, fmt.Errorf("creating container capacity: %w", err)
	}
	return kap, nil
}

func (rh *RetinaHubble) start(ctx context.Context) error {
	var (
		localSrvOpts []serveroption.Option
		remoteOpts   []serveroption.Option
		observerOpts []observeroption.Option
		// parserOpts   []parserOptions.Option
	)

	// ---------------------------------------------------------------------------------------------------------------------------------------------------- //
	// Setup metrics.
	grpcMetrics := grpc_prometheus.NewServerMetrics()
	if err := metrics.EnableMetrics(rh.log.Logger, option.Config.HubbleMetricsServer, nil, option.Config.HubbleMetrics, grpcMetrics, option.Config.EnableHubbleOpenMetrics); err != nil {
		rh.log.Error("Failed to enable metrics", zap.Error(err))
		return fmt.Errorf("enabling metrics: %w", err)
	}

	// ---------------------------------------------------------------------------------------------------------------------------------------------------- //
	// Start the Hubble observer.
	maxFlows, err := rh.getHubbleEventBufferCapacity()
	if err != nil {
		rh.log.Error("Failed to get Hubble event buffer capacity", zap.Error(err))
		return err
	}
	observerOpts = append(observerOpts,
		observeroption.WithMaxFlows(maxFlows),
		observeroption.WithMonitorBuffer(option.Config.HubbleEventQueueSize),
		observeroption.WithOnDecodedFlowFunc(func(ctx context.Context, flow *flow.Flow) (bool, error) {
			err = metrics.ProcessFlow(ctx, flow)
			if err != nil {
				rh.log.Error("Failed to process flow", zap.Any("flow", flow), zap.Error(err))
				return false, fmt.Errorf("processing flow: %w", err)
			}
			return false, nil
		}),
	)

	// TODO: Replace with our custom parser.
	payloadParser := parser.New(rh.log, rh.svc, rh.ipc)

	namespaceManager := observer.NewNamespaceManager()
	go namespaceManager.Run(ctx)

	hubbleObserver, err := observer.NewLocalServer(
		payloadParser,
		namespaceManager,
		rh.log,
		observerOpts...,
	)
	if err != nil {
		rh.log.Error("Failed to create Hubble observer", zap.Error(err))
		return fmt.Errorf("starting local server: %w", err)
	}
	go hubbleObserver.Start()

	// Registering the Observer as consumer for monitor events.
	rh.monitorAgent.RegisterNewConsumer(monitor.NewConsumer(hubbleObserver))

	// ---------------------------------------------------------------------------------------------------------------------------------------------------- //
	// Start the local server.
	sockPath := "unix://" + option.Config.HubbleSocketPath
	var peerServiceOptions []serviceoption.Option
	var tlsCfg *certloader.WatchedServerConfig

	tlsPeerOpt := []serviceoption.Option{serviceoption.WithoutTLSInfo()}
	tlsSrvOpt := serveroption.WithInsecure()
	if !option.Config.HubbleTLSDisabled {
		tlsCfg, err = rh.fetchTLSConfig(ctx)
		if err != nil {
			return errors.Wrap(err, "fetching TLS config")
		}

		tlsPeerOpt = []serviceoption.Option{}
		tlsSrvOpt = serveroption.WithServerTLS(tlsCfg)
	}
	peerServiceOptions = append(peerServiceOptions, tlsPeerOpt...)

	peerSvc := peer.NewService(rh.nodeReconciler, peerServiceOptions...)
	localSrvOpts = append(localSrvOpts,
		serveroption.WithUnixSocketListener(sockPath),
		serveroption.WithHealthService(),
		serveroption.WithObserverService(hubbleObserver),
		serveroption.WithPeerService(peerSvc),
		// The local server does not need to be guarded by TLS.
		// It's only used for local communication.
		serveroption.WithInsecure(),
	)

	localSrv, err := server.NewServer(rh.log, localSrvOpts...)
	if err != nil {
		rh.log.Error("Failed to initialize local Hubble server", zap.Error(err))
		return fmt.Errorf("starting peer service: %w", err)
	}
	rh.log.Info("Started local Hubble server", zap.String("address", sockPath))

	go func() {
		//nolint:govet // shadowing the err is intentional here
		if err := localSrv.Serve(); err != nil {
			rh.log.Error("Error while serving from local Hubble server", zap.Error(err))
		}
	}()
	// Cleanup the local socket on exit.
	go func() {
		<-ctx.Done()
		localSrv.Stop()
		peerSvc.Close()
		rh.log.Info("Stopped local Hubble server")
	}()

	// ---------------------------------------------------------------------------------------------------------------------------------------------------- //
	// Start remote server.
	address := option.Config.HubbleListenAddress
	remoteOpts = append(remoteOpts,
		serveroption.WithTCPListener(address),
		serveroption.WithHealthService(),
		serveroption.WithPeerService(peerSvc),
		serveroption.WithObserverService(hubbleObserver),
		tlsSrvOpt,
	)

	srv, err := server.NewServer(rh.log, remoteOpts...)
	if err != nil {
		rh.log.Error("Failed to initialize Hubble remote server", zap.Error(err))
		return fmt.Errorf("starting remote server: %w", err)
	}
	rh.log.Info("Started Hubble remote server", zap.String("address", address))

	go func() {
		if err := srv.Serve(); err != nil {
			rh.log.Error("Error while serving from Hubble remote server", zap.Error(err))
		}
	}()
	// Cleanup the remote server on exit.
	go func() {
		<-ctx.Done()
		srv.Stop()
		rh.log.Info("Stopped Hubble remote server")
	}()
	return nil
}

func (rh *RetinaHubble) fetchTLSConfig(ctx context.Context) (*certloader.WatchedServerConfig, error) {
	tlsChan, err := certloader.FutureWatchedServerConfig(rh.log, option.Config.HubbleTLSClientCAFiles, option.Config.HubbleTLSCertFile, option.Config.HubbleTLSKeyFile)
	if err != nil {
		return nil, errors.Wrap(err, "retrieving TLS configuration future")
	}

	rh.log.Info("waiting for TLS credentials")
	select {
	case t := <-tlsChan:
		rh.log.Info("received TLS credentials")

		// ensure the certificate fetching stops when the context is canceled
		go func() {
			<-ctx.Done()
			t.Stop()
		}()

		return t, nil
	case <-ctx.Done():
		return nil, errors.Wrap(ctx.Err(), "waiting for TLS credentials")
	}
}

func (rh *RetinaHubble) launchWithDefaultOptions(ctx context.Context) error {
	rh.defaultOptions()
	return rh.start(ctx)
}
