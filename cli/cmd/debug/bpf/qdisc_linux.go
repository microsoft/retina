package bpf

import (
	"fmt"
	"net"
	"os"

	"github.com/florianl/go-tc"
	"github.com/mdlayher/netlink"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	ifaceName string
	qdiscCmd  = &cobra.Command{
		Use:   "qdisc",
		Short: "Output all qdiscs on the interfaces",
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
					fmt.Fprintf(os.Stderr, "could not get interface for index %d: %v\n", qdisc.Ifindex, err)
					continue
				}
				if ifaceName != "" && iface.Name != ifaceName {
					fmt.Printf("%20s\t%s\n", iface.Name, qdisc.Kind)
				} else {
					fmt.Printf("%20s\t%s\n", iface.Name, qdisc.Kind)
				}
			}
			return nil
		},
	}
)

func init() {
	qdiscCmd.Flags().StringVarP(&ifaceName, "interface", "i", "", "Interface name to filter qdiscs")
}
