package bpf

import (
	"fmt"

	"github.com/cilium/ebpf"
	"github.com/pkg/errors"
)

var (
	eBPFMapList = []ebpf.MapType{
		ebpf.Hash,
		ebpf.Array,
		ebpf.ProgramArray,
		ebpf.PerfEventArray,
		ebpf.PerCPUHash,
		ebpf.PerCPUArray,
		ebpf.StackTrace,
		ebpf.CGroupArray,
		ebpf.LRUHash,
		ebpf.LRUCPUHash,
		ebpf.LPMTrie,
		ebpf.ArrayOfMaps,
		ebpf.HashOfMaps,
		ebpf.DevMap,
		ebpf.SockMap,
		ebpf.CPUMap,
		ebpf.XSKMap,
		ebpf.SockHash,
		ebpf.CGroupStorage,
		ebpf.ReusePortSockArray,
		ebpf.PerCPUCGroupStorage,
		ebpf.Queue,
		ebpf.Stack,
		ebpf.SkStorage,
		ebpf.DevMapHash,
		ebpf.StructOpsMap,
		ebpf.RingBuf,
		ebpf.InodeStorage,
		ebpf.TaskStorage,
	}

	eBPFProgramList = []ebpf.ProgramType{
		ebpf.SocketFilter,
		ebpf.Kprobe,
		ebpf.SchedCLS,
		ebpf.SchedACT,
		ebpf.TracePoint,
		ebpf.XDP,
		ebpf.PerfEvent,
		ebpf.CGroupSKB,
		ebpf.CGroupSock,
		ebpf.LWTIn,
		ebpf.LWTOut,
		ebpf.LWTXmit,
		ebpf.SockOps,
		ebpf.SkSKB,
		ebpf.CGroupDevice,
		ebpf.SkMsg,
		ebpf.RawTracepoint,
		ebpf.CGroupSockAddr,
		ebpf.LWTSeg6Local,
		ebpf.LircMode2,
		ebpf.SkReuseport,
		ebpf.FlowDissector,
		ebpf.CGroupSysctl,
		ebpf.RawTracepointWritable,
		ebpf.CGroupSockopt,
		ebpf.Tracing,
		ebpf.StructOps,
		ebpf.Extension,
		ebpf.LSM,
		ebpf.SkLookup,
		ebpf.Syscall,
		ebpf.Netfilter,
	}
)

func isSupported(err error) string {
	if errors.Is(err, ebpf.ErrNotSupported) {
		return "not supported"
	}
	return "supported"
}

func getLinuxKernelVersion(versionCode uint32) string {
	return fmt.Sprintf("%d.%d.%d", versionCode>>16, (versionCode>>8)&0xff, versionCode&0xff) //nolint:gomnd // bit shifting
}