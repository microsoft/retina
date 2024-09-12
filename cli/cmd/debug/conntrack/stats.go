package conntrack

import (
	"github.com/microsoft/retina/pkg/plugin/conntrack"
	"github.com/spf13/cobra"
)

var stats = &cobra.Command{
	Use:   "stats",
	Short: "Print conntrack stats",
	RunE: func(*cobra.Command, []string) error {
		return conntrack.Stats()
	},
}
