// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
// Copyright Authors of Cilium.
// Modified by Authors of Retina.
package dns_notify_forwarding

import (
	"github.com/cilium/cilium/pkg/defaults"
	"github.com/cilium/cilium/pkg/gops"
	"github.com/cilium/cilium/pkg/hive/cell"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	k8sClient "github.com/cilium/cilium/pkg/k8s/client"
	"github.com/cilium/cilium/pkg/option"
	"github.com/cilium/cilium/pkg/pprof"
	"github.com/cilium/proxy/pkg/logging"
	"github.com/cilium/proxy/pkg/logging/logfields"
	"github.com/microsoft/retina/pkg/config"
	rnode "github.com/microsoft/retina/pkg/controllers/daemon/nodereconciler"
	retinak8s "github.com/microsoft/retina/pkg/k8s"
	"github.com/microsoft/retina/pkg/managers/pluginmanager"
	"github.com/microsoft/retina/pkg/shared/telemetry"
	"k8s.io/client-go/rest"
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
		pprof.Cell,
		cell.Config(pprof.Config{
			Pprof:        true,
			PprofAddress: option.PprofAddressAgent,
			PprofPort:    option.PprofPortAgent,
		}),

		// Runs the gops agent, a tool to diagnose Go processes.
		gops.Cell(defaults.GopsPortAgent),

		// Parse Retina specific configuration
		config.Cell,

		// Kubernetes client
		k8sClient.Cell,

		cell.Provide(func(cfg config.Config, k8sCfg *rest.Config) telemetry.Config {
			return telemetry.Config{
				Component:             "retina-agent",
				EnableTelemetry:       cfg.EnableTelemetry,
				ApplicationInsightsID: applicationInsightsID,
				RetinaVersion:         retinaVersion,
				EnabledPlugins:        []string{"dns"},
			}
		}),
		telemetry.Constructor,
	)

	ControlPlane = cell.Module(
		"control-plane",
		"Control Plane",

		// provides retina with controllermanager
		daemonCell,

		// Provides the node reconciler
		rnode.Cell,

		// events channel used by plugin manager and dns notify forward cell
		cell.Provide(func() chan *v1.Event {
			return make(chan *v1.Event, pluginmanager.DefaultExternalEventChannelSize)
		}),

		pluginmanager.Cell,

		retinak8s.Cell,

		telemetry.Heartbeat,

		dnsNotifyForwardCell,
	)
)
