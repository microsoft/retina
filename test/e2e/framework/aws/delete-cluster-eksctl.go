package aws

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/config"
)

type DeleteCluster struct {
	AccountID   string
	Region      string
	ClusterName string
}

func (d *DeleteCluster) Run() error {

	// Initialize AWS session
	_, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(d.Region),
	)
	if err != nil {
		return fmt.Errorf("unable to load SDK config, %v", err)
	}

	deleteArgs := []string{
		"delete",
		"cluster",
		"-n",
		d.ClusterName,
		"--region",
		d.Region,
	}

	rootCmd := CreateEKSCtlCmd()
	rootCmd.SetArgs(deleteArgs)
	err = rootCmd.Execute()

	if err != nil {
		return fmt.Errorf("eksctl failed with %s", err)
	}

	log.Printf("Cluster deleted successfully!")
	return nil
}

func (d *DeleteCluster) Prevalidate() error {
	return nil
}

func (d *DeleteCluster) Stop() error {
	return nil
}
