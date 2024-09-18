package bpf

import (
	"fmt"
	"net"
	"os"

	tc "github.com/florianl/go-tc"
	"github.com/florianl/go-tc/core"
	"github.com/mdlayher/netlink"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

// val is a struct to hold qdiscs and filters for an interface
type val struct {
	qdiscs         []tc.Object
	egressfilters  []tc.Object
	ingressfilters []tc.Object
}

var (
	ifaceName                  string
	ifaceToQdiscsAndFiltersMap = make(map[string]*val)
	qdiscCmd                   = &cobra.Command{
		Use:   "tc",
		Short: "Output all qdiscs and attached bpf programs on each interface on the host",
		RunE: func(*cobra.Command, []string) error {
			// open a rtnetlink socket
			rtnl, err := tc.Open(&tc.Config{})
			if err != nil {
				return errors.Wrap(err, "could not open rtnetlink socket")
			}
			defer func() {
				if err = rtnl.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "could not close rtnetlink socket: %v\n", err)
				}
			}()

			err = rtnl.SetOption(netlink.ExtendedAcknowledge, true)
			if err != nil {
				return errors.Wrap(err, "could not set NETLINK_EXT_ACK option")
			}

			qdiscs, err := rtnl.Qdisc().Get()
			if err != nil {
				return errors.Wrap(err, "could not get qdiscs")
			}

			for _, qdisc := range qdiscs {
				iface, err := net.InterfaceByIndex(int(qdisc.Ifindex))
				if err != nil {
					return errors.Wrap(err, "could not get interface by index")
				}
				if _, ok := ifaceToQdiscsAndFiltersMap[iface.Name]; !ok {
					ifaceToQdiscsAndFiltersMap[iface.Name] = &val{}
				}
				ifaceToQdiscsAndFiltersMap[iface.Name].qdiscs = append(ifaceToQdiscsAndFiltersMap[iface.Name].qdiscs, qdisc)

				ingressFilters, err := rtnl.Filter().Get(&tc.Msg{
					Family:  unix.AF_UNSPEC,
					Ifindex: uint32(iface.Index),
					Handle:  0,
					Parent:  core.BuildHandle(tc.HandleRoot, tc.HandleMinIngress),
					Info:    0x10300, // nolint:gomnd // info
				})
				if err != nil {
					return errors.Wrap(err, "could not get ingress filters for interface")
				}
				ifaceToQdiscsAndFiltersMap[iface.Name].ingressfilters = append(ifaceToQdiscsAndFiltersMap[iface.Name].ingressfilters, ingressFilters...)

				egressFilters, err := rtnl.Filter().Get(&tc.Msg{
					Family:  unix.AF_UNSPEC,
					Ifindex: uint32(iface.Index),
					Handle:  1,
					Parent:  core.BuildHandle(tc.HandleRoot, tc.HandleMinEgress),
					Info:    0x10300, // nolint:gomnd // info
				})
				if err != nil {
					return errors.Wrap(err, "could not get egress filters for interface")
				}
				ifaceToQdiscsAndFiltersMap[iface.Name].egressfilters = append(ifaceToQdiscsAndFiltersMap[iface.Name].egressfilters, egressFilters...)
			}

			if ifaceName != "" {
				if val, ok := ifaceToQdiscsAndFiltersMap[ifaceName]; ok {
					fmt.Printf("Interface: %s\n", ifaceName)
					fmt.Printf("Qdiscs:\n")
					for _, qdisc := range val.qdiscs {
						fmt.Printf("  %s\n", qdisc.Kind)
					}
					fmt.Printf("Ingress filters:\n")
					for _, ingressFilter := range val.ingressfilters {
						fmt.Printf("  %+v\n", ingressFilter)
					}
					fmt.Printf("Egress filters:\n")
					for _, egressFilter := range val.egressfilters {
						fmt.Printf("  %+v\n", egressFilter)
					}
				} else {
					fmt.Printf("Interface %s not found\n", ifaceName)
				}
			} else {
				for iface, val := range ifaceToQdiscsAndFiltersMap {
					fmt.Printf("Interface: %s\n", iface)
					fmt.Printf("Qdiscs:\n")
					for _, qdisc := range val.qdiscs {
						fmt.Printf("  %s\n", qdisc.Kind)
					}
					fmt.Printf("Ingress filters:\n")
					for _, ingressFilter := range val.ingressfilters {
						fmt.Printf("  %+v\n", ingressFilter)
					}
					fmt.Printf("Egress filters:\n")
					for _, egressFilter := range val.egressfilters {
						fmt.Printf("  %+v\n", egressFilter)
					}
				}
			}

			return nil
		},
	}
)

func init() {
	qdiscCmd.Flags().StringVarP(&ifaceName, "interface", "i", "", "Filter output to a specific interface")
}
