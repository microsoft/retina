// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package cmd

import (
	"fmt"

	"github.com/cilium/cilium/pkg/hive"
	"github.com/microsoft/retina/cmd/dns_notify_forwarding"
	"github.com/spf13/cobra"
	"go.etcd.io/etcd/version"
)

var (
	dnsHive = hive.New(dns_notify_forwarding.Agent)

	dnsNotifyForwarding = &cobra.Command{
		Use:   "dns-notify-forwarding",
		Short: "Forward DNS events to grpc connection",
		Run: func(cobraCmd *cobra.Command, _ []string) {
			if v, _ := cobraCmd.Flags().GetBool("version"); v {
				fmt.Printf("%s %s\n", cobraCmd.Name(), version.Version)
			}
			dns_notify_forwarding.Execute(cobraCmd, h)
		},
	}
)

func init() {
	dnsHive.RegisterFlags(dnsNotifyForwarding.Flags())
	dnsNotifyForwarding.AddCommand(dnsHive.Command())
	dns_notify_forwarding.InitGlobalFlags(dnsNotifyForwarding, h.Viper())
	rootCmd.AddCommand(dnsNotifyForwarding)
}
