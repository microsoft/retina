package bpf

import (
	"fmt"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/features"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// featuresCmd outputs available BPF features on the host
var featuresCmd = &cobra.Command{
	Use:   "features",
	Short: "Output available BPF features on the host",
	RunE: func(*cobra.Command, []string) error {
		linuxVersion, err := features.LinuxVersionCode()
		if err != nil {
			return errors.Wrap(err, "failed to get Linux version code")
		}
		fmt.Printf("Linux kernel version: %s\n", getLinuxKernelVersion(linuxVersion))

		fmt.Println("--------------------------------------------------")

		err = features.HaveBoundedLoops()
		if err != nil && !errors.Is(err, ebpf.ErrNotSupported) {
			return errors.Wrap(err, "failed to check for bounded loops")
		}
		fmt.Printf("Bounded loops: %s\n", isSupported(err))

		fmt.Println("--------------------------------------------------")

		err = features.HaveLargeInstructions()
		if err != nil && !errors.Is(err, ebpf.ErrNotSupported) {
			return errors.Wrap(err, "failed to check for large instructions")
		}
		fmt.Printf("Large instructions: %s\n", isSupported(err))

		fmt.Println("--------------------------------------------------")
		fmt.Println("eBPF map availability:")
		for _, mt := range eBPFMapList {
			err = features.HaveMapType(mt)
			if err != nil && !errors.Is(err, ebpf.ErrNotSupported) {
				return errors.Wrapf(err, "failed to check for map type %s", mt.String())
			}
			fmt.Printf("%s: %s\n", mt.String(), isSupported(err))
		}

		fmt.Println("--------------------------------------------------")
		fmt.Println("eBPF program types availability:")
		for _, pt := range eBPFProgramList {
			err = features.HaveProgramType(pt)
			if err != nil && !errors.Is(err, ebpf.ErrNotSupported) {
				return errors.Wrapf(err, "failed to check for program type %s", pt.String())
			}
			fmt.Printf("%s: %s\n", pt.String(), isSupported(err))
		}

		return nil
	},
}
