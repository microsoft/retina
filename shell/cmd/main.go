package main

import (
	"fmt"
	"os"

	"github.com/microsoft/retina/shell/cmd/pprof"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "retina",
	Short: "Retina debugging tools",
	Long:  "Retina debugging and diagnostic tools for troubleshooting",
}

var sysdumpCmd = &cobra.Command{
	Use:   "sysdump",
	Short: "Dump pprof profiles from retina-agent",
	Long:  "Dump pprof profiles from retina-agent and package them into a tarball",
	Run: func(cmd *cobra.Command, args []string) {
		duration, _ := cmd.Flags().GetInt("duration")
		if err := pprof.DownloadAll(duration); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	sysdumpCmd.Flags().Int("duration", 30, "Duration in seconds for trace and CPU profile collection")
	rootCmd.AddCommand(sysdumpCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
