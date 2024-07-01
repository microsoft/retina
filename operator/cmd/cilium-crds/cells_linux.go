// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium and Retina

// NOTE: separated the cells from root.go into this file.
// See other note in root.go for modification info.

package ciliumcrds

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/microsoft/retina/pkg/shared/telemetry"
	"github.com/sirupsen/logrus"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	zapf "sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/microsoft/retina/operator/cilium-crds/config"
	operatorK8s "github.com/microsoft/retina/operator/cilium-crds/k8s"
	"github.com/microsoft/retina/operator/cilium-crds/k8s/apis"
	endpointcontroller "github.com/microsoft/retina/pkg/controllers/operator/cilium-crds/endpoint"

	"github.com/cilium/cilium/operator/auth"
	"github.com/cilium/cilium/operator/endpointgc"
	"github.com/cilium/cilium/operator/identitygc"
	operatorMetrics "github.com/cilium/cilium/operator/metrics"
	operatorOption "github.com/cilium/cilium/operator/option"
	cmtypes "github.com/cilium/cilium/pkg/clustermesh/types"
	"github.com/cilium/cilium/pkg/controller"
	"github.com/cilium/cilium/pkg/hive/cell"
	k8sClient "github.com/cilium/cilium/pkg/k8s/client"
	"github.com/cilium/cilium/pkg/kvstore/store"
	"github.com/cilium/cilium/pkg/option"
	"github.com/cilium/cilium/pkg/pprof"
)

const operatorK8sNamespace = "kube-system"

