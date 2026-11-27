package kubernetes

import (
	"os"
	"os/exec"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/yaml"
)

func TestWindowsInstallerCommandExitsOnFailure(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("requires native PowerShell on a Windows runner")
	}
	data, err := os.ReadFile("../../yaml/windows/install-ebpf-xdp.yaml")
	require.NoError(t, err)
	var daemonSet appsv1.DaemonSet
	require.NoError(t, yaml.Unmarshal(data, &daemonSet))
	command := daemonSet.Spec.Template.Spec.Containers[0].Command
	require.Equal(t, "powershell.exe", command[0])
	for _, tc := range []struct {
		name, script string
		wantErr      bool
	}{
		{name: "failed installer must exit", script: "throw 'simulated download failure'", wantErr: true},
		{
			name:    "nonterminating installer error must exit",
			script:  `function Install-EbpfXdp { [pscustomobject]@{CiliumOnWindows=1}; Write-Error 'simulated download failure'; return $false }; if (-not (Install-EbpfXdp)) { throw 'installer failed' }`,
			wantErr: true,
		},
		{name: "successful installer enters keep-alive", script: "Write-Output 'installer completed'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(dir+"/install-ebpf-xdp.ps1", []byte(tc.script), 0o600))
			//nolint:gosec // Execute the repository-owned installer command under test.
			cmd := exec.CommandContext(t.Context(), "powershell.exe", "-NoProfile", command[1],
				// The marker is joined at runtime because PowerShell echoes the command text in error output.
				"function Start-Sleep { Write-Output ('KEEP_ALIVE_'+'REACHED'); exit 0 }; "+command[2])
			cmd.Dir = dir
			output, runErr := cmd.CombinedOutput()
			if tc.wantErr {
				require.Error(t, runErr)
				var exitErr *exec.ExitError
				require.ErrorAs(t, runErr, &exitErr)
				require.Equal(t, 1, exitErr.ExitCode())
				require.NotContains(t, string(output), "KEEP_ALIVE_REACHED")
			} else {
				require.NoError(t, runErr, "%s", output)
				require.Contains(t, string(output), "KEEP_ALIVE_REACHED")
			}
		})
	}
}
