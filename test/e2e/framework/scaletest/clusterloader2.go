package scaletest

import (
	"fmt"
	"os"
	"os/exec"
)

type ClusterLoader2 struct{}

func (d *ClusterLoader2) Prevalidate() error {
	return nil
}

func (d *ClusterLoader2) Run() error {
	args := []string{
		"--testconfig=../../perf-tests/clusterloader2/test/config.yaml",
		"--provider=aks",
		"--kubeconfig=/home/runner/.kube/config",
		"--v=2",
		"--report-dir=../../perf-tests/clusterloader2/report",
	}
	cl2Path := "../../perf-tests/clusterloader2/clusterloader"
	cmd := exec.Command(cl2Path, args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("Error executing CL2: %w", err)
	}

	return nil
}

func (d *ClusterLoader2) Stop() error {
	return nil
}
