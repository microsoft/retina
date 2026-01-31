// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
// Copyright Authors of Cilium.
// Modified by Authors of Retina.
package hubble

import (
	"github.com/cilium/cilium/pkg/datapath/link"
	"github.com/cilium/cilium/pkg/defaults"
	"github.com/cilium/cilium/pkg/gops"
	hubblecell "github.com/cilium/cilium/pkg/hubble/cell"
	ciliumparser "github.com/cilium/cilium/pkg/hubble/parser"
	k8sClient "github.com/cilium/cilium/pkg/k8s/client"
	"github.com/cilium/cilium/pkg/kpr"
	"github.com/cilium/cilium/pkg/kvstore"
	"github.com/cilium/cilium/pkg/logging"
	"github.com/cilium/cilium/pkg/logging/logfields"
	"github.com/cilium/cilium/pkg/node/manager"
	"github.com/cilium/cilium/pkg/option"
	"github.com/cilium/cilium/pkg/pprof"
	"github.com/cilium/cilium/pkg/recorder"
	"github.com/cilium/hive/cell"
	"github.com/cilium/statedb"
	"k8s.io/client-go/rest"

	"github.com/microsoft/retina/internal/buildinfo"
	"github.com/microsoft/retina/pkg/config"
	rnode "github.com/microsoft/retina/pkg/controllers/daemon/nodereconciler"
	"github.com/microsoft/retina/pkg/hubble/parser"
	"github.com/microsoft/retina/pkg/hubble/resources"
	retinak8s "github.com/microsoft/retina/pkg/k8s"
	"github.com/microsoft/retina/pkg/managers/pluginmanager"
	"github.com/microsoft/retina/pkg/monitoragent"
	"github.com/microsoft/retina/pkg/servermanager"
	"github.com/microsoft/retina/pkg/shared/telemetry"
)

// disabledKVStoreClient wraps a kvstore.Client but returns IsEnabled() = false.
// This is needed because K8sCiliumEndpointsWatcher only initializes if kvstore is disabled.
// When kvstore is enabled, Cilium expects CiliumEndpoint data to come from kvstore,
// but Retina watches CiliumEndpoint CRDs directly and needs the watcher to populate IPCache.
type disabledKVStoreClient struct {
	kvstore.Client
}

// IsEnabled returns false to indicate kvstore is not being used for CiliumEndpoint sync.
// This allows the K8sCiliumEndpointsWatcher to initialize and populate IPCache with K8sMetadata.
func (d *disabledKVStoreClient) IsEnabled() bool {
	return false
}

var (
	Agent = cell.Module(
		"agent",
		"Retina-Agent",
		Infrastructure,
		ControlPlane,
	)
	daemonSubsys = "daemon"
	logger       = logging.DefaultSlogLogger.With(logfields.LogSubsys, daemonSubsys)

	Infrastructure = cell.Module(
		"infrastructure",
		"Infrastructure",

		// Register the pprof HTTP handlers, to get runtime profiling data.
		pprof.Cell(pprof.Config{
			Pprof:        true,
			PprofAddress: option.PprofAddressAgent,
			PprofPort:    option.PprofPortAgent,
		}),

		// Runs the gops agent, a tool to diagnose Go processes.
		gops.Cell(true, defaults.GopsPortAgent),

		// Parse Retina specific configuration
		config.Cell,

		// Kubernetes client
		k8sClient.Cell,

		// Kube proxy replacement config (needed by loadbalancer cells)
		kpr.Cell,

		// Provide a disabled kvstore client for Retina.
		// This is important: the K8sCiliumEndpointsWatcher only initializes
		// if kvstore.IsEnabled() returns false (because with a real kvstore,
		// CiliumEndpoint data would come from kvstore instead of watching CRDs).
		// Since Retina doesn't use etcd/consul and relies on watching CiliumEndpoint CRDs,
		// we need IsEnabled() to return false so the watcher populates IPCache with K8sMetadata.
		cell.Provide(func(db *statedb.DB) kvstore.Client {
			return &disabledKVStoreClient{Client: kvstore.NewInMemoryClient(db, "default")}
		}),

		cell.Provide(func(cfg config.Config, k8sCfg *rest.Config) telemetry.Config {
			return telemetry.Config{
				Component:             "retina-agent",
				EnableTelemetry:       cfg.EnableTelemetry,
				ApplicationInsightsID: buildinfo.ApplicationInsightsID,
				RetinaVersion:         buildinfo.Version,
				EnabledPlugins:        cfg.EnabledPlugin,
			}
		}),
		telemetry.Constructor,

		// cell.Provide(func() cell.Lifecycle {
		// 	return &cell.DefaultLifecycle{}
		// }),
	)

	ControlPlane = cell.Module(
		"control-plane",
		"Control Plane",

		// monitorAgent.Cell,
		monitoragent.Cell,

		daemonCell,

		pluginmanager.Cell,

		servermanager.Cell,

		retinak8s.Cell,

		recorder.Cell,

		// Provides resources for hubble
		resources.Cell,

		// Provides link cache needed by hubble parser
		link.Cell,

		// Provides the node reconciler as node manager
		rnode.Cell,
		cell.Provide(
			func(nr *rnode.NodeReconciler) manager.NodeManager {
				return nr
			},
		),

		// Provides the full hubble agent (includes parser, exporter, metrics, and TLS)
		hubblecell.Cell,

		// Override Cilium's parser with Retina's parser that understands v1.Event from plugins
		cell.DecorateAll(func(_ ciliumparser.Decoder, params parser.Params) ciliumparser.Decoder {
			return parser.New(params)
		}),

		telemetry.Heartbeat,
	)
)
