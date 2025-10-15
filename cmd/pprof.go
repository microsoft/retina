package cmd

import (
	"github.com/microsoft/retina/cmd/pprof"
	"github.com/spf13/cobra"
)

var pprofDumpCmd = &cobra.Command{
	Use:   "pprof-dump",
	Short: "Dump pprof profiles from retina-agent pods",
	Long:  "Dump pprof profiles from retina-agent pods",
	Run: func(cmd *cobra.Command, args []string) {
		duration, _ := cmd.Flags().GetInt("duration")
		pprof.DownloadAll(duration)
	},
}

func init() {
	pprofDumpCmd.Flags().Int("duration", 30, "Duration in seconds for trace and CPU profile collection")

	rootCmd.AddCommand(pprofDumpCmd)
}
