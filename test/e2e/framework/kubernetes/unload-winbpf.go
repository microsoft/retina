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
	UnLoadAndPinWinBPFDLabelSelector := fmt.Sprintf("name=%s", a.UnLoadAndPinWinBPFDeamonSetName)
	output, err := ExecCommandInWinPod(a.KubeConfigFilePath, "C:\\event-writer-helper.bat EventWriter-UnPinPrgAndMaps", a.UnLoadAndPinWinBPFDeamonSetNamespace, UnLoadAndPinWinBPFDLabelSelector, true)
	if err != nil {
		return err
	}

	// Failure to unpin the maps and program is not a failure of the test, so we just log it
	// and continue.
	// This is because the test may have already unpinned them during a retry
	fmt.Println(output)
	if strings.Contains(output, "error") || strings.Contains(output, "failed") {
		fmt.Printf("error in UnLoading and pinning BPF maps and program: %s", output)
	}
	return nil
}

func (a *UnLoadAndPinWinBPF) Prevalidate() error {
	return nil
}

func (a *UnLoadAndPinWinBPF) Stop() error {
	return nil
}
