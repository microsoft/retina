package bpf

import (
	"fmt"
	"os"

	"github.com/cilium/cilium/pkg/mountinfo"
	plugincommon "github.com/microsoft/retina/pkg/plugin/common"
	"golang.org/x/sys/unix"
)

func __mount() error {
	// Check if the path exists.
	_, err := os.Stat(plugincommon.MapPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Path does not exist. Create it.
			err = os.MkdirAll(plugincommon.MapPath, 0o755)
			if err != nil {
				return err
			}
		} else {
			// Some other error. Return.
			return err
		}
	}
	err = unix.Mount(plugincommon.MapPath, plugincommon.MapPath, "bpf", 0, "")
	return err
}

func MountRetinaBpfFS() error {
	// Check if /sys/fs/bpf is already mounted.
	mounted, bpfMount, err := mountinfo.IsMountFS(mountinfo.FilesystemTypeBPFFS, plugincommon.MapPath)
	if err != nil {
		return err
	}
	if !mounted {
		if err := __mount(); err != nil {
			return err
		}
		return nil
	}
	// Else mounted. Check the type of mount.
	if !bpfMount {
		// Custom mount of /sys/fs/bpf. Unknown setup. Exit.
		return fmt.Errorf("%+s is already mounted but not as bpf. Not supported", plugincommon.MapPath)
	}
	return nil
}