var (
	Operator = cell.Module(
		"operator",
		"Retina Operator",

		cell.Invoke(func(l logrus.FieldLogger) {
			// to help prevent user confusion, explain why logs may include lines referencing "cilium" or "cilium operator"
			// e.g. level=info msg="Cilium Operator  go version go1.21.4 linux/amd64" subsys=retina-operator
			l.Info("starting hive. Some logs will say 'cilium' since some code is derived from cilium")
		}),
		Infrastructure,
		ControlPlane,
	)

	Infrastructure = cell.Module(
		"operator-infra",
		"Operator Infrastructure",

		// operator config
		config.Cell,

		// start sending logs to zap telemetry (if enabled)
		cell.Invoke(setupZapHook),

		cell.Provide(func(cfg config.Config) telemetry.Config {
			return telemetry.Config{
				Component:             "retina-operator",
				EnableTelemetry:       cfg.EnableTelemetry,
				ApplicationInsightsID: applicationInsightsID,
				RetinaVersion:         retinaVersion,
			}
		}),
		telemetry.Constructor,

		// Register the pprof HTTP handlers, to get runtime profiling data.
		pprof.Cell,
		cell.Config(pprof.Config{
			Pprof:        true,
			PprofAddress: option.PprofAddressAgent,
			PprofPort:    option.PprofPortAgent,
		}),

		// // Runs the gops agent, a tool to diagnose Go processes.
		// gops.Cell(defaults.GopsPortOperator),

		// Provides Clientset, API for accessing Kubernetes objects.
		k8sClient.Cell,

		// Provides the modular metrics registry, metric HTTP server and legacy metrics cell.
		// NOTE: no server/metrics are created when --enable-metrics=false (default)
		operatorMetrics.Cell,
		cell.Provide(func(
			operatorCfg *operatorOption.OperatorConfig,
		) operatorMetrics.SharedConfig {
			return operatorMetrics.SharedConfig{
				// Cloud provider specific allocators needs to read operatorCfg.EnableMetrics
				// to add their metrics when it's set to true. Therefore, we leave the flag as global
				// instead of declaring it as part of the metrics cell.
				// This should be changed once the IPAM allocator is modularized.
				EnableMetrics:    operatorCfg.EnableMetrics,
				EnableGatewayAPI: operatorCfg.EnableGatewayAPI,
			}
		}),
		cell.Provide(func() *k8sruntime.Scheme {
			scheme := k8sruntime.NewScheme()

			//+kubebuilder:scaffold:scheme
			utilruntime.Must(clientgoscheme.AddToScheme(scheme))

			return scheme
		}),
	)

	// ControlPlane implements the control functions.
	ControlPlane = cell.Module(
		"operator-controlplane",
		"Operator Control Plane",

		cell.Config(cmtypes.DefaultClusterInfo),
		cell.Invoke(func(cinfo cmtypes.ClusterInfo) error {
			err := cinfo.Validate()
			if err != nil {
				return fmt.Errorf("error validating cluster info: %w", err)
			}
			return nil
		}),

		cell.Invoke(
			registerOperatorHooks,
		),

		cell.Provide(func() *option.DaemonConfig {
			return option.Config
		}),

		cell.Provide(func() *operatorOption.OperatorConfig {
			return operatorOption.Config
		}),

		cell.Provide(func(
			daemonCfg *option.DaemonConfig,
			_ *operatorOption.OperatorConfig,
		) identitygc.SharedConfig {
			return identitygc.SharedConfig{
				IdentityAllocationMode: daemonCfg.IdentityAllocationMode,
			}
		}),

		// TODO uncomment if we use endpoint slices
		// cell.Provide(func(
		// 	daemonCfg *option.DaemonConfig,
		// ) ciliumendpointslice.SharedConfig {
		// 	return ciliumendpointslice.SharedConfig{
		// 		EnableCiliumEndpointSlice: daemonCfg.EnableCiliumEndpointSlice,
		// 	}
		// }),

		cell.Provide(func(
			operatorCfg *operatorOption.OperatorConfig,
			daemonCfg *option.DaemonConfig,
		) endpointgc.SharedConfig {
			return endpointgc.SharedConfig{
				Interval:                 operatorCfg.EndpointGCInterval,
				DisableCiliumEndpointCRD: daemonCfg.DisableCiliumEndpointCRD,
			}
		}),

		// TODO: uncomment to expose healthz endpoint for this hive?
		// api.HealthHandlerCell(
		// 	kvstoreEnabled,
		// 	isLeader.Load,
		// ),

		// NOTE: might need to uncomment to support metrics? Code might be legacy though?
		// api.MetricsHandlerCell,

		controller.Cell,

		// These cells are started only after the operator is elected leader.
		WithLeaderLifecycle(
			// The CRDs registration should be the first operation to be invoked after the operator is elected leader.
			apis.RegisterCRDsCell,

			// below cluster of cells carries out Retina's custom operator logic for creating Cilium Identities and Endpoints
			cell.Provide(func(scheme *k8sruntime.Scheme) (ctrl.Manager, error) {
				// controller-runtime requires its own logger
				logf.SetLogger(zapf.New())

				manager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
					Scheme: scheme,
					// Metrics: server.Options{
					// 	BindAddress: metricsAddr,
					// },
					// FIXME readiness probe not working after controller-runtime upgrade
					// FIXME: don't know where this field went in controller-runtime v0.16.2 (previous: v0.15.0)
					// Port:                   9443,
					// HealthProbeBindAddress: probeAddr,
				})
				if err != nil {
					return nil, fmt.Errorf("failed to create manager: %w", err)
				}
				return manager, nil
			}),
			endpointcontroller.Cell,

			// below cells are required to run identitygc and endpointgc when the operator is elected leader

			// NOTE: we've slimmed down the resources required here (see resources.go for more info)
			operatorK8s.ResourcesCell,

			// NOTE: gc cells require this auth cell
			// this cell will do nothing since --mesh-auth-mutual-enabled=false
			auth.Cell,

			store.Cell,

			identitygc.Cell,

			// TODO: uncomment if we use endpoint slices
			// CiliumEndpointSlice controller depends on the CiliumEndpoint and
			// CiliumEndpointSlice resources. It reconciles the state of CESs in the
			// cluster based on the CEPs and CESs events.
			// It is disabled if CiliumEndpointSlice is disabled in the cluster -
			// when --enable-cilium-endpoint-slice is false.
			// ciliumendpointslice.Cell,

			// Cilium Endpoint Garbage Collector. It removes all leaked Cilium
			// Endpoints. Either once or periodically it validates all the present
			// Cilium Endpoints and delete the ones that should be deleted.
			endpointgc.Cell,

			// only send heartbeat if leader
			telemetry.Heartbeat,
		),
	)

	FlagsHooks []ProviderFlagsHooks

	leaderElectionResourceLockName = "cilium-operator-resource-lock"

	// Use a Go context so we can tell the leaderelection code when we
	// want to step down
	leaderElectionCtx       context.Context
	leaderElectionCtxCancel context.CancelFunc

	// isLeader is an atomic boolean value that is true when the Operator is
	// elected leader. Otherwise, it is false.
	isLeader atomic.Bool
)
