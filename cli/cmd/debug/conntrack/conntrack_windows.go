package conntrack

import "github.com/spf13/cobra"

var Cmd = &cobra.Command{
	Use:   "conntrack",
	Short: "Conntrack debug commands (Not supported on Windows)",
}
