package debug

import (
	"github.com/microsoft/retina/cli/cmd/debug/bpf"
	"github.com/microsoft/retina/cli/cmd/debug/conntrack"
	"github.com/microsoft/retina/cli/cmd/debug/status"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "debug",
	Short: "Dataplane debug commands",
}

func init() {
	Cmd.AddCommand(conntrack.Cmd)
	Cmd.AddCommand(bpf.Cmd)
	Cmd.AddCommand(status.Cmd)
}
