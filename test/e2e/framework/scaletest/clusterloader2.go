package scaletest

import (
	"os"
	"os/exec"
)

type ClusterLoader2 struct {
	Args []string
}

func (d *ClusterLoader2) Prevalidate() error {
	return nil
}

func (d *ClusterLoader2) Run() error {
	cl2Path := "../../perf-tests/clusterloader2/clusterloader"
	cmd := exec.Command(cl2Path, d.Args...)
	// cmd := exec.Command("ls", "-l", "../../perf-tests/clusterloader2")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// output, err := cmd.Output()
	// log.Println("CL2 Output: ", string(output))
	return cmd.Run()
	// return err
}

func (d *ClusterLoader2) Stop() error {
	return nil
}
