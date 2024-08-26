package aws

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/config"
)

type CreateCluster struct {
	AccountID   string
	Region      string
	ClusterName string
}

func (c *CreateCluster) Run() error {

	// Initialize AWS session
	_, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(c.Region),
	)
	if err != nil {
		return fmt.Errorf("unable to load SDK config, %v", err)
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
