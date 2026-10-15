// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium and Retina

// NOTE: copied and slimmed down for our use case

package ciliumcrds

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cilium/cilium/pkg/defaults"
	"github.com/cilium/cilium/pkg/option"
)

var (
	durationLeaderElector     = 2 * time.Second
	durationNonLeaderOperator = 15 * time.Second
	durationActingMaster      = 10 * time.Second
)

// Cilium v1.20 stopped exporting these names; keep their values stable for
// existing operator configuration.
const (
	leaderElectionLeaseDuration = "leader-election-lease-duration"
	leaderElectionRenewDeadline = "leader-election-renew-deadline"
	leaderElectionRetryPeriod   = "leader-election-retry-period"
)

func InitGlobalFlags(cmd *cobra.Command, vp *viper.Viper) {
	flags := cmd.Flags()

	// include this line so that we don't see the following log from Cilium:
	// "Running Cilium with \"kvstore\"=\"\" requires identity allocation via CRDs. Changing identity-allocation-mode to \"crd\""
	flags.String(option.IdentityAllocationMode, option.IdentityAllocationModeCRD, "Identity allocation mode")

	flags.String(option.ConfigFile, "", `Configuration file (to configure the operator, this argument is required)`)
	option.BindEnv(vp, option.ConfigFile)

	flags.String(option.ConfigDir, "", `Configuration directory that contains a file for each option`)
	option.BindEnv(vp, option.ConfigDir)

	flags.BoolP(option.DebugArg, "D", false, "Enable debugging mode")
	option.BindEnv(vp, option.DebugArg)

	// NOTE: without this the option gets overridden from the default value to the zero value via option.Config.Populate(vp)
	// specifically, here options.Config.AllocatorListTimeout gets overridden from the default value to 0s
	flags.Duration(option.AllocatorListTimeoutName, defaults.AllocatorListTimeout, "timeout to list initial allocator state")

	flags.StringSlice(option.LogDriver, []string{}, "Logging endpoints to use for example syslog")
	option.BindEnv(vp, option.LogDriver)

	flags.Var(option.NewMapOptions(&option.Config.LogOpt),
		option.LogOpt, `Log driver options for cilium-operator, `+
			`configmap example for syslog driver: {"syslog.level":"info","syslog.facility":"local4"}`)
	option.BindEnv(vp, option.LogOpt)

	flags.Duration(leaderElectionLeaseDuration, durationNonLeaderOperator,
		"Duration that non-leader operator candidates will wait before forcing to acquire leadership")
	option.BindEnv(vp, leaderElectionLeaseDuration)

	flags.Duration(leaderElectionRenewDeadline, durationActingMaster,
		"Duration that current acting master will retry refreshing leadership in before giving up the lock")
	option.BindEnv(vp, leaderElectionRenewDeadline)

	flags.Duration(leaderElectionRetryPeriod, durationLeaderElector,
		"Duration that LeaderElector clients should wait between retries of the actions")
	option.BindEnv(vp, leaderElectionRetryPeriod)

	err := vp.BindPFlags(flags)
	if err != nil {
		fmt.Printf("Failed to bind flags: %v\n", err)
	}
}
