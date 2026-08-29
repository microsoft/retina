// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package config

import (
	"path/filepath"

	"github.com/cilium/cilium/pkg/option"
	"github.com/cilium/hive/cell"
	sharedconfig "github.com/microsoft/retina/pkg/shared/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

const configFileName string = "config.yaml"

// RetinaHubbleConfig is a collection of configuration information needed by
// Retina-services for proper functioning.
type RetinaHubbleConfig struct {
	// NOTE: metrics-bind-address and health-probe-bind-address should be used ONLY as container args (NOT in ConfigMap) to keep parity with non-enterprise Retina
	MetricsBindAddress     string
	HealthProbeBindAddress string

	LeaderElection bool
	ClusterName    string // the name of the cluster (primarily used for TLS)
}

// Flags is responsible for binding flags provided by the user to the various
// fields of the Config.
func (c RetinaHubbleConfig) Flags(flags *pflag.FlagSet) {
	// NOTE: metrics-bind-address and health-probe-bind-address should be used ONLY as container args (NOT in ConfigMap) to keep parity with non-enterprise Retina
	flags.String("metrics-bind-address", c.MetricsBindAddress, "The address the metric endpoint binds to.")
	flags.String("health-probe-bind-address", c.HealthProbeBindAddress, "The address the probe endpoint binds to.")

	flags.Bool("leader-election", c.LeaderElection, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flags.String("cluster-name", c.ClusterName, "name of the cluster")
}

var (
	DefaultRetinaHubbleConfig = RetinaHubbleConfig{
		MetricsBindAddress:     ":18000",
		HealthProbeBindAddress: ":18001",
		LeaderElection:         false,
		ClusterName:            "default",
	}

	DefaultRetinaConfig = &Config{
		EnableTelemetry:          false,
		EnabledPlugin:            []string{"packetforward", "dropreason", "linuxutil", "dns"},
		EnablePodLevel:           true,
		LogLevel:                 "info",
		BypassLookupIPOfInterest: true,
		DataAggregationLevel:     High,
	}

	Cell = cell.Module(
		"agent-config",
		"Agent Config",

		// Provide option.Config via hive so cells can depend on the agent config.
		cell.Provide(func() *option.DaemonConfig {
			return option.Config
		}),

		cell.Config(DefaultRetinaHubbleConfig),

		cell.Provide(func(logger logrus.FieldLogger) (Config, error) {
			retinaConfigFile := filepath.Join(option.Config.ConfigDir, configFileName)
			conf, err := GetConfig(retinaConfigFile)
			if err != nil {
				logger.Error(err)
				conf = DefaultRetinaConfig
			}
			logger.Info(conf)
			return *conf, nil
		}),
		sharedconfig.Cell,
	)
)
