package perf

import (
	"io"
	"os"

	"github.com/pkg/errors"
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
		Image:         "ghcr.io/azure/nptest:v20241002",
		TestNamespace: "netperf",
		TestFrom:      13,
		TestTo:        17,
		JsonOutput:    true,
		Tag:           v.ResultTag,
		KubeConfig:    v.KubeConfigFilePath,
	})
	if err != nil {
		return errors.Wrap(err, "failed to get network performance measures")
	}
	sourceJsonOutputFile := results[0].JsonResultFile
	err = moveFile(sourceJsonOutputFile, v.JsonOutputFile)
	if err != nil {
		return errors.Wrap(err, "failed to move json output file")
	}
	return nil
}

func (v *GetNetworkPerformanceMeasures) Stop() error {
	return nil
}

func moveFile(src, dst string) error {
	err := copyFile(src, dst)
	if err != nil {
		return errors.Wrap(err, "failed to copy file")
	}

	err = os.Remove(src)
	if err != nil {
		return errors.Wrap(err, "failed to remove source file")
	}

	return nil
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return errors.Wrap(err, "failed to open source file")
	}
	defer sourceFile.Close()

	// Create the destination file
	destinationFile, err := os.Create(dst)
	if err != nil {
		return errors.Wrap(err, "failed to create destination file")
	}
	defer destinationFile.Close()

	// Copy the contents from source to destination
	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return errors.Wrap(err, "failed to copy contents from source to destination")
	}

	// Flush the destination file to ensure all data is written
	err = destinationFile.Sync()
	if err != nil {
		return errors.Wrap(err, "failed to flush destination file")
	}

	return nil
}
