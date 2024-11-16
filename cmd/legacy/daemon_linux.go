package legacy

import "github.com/cilium/ebpf/rlimit"

func (d *Daemon) RemoveMemlock() error {
	return rlimit.RemoveMemlock()
}
