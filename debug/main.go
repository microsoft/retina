package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/microsoft/retina/internal/buildinfo"
	"github.com/microsoft/retina/pkg/log"
	plugincommon "github.com/microsoft/retina/pkg/plugin/common"
	"go.uber.org/zap"
)

const (
	bpftoolCmd = "bpftool"
	ipCmd      = "ip"
)

type Debugger struct {
	l *zap.Logger
}

func (d *Debugger) run() {
	// List all the veth interfaces.
	d.runCmd(ipCmd, "link", "show", "type", "veth")

	// List all the BPF programs.
	d.runCmd(bpftoolCmd, "prog", "list")

	// List all the BPF maps.
	d.runCmd(bpftoolCmd, "map", "list")

	// Dump the BPF conntrack map pinned at /sys/fs/bpf/retina_conntrack_map.
	p := fmt.Sprintf("%s/%s", plugincommon.MapPath, plugincommon.ConntrackMapName)
	d.runCmd(bpftoolCmd, "map", "dump", "pinned", p)
}

func (d *Debugger) runCmd(cmd string, args ...string) error {
	d.l.Info("Running command", zap.String("cmd", cmd), zap.Strings("args", args))
	defer d.l.Info("-------------------------------------------------------------")

	c := exec.Command(cmd, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	err := c.Run()
	if err != nil {
		d.l.Error("Failed to run command", zap.String("cmd", cmd), zap.Strings("args", args), zap.Error(err))
		return err
	}
	return nil
}

func main() {
	opts := log.GetDefaultLogOpts()
	zl, err := log.SetupZapLogger(opts)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	l := zl.Named("debug-retina-dataplane").With(zap.String("version", buildinfo.Version))
	dbg := Debugger{l: l}
	dbg.run()
}
