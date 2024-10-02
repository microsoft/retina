package perf

import (
	"fmt"
	"io"
	"os"

	"k8s.io/perf-tests/network/benchmarks/netperf/lib"
)

type GetNetworkPerformanceMeasures struct {
	ResultTag          string
	KubeConfigFilePath string
	JsonOutputFile     string
}

func (v *GetNetworkPerformanceMeasures) Prevalidate() error {
	return nil
}

func (v *GetNetworkPerformanceMeasures) Run() error {
	results, err := lib.PerformTests(lib.TestParams{
		Iterations:    1,
		Image:         "ghcr.io/azure/nptest:latest", // We would use the versioned image once we make a release in Azure/perf-tests
		TestNamespace: "netperf",
		TestFrom:      13,
		TestTo:        17,
		JsonOutput:    true,
		Tag:           v.ResultTag,
		KubeConfig:    v.KubeConfigFilePath,
	})
	if err != nil {
		return fmt.Errorf("failed to get network performance measures: %v", err)
	}
	sourceJsonOutputFile := results[0].JsonResultFile
	err = moveFile(sourceJsonOutputFile, v.JsonOutputFile)
	if err != nil {
		return fmt.Errorf("failed to copy json output file: %v", err)
	}
	return nil
}

func (v *GetNetworkPerformanceMeasures) Stop() error {
	return nil
}

func moveFile(src, dst string) error {
	err := copyFile(src, dst)
	if err != nil {
		return fmt.Errorf("failed to move file: %v", err)
	}

	err = os.Remove(src)
	if err != nil {
		fmt.Printf("warning: failed to delete source file %s: %v\n", src, err)
	}

	return nil
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %v", err)
	}
	defer sourceFile.Close()

	// Create the destination file
	destinationFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %v", err)
	}
	defer destinationFile.Close()

	// Copy the contents from source to destination
	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %v", err)
	}

	// Flush the destination file to ensure all data is written
	err = destinationFile.Sync()
	if err != nil {
		return fmt.Errorf("failed to sync destination file: %v", err)
	}

	return nil
}
