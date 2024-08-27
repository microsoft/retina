package aws

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"

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

	// Exec this
	// eksctl create cluster --kubeconfig test.pem -n cluster_name
	cmd := exec.Command(
		"eksctl",
		"create",
		"cluster",
		"--kubeconfig",
		c.KubeConfigFilePath,
		"-n",
		c.ClusterName,
		"--region",
		c.Region,
		"--managed",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
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
