package main

import (
	"github.com/microsoft/retina/shell/cmd/pprof"
	"github.com/spf13/cobra"
)

var sysdump = &cobra.Command{
	Use:   "sysdump",
	Short: "Dump pprof profiles from retina-agent pods",
	Long:  "Dump pprof profiles from retina-agent pods",
	Run: func(cmd *cobra.Command, args []string) {
		duration, _ := cmd.Flags().GetInt("duration")
		pprof.DownloadAll(duration)
	},
}

func init() {
	sysdump.Flags().Int("duration", 30, "Duration in seconds for trace and CPU profile collection")

	rootCmd.AddCommand(sysdump)
}
