package dropreason

import (
	"fmt"
	"runtime"

	"github.com/blang/semver/v4"
	"github.com/cilium/cilium/pkg/version"
	"github.com/cilium/cilium/pkg/versioncheck"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	plugincommon "github.com/microsoft/retina/pkg/plugin/common"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	MinAmdVersionNum = "5.5"
	MinArmVersionNum = "6.0"
)

/*
getEbpfPayload() returns the ebpf program and map objects to load, based on the kernel version, architecture, and distro.
We use fexit programs for better performance, and fall back to kprobes.

eBPF Program Support Matrix
===========================

Program Type Selection:
- If:
  - Arch == amd64 and kernel >= 5.5
  - Arch == arm64 and kernel >= 6.0
    → Use `fexit`
  - Else:
    → Use `kprobe`

Scope Selection:
- If:
  - Distro == Mariner
    → Scope = core (only core kernel funcs)
  - Else:
    → Scope = all (core + module funcs)

+-----------+------------------------+--------------+--------+
| Distro    | Arch + Kernel          | Prog         | Scope  |
+-----------+------------------------+--------------+--------+
| Mariner   | amd64, kernel >= 5.5   | fexit        | core   |
| Mariner   | arm64, kernel >= 6.0   | fexit        | core   |
| Non-Marin | amd64, kernel >= 5.5   | fexit        | all    |
| Non-Marin | arm64, kernel >= 6.0   | fexit        | all    |
| *         | (otherwise)            | kprobe       | per OS |
+-----------+------------------------+--------------+--------+

core kernel funcs:
- tcp_v4_connect
- inet_csk_accept
- nf_hook_slow

module funcs:
- nf_conntrack_confirm
- nf_nat_inet_fn
*/

func (dr *dropReason) getEbpfPayload() (objs interface{}, maps *kprobeMaps, supportsFexit bool, err error) {
	isMariner := plugincommon.IsAzureLinux()
	dr.l.Info("Distro check:", zap.Bool("isMariner", isMariner))

	kv, err := version.GetKernelVersion()
	if err != nil {
		kv, err = plugincommon.GetKernelVersionMajMin()
		if err != nil {
			return nil, nil, false, fmt.Errorf("failed to get kernel version: %w", err) //nolint:goerr113 //wrapping error from external module
		}
	}
	dr.l.Info("Detected kernel", zap.String("version", kv.String()))

	objs, maps, supportsFexit = resolvePayload(runtime.GOARCH, kv, isMariner)
	return objs, maps, supportsFexit, nil
}

func resolvePayload(arch string, kv semver.Version, isMariner bool) (interface{}, *kprobeMaps, bool) {
	minVersionAmd64, _ := versioncheck.Version(MinAmdVersionNum)
	minVersionArm64, _ := versioncheck.Version(MinArmVersionNum)

	supportsFexit := (arch == "amd64" && kv.GTE(minVersionAmd64)) ||
		(arch == "arm64" && kv.GTE(minVersionArm64))

	var objs interface{}
	var maps *kprobeMaps

	switch {
	case isMariner && supportsFexit: // Mariner supports a subset of the fexit programs, need to check for it first.
		objs = &marinerObjects{} //nolint:typecheck // needs to match a generated struct until we fix Mariner
		maps = &objs.(*marinerObjects).kprobeMaps
	case supportsFexit:
		objs = &allFexitObjects{} //nolint:typecheck // this is a generated struct
		maps = &objs.(*allFexitObjects).kprobeMaps
	default:
		objs = &allKprobeObjects{} //nolint:typecheck // this is a generated struct
		maps = &objs.(*allKprobeObjects).kprobeMaps
	}

	return objs, maps, supportsFexit
}

