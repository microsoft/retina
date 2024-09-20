package conntrack

import (
	"github.com/microsoft/retina/pkg/plugin/conntrack"
	"github.com/spf13/cobra"
)

var dump = &cobra.Command{
	Use:   "dump",
	Short: "Dump all conntrack entries",
	RunE: func(*cobra.Command, []string) error {
		return conntrack.Dump()
	},
}
