package bpf

import (
	"fmt"
	"net"
	"os"

	tc "github.com/florianl/go-tc"
	"github.com/mdlayher/netlink"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	ifaceName string
	qdiscCmd  = &cobra.Command{
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
					fmt.Fprintf(os.Stderr, "could not get interface for index %d: %v\n", qdisc.Ifindex, err)
					continue
				}
				if ifaceName != "" {
					// Output only qdiscs and attached bpf programs for the specified interface
					if iface.Name == ifaceName {
						qdiscKind := qdisc.Kind
						fmt.Printf("Interface: %s\n", iface.Name)
						fmt.Printf("|->Qdisc: %s\n", qdiscKind)
						if qdisc.BPF != nil {
							fmt.Printf("   |->Attached bpf programs:\n")
							fmt.Printf("	  |->Name: %s\n", *qdisc.BPF.Name)
						}
					} else {
						continue
					}
				} else {
					// Output all qdiscs and attached bpf programs
					qdiscKind := qdisc.Kind
					fmt.Printf("Interface: %s\n", iface.Name)
					fmt.Printf("|->Qdisc: %s\n", qdiscKind)
					if qdisc.BPF != nil {
						fmt.Printf("   |->Attached bpf programs:\n")
						fmt.Printf("	  |->Name: %s\n", *qdisc.BPF.Name)
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
