package config

import (
	"github.com/cilium/hive/cell"
	"github.com/spf13/pflag"

	sharedconfig "github.com/microsoft/retina/pkg/shared/config"
)

type Config struct {
	EnableTelemetry bool
	LeaderElection  bool
}

func (c Config) Flags(flags *pflag.FlagSet) {
	flags.Bool("enable-telemetry", c.EnableTelemetry, "enable telemetry (send logs and metrics to a remote server)")
	flags.Bool("leader-election", c.LeaderElection, "Enable leader election for operator. Ensures there is only one active operator Pod")
}

var (
	DefaultConfig = Config{
		EnableTelemetry: false,
		LeaderElection:  false,
	}

	Cell = cell.Module(
		"operator-config",
		"Operator Config",
		cell.Config(DefaultConfig),
		sharedconfig.Cell,
	)
)
