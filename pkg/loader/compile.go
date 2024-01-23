package loader

import (
	"bytes"
	"context"
	"os/exec"

	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
)

var compiler string = "clang"

func runCmd(cmd *exec.Cmd) error {
	l := log.Logger().Named(string("run-command"))

	var out, stderr bytes.Buffer

	cmd.Stdout = &out
	cmd.Stderr = &stderr

	l.Debug("Running", zap.String("command", cmd.String()))

	err := cmd.Run()
	if err != nil {
		l.Debug("Error running command", zap.String("command", cmd.String()), zap.String("stderr", stderr.String()), zap.Error(err))
		return err
	}
	l.Debug("Output running command", zap.String("command", cmd.String()), zap.String("stdout", out.String()))
	return nil
}

func CompileEbpf(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, compiler, args...)
	err := runCmd(cmd)
	if err != nil {
		return err
	}
	return nil
}
