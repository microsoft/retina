package kubernetes

import (
	"fmt"
	"strings"
)

type UnLoadAndPinWinBPF struct {
	KubeConfigFilePath                   string
	UnLoadAndPinWinBPFDeamonSetNamespace string
	UnLoadAndPinWinBPFDeamonSetName      string
}

func (a *UnLoadAndPinWinBPF) Run() error {
	UnLoadAndPinWinBPFDLabelSelector := "name=" + a.UnLoadAndPinWinBPFDeamonSetName
	nodeNames, err := runningWindowsPodNodeNames(a.KubeConfigFilePath, a.UnLoadAndPinWinBPFDeamonSetNamespace, UnLoadAndPinWinBPFDLabelSelector)
	if err != nil {
		return err
	}

	for _, nodeName := range nodeNames {
		output, execErr := ExecCommandInWinPod(a.KubeConfigFilePath, "C:\\event-writer-helper.bat EventWriter-UnPinPrgAndMaps", a.UnLoadAndPinWinBPFDeamonSetNamespace, UnLoadAndPinWinBPFDLabelSelector, nodeName, false)
		if execErr != nil {
			return execErr
		}

		// Failure to unpin the maps and program is not a failure of the test, so we just log it
		// and continue.
		// This is because the test may have already unpinned them during a retry
		fmt.Println(output)
		if strings.Contains(output, "error") || strings.Contains(output, "failed") {
			fmt.Printf("error in UnLoading and pinning BPF maps and program on node %s: %s", nodeName, output)
		}
	}

	return nil
}

func (a *UnLoadAndPinWinBPF) Prevalidate() error {
	return nil
}

func (a *UnLoadAndPinWinBPF) Stop() error {
	return nil
}
