package kubernetes

import "fmt"

const windowsInstallerName = "install-ebpf-xdp"

// SetWindowsMetricMode selects the Windows metrics source on every Windows node:
// HNS (CiliumOnWindows=0) or eBPF (CiliumOnWindows=1). The hnsstats plugin only collects
// when the value is not 1, so each phase must set the mode before the agent starts.
type SetWindowsMetricMode struct {
	KubeConfigFilePath string
	EnableEBPF         bool
}

func (s *SetWindowsMetricMode) Prevalidate() error { return nil }
func (s *SetWindowsMetricMode) Stop() error        { return nil }

func (s *SetWindowsMetricMode) Run() error {
	value := "0"
	if s.EnableEBPF {
		value = "1"
	}
	selector := "name=" + windowsInstallerName
	nodeNames, err := runningWindowsPodNodeNames(s.KubeConfigFilePath, windowsInstallerName, selector)
	if err != nil {
		return err
	}
	for _, nodeName := range nodeNames {
		command := `reg add HKLM\SYSTEM\CurrentControlSet\Services\hns\State /v CiliumOnWindows /t REG_DWORD /d ` + value + ` /f`
		if _, err := ExecCommandInWinPod(s.KubeConfigFilePath, command, windowsInstallerName, selector, nodeName, true); err != nil {
			return fmt.Errorf("setting CiliumOnWindows=%s on %s: %w", value, nodeName, err)
		}
	}
	return nil
}