func (dr *dropReason) attachKprobes(kprobes, kprobesRet map[string]*ebpf.Program) error {
	for name := range kprobes {
		progLink, err := link.Kprobe(name, kprobes[name], nil)
		if err != nil {
			dr.l.Error("Failed to attach kprobe", zap.String("program", name), zap.Error(err))
		} else {
			dr.hooks = append(dr.hooks, progLink)
			dr.l.Info("Attached kprobe", zap.String("program", name))
		}
	}

	// The kretprobes set metric values. If none were attached, report an error.
	retprobeCount := 0
	for name := range kprobesRet {
		progLink, err := link.Kretprobe(name, kprobesRet[name], nil)
		if err != nil {
			dr.l.Error("Failed to attach kretprobe", zap.String("program", name), zap.Error(err))
		} else {
			dr.hooks = append(dr.hooks, progLink)
			retprobeCount++
			dr.l.Info("Attached kretprobe", zap.String("program", name))
		}
	}
	if retprobeCount == 0 {
		dr.l.Error("No kretprobes attached, cannot collect drop metrics")
		return errors.New("No kretprobes attached, cannot collect drop metrics") //nolint:goerr113 // no sentinel type used
	}

	return nil
}

func (dr *dropReason) attachFexitPrograms(objs map[string]*ebpf.Program) error {
	progCount := 0
	for name, prog := range objs {
		progLink, err := link.AttachTracing(link.TracingOptions{Program: prog, AttachType: ebpf.AttachTraceFExit})
		if err != nil {
			dr.l.Error("Failed to attach", zap.String("program", name), zap.Error(err))
		} else {
			dr.hooks = append(dr.hooks, progLink)
			progCount++
			dr.l.Info("Attached program", zap.String("program", name))
		}
	}

	if progCount == 0 {
		dr.l.Error("No programs attached, cannot collect drop metrics")
		return errors.New("No programs attached, cannot collect drop metrics") //nolint:goerr113 // no sentinel type used
	}

	return nil
}

func buildKprobePrograms(objs any) (progsKprobe, progsKprobeRet map[string]*ebpf.Program) {
	progsKprobe = make(map[string]*ebpf.Program)
	progsKprobeRet = make(map[string]*ebpf.Program)

	if o, ok := objs.(*allKprobeObjects); ok {
		progsKprobe[inetCskAcceptFn] = o.InetCskAccept
		progsKprobe[nfHookSlowFn] = o.NfHookSlow
		progsKprobe[nfNatInetFn] = o.NfNatInetFn
		progsKprobe[nfConntrackConfirmFn] = o.NfConntrackConfirm

		progsKprobeRet[nfHookSlowFn] = o.NfHookSlowRet
		progsKprobeRet[inetCskAcceptFn] = o.InetCskAcceptRet
		progsKprobeRet[tcpConnectFn] = o.TcpV4ConnectRet
		progsKprobeRet[nfNatInetFn] = o.NfNatInetFnRet
		progsKprobeRet[nfConntrackConfirmFn] = o.NfConntrackConfirmRet
	}
	return progsKprobe, progsKprobeRet
}

func buildFexitPrograms(objs any) map[string]*ebpf.Program {
	progsFexit := make(map[string]*ebpf.Program)

	switch o := objs.(type) {
	case *allFexitObjects:
		progsFexit[inetCskAcceptFnFexit] = o.InetCskAcceptFexit
		progsFexit[nfHookSlowFnFexit] = o.NfHookSlowFexit
		progsFexit[tcpV4ConnectFexit] = o.TcpV4ConnectFexit
		progsFexit[nfNatInetFnFexit] = o.NfNatInetFnFexit
		progsFexit[nfConntrackConfirmFnFexit] = o.NfConntrackConfirmFexit

	case *marinerObjects:
		progsFexit[inetCskAcceptFnFexit] = o.InetCskAcceptFexit
		progsFexit[nfHookSlowFnFexit] = o.NfHookSlowFexit
		progsFexit[tcpV4ConnectFexit] = o.TcpV4ConnectFexit
	}
	return progsFexit
}
