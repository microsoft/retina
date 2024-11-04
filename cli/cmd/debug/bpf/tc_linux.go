package bpf

import (
	"fmt"
	"net"
	"os"

	tc "github.com/florianl/go-tc"
	"github.com/florianl/go-tc/core"
	"github.com/mdlayher/netlink"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

// val is a struct to hold qdiscs and filters for an interface
type val struct {
	qdisc              []tc.Object
	egressFilterExist  bool
	ingressFilterExist bool
}

var (
	ifaceName                  string
	ifaceToQdiscsAndFiltersMap = make(map[string]any)
	qdiscCmd                   = &cobra.Command{
		Use:   "tc",
		Short: "Output all qdiscs and attached bpf programs on each interface on the host",
		Run: func(*cobra.Command, []string) {
			logger := pterm.DefaultLogger.WithLevel(pterm.LogLevelTrace)
			// open a rtnetlink socket
			rtnl, err := tc.Open(&tc.Config{})
			if err != nil {
				logger.Error("could not open rtnetlink socket", logger.Args(err))
				return
			}
			defer func() {
				if err = rtnl.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "could not close rtnetlink socket: %v\n", err)
				}
			}()

			// set NETLINK_EXT_ACK option for detailed error messages
			err = rtnl.SetOption(netlink.ExtendedAcknowledge, true)
			if err != nil {
				logger.Warn("could not set NETLINK_EXT_ACK option", logger.Args(err))
			}

			// get all qdiscs
			qdiscs, err := rtnl.Qdisc().Get()
			if err != nil {
				logger.Error("could not get qdiscs", logger.Args(err))
				return
			}

			// populate ifaceToQdiscsAndFiltersMap
			for _, qdisc := range qdiscs {
				iface, err := net.InterfaceByIndex(int(qdisc.Ifindex))
				if err != nil {
					logger.Error("could not get interface by index", logger.Args(err, qdisc.Ifindex))
					continue
				}
				if _, ok := ifaceToQdiscsAndFiltersMap[iface.Name]; !ok {
					ifaceToQdiscsAndFiltersMap[iface.Name] = &val{}
				}
				v := ifaceToQdiscsAndFiltersMap[iface.Name].(*val)
				v.qdisc = append(v.qdisc, qdisc)

				ingressFilters, err := rtnl.Filter().Get(&tc.Msg{
					Family:  unix.AF_UNSPEC,
					Ifindex: uint32(iface.Index),
					Handle:  0,
					Parent:  core.BuildHandle(tc.HandleRoot, tc.HandleMinIngress),
					Info:    0x10300, // nolint:gomnd // info
				})
				if err != nil {
					logger.Error("could not get ingress filters for interface", logger.Args(err))
					continue
				}
				v.ingressFilterExist = len(ingressFilters) > 0

				egressFilters, err := rtnl.Filter().Get(&tc.Msg{
					Family:  unix.AF_UNSPEC,
					Ifindex: uint32(iface.Index),
					Handle:  1,
					Parent:  core.BuildHandle(tc.HandleRoot, tc.HandleMinEgress),
					Info:    0x10300, // nolint:gomnd // info
				})
				if err != nil {
					logger.Error("could not get egress filters for interface", logger.Args(err))
					continue
				}
				v.egressFilterExist = len(egressFilters) > 0
			}

			if ifaceName != "" {
				if value, ok := ifaceToQdiscsAndFiltersMap[ifaceName]; ok {
					v := value.(*val)
					outputMap := make(map[string]any)
					outputMap["name"] = ifaceName
					outputMap["qdiscs"] = v.qdisc
					outputMap["ingressFilterExist"] = v.ingressFilterExist
					outputMap["egressFilterExist"] = v.egressFilterExist
					logger.Info("Interface", logger.ArgsFromMap(outputMap))
				} else {
					logger.Error("Interface not found", logger.Args(ifaceName))
					return
				}
			} else {
				logger.Info("Interfaces", logger.ArgsFromMap(ifaceToQdiscsAndFiltersMap))
			}
		},
	}
)

func init() {
	qdiscCmd.Flags().StringVarP(&ifaceName, "interface", "i", "", "Filter output to a specific interface")
}
