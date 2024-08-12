package networkpolicy

import (
	"github.com/cilium/cilium/pkg/hive/cell"
	"github.com/spf13/pflag"
)

var Cell = cell.Config(defaultConfig)

type Config struct {
	EnableNetworkPolicyEnrichment bool
}

var defaultConfig = Config{
	EnableNetworkPolicyEnrichment: true,
}

func (cfg Config) Flags(flags *pflag.FlagSet) {
	flags.Bool("enable-network-policy-enrichment", cfg.EnableNetworkPolicyEnrichment, "Watch network policies and enrich flows/metrics with information on which policies caused dropped traffic")
}
