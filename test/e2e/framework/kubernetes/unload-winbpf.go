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

	fmt.Println(output)
	if strings.Contains(output, "error") || strings.Contains(output, "failed") {
		return fmt.Errorf("error in UnLoading and pinning BPF maps and program: %s", output)
	}
	return nil
}

func (a *UnLoadAndPinWinBPF) Prevalidate() error {
	return nil
}

func (a *UnLoadAndPinWinBPF) Stop() error {
	return nil
}
