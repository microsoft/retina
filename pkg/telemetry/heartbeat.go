package telemetry

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/pkg/errors"
)

// HostInfoClient is a collection of functionality for retrieving
// information about the host.
type HostInfoClient struct{}

// KernelVersion returs the version of the kernel running on the host system.
func (p *HostInfoClient) KernelVersion(ctx context.Context) (string, error) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.CommandContext(ctx, "powershell", "-command", "$([Environment]::OSVersion).VersionString")
	case "linux":
		cmd = exec.CommandContext(ctx, "uname", "-r")
	default:
		return "", fmt.Errorf("unsupported platform %q", runtime.GOOS)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", errors.Wrapf(err, "failed to get %s kernel version: %s", runtime.GOOS, string(output))
	}
	return strings.TrimSuffix(string(output), "\n"), nil
}
