// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
// Copyright Authors of Cilium.
// Modified by Authors of Retina.
package hubble

import (
	"github.com/cilium/cilium/pkg/defaults"
	"github.com/cilium/cilium/pkg/gops"
	hubblecell "github.com/cilium/cilium/pkg/hubble/cell"
	exportercell "github.com/cilium/cilium/pkg/hubble/exporter/cell"
	hubbleParser "github.com/cilium/cilium/pkg/hubble/parser"
	"github.com/cilium/cilium/pkg/ipcache"
	"github.com/cilium/cilium/pkg/k8s"
	k8sClient "github.com/cilium/cilium/pkg/k8s/client"
	"github.com/cilium/cilium/pkg/logging"
	"github.com/cilium/cilium/pkg/logging/logfields"
	"github.com/cilium/cilium/pkg/node/manager"
	"github.com/cilium/cilium/pkg/option"
	"github.com/cilium/cilium/pkg/pprof"
	"github.com/cilium/cilium/pkg/recorder"
	"github.com/cilium/hive/cell"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"

	"github.com/microsoft/retina/internal/buildinfo"
	"github.com/microsoft/retina/pkg/config"
	rnode "github.com/microsoft/retina/pkg/controllers/daemon/nodereconciler"
	"github.com/microsoft/retina/pkg/hubble/parser"
	retinak8s "github.com/microsoft/retina/pkg/k8s"
	"github.com/microsoft/retina/pkg/managers/pluginmanager"
	"github.com/microsoft/retina/pkg/monitoragent"
	"github.com/microsoft/retina/pkg/servermanager"
	"github.com/microsoft/retina/pkg/shared/telemetry"
)

var (
	Agent = cell.Module(
		"agent",
		"Retina-Agent",
		Infrastructure,
		ControlPlane,
	)
	daemonSubsys = "daemon"
	logger       = logging.DefaultLogger.WithField(logfields.LogSubsys, daemonSubsys)

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

		cell.Provide(
			func(l logrus.FieldLogger, ipc *ipcache.IPCache, sc *k8s.ServiceCacheImpl) hubbleParser.Decoder {
				return parser.New(l.WithField("decoder", nil), sc, ipc)
			},
		),

		// Provides the node reconciler as node manager
		rnode.Cell,
		cell.Provide(
			func(nr *rnode.NodeReconciler) manager.NodeManager {
				return nr
			},
		),

		exportercell.Cell,
		// Provides the hubble agent
		hubblecell.Core,

		telemetry.Heartbeat,
	)
)
