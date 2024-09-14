package aws

import (
	"context"
	"fmt"
	"log"
	"path"
	"runtime"

	"github.com/aws/aws-sdk-go-v2/config"
)

type CreateCluster struct {
	AccountID          string
	Region             string
	ClusterName        string
	KubeConfigFilePath string
}

func (c *CreateCluster) Run() error {

	// Initialize AWS session
	_, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(c.Region),
	)
	if err != nil {
		return fmt.Errorf("unable to load SDK config, %v", err)
	}

	// Get the directory of the current test file.
	_, filename, _, ok := runtime.Caller(0)

	if !ok {
		return fmt.Errorf("failed to determine test file path")
	}

	currDir := path.Dir(filename)
	templateName := path.Join(currDir, "cluster-config.tpl")

	err = templateCluster(templateName, "cluster-config.yaml", c)

	if err != nil {
		return fmt.Errorf("unable to template cluster config, %v", err)
	}

	createArgs := []string{
		"create",
		"cluster",
		"--kubeconfig",
		c.KubeConfigFilePath,
		"-f",
		"cluster-config.yaml",
	}

	rootCmd := CreateEKSCtlCmd()
	rootCmd.SetArgs(createArgs)
	err = rootCmd.Execute()

	if err != nil {
		return fmt.Errorf("eksctl failed with %s", err)
	}

	log.Printf("Cluster created successfully!")
	return nil
}

func (d *CreateCluster) Prevalidate() error {
	return nil
}

func (d *CreateCluster) Stop() error {
	return nil
}
