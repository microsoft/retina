package bpf

import "github.com/spf13/cobra"

var Cmd = &cobra.Command{
	Use:   "bpf",
	Short: "BPF debug commands (Not supported on Windows)",
}
