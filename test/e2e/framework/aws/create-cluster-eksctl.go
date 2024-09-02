package aws

import (
	"bytes"
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/kris-nova/logger"
	"github.com/spf13/cobra"
	"github.com/weaveworks/eksctl/pkg/ctl/cmdutils"
	"github.com/weaveworks/eksctl/pkg/ctl/create"
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

	// Create the ekctl root cmd to execute
	rootCmd := &cobra.Command{
		Use:   "eksctl [command]",
		Short: "The official CLI for Amazon EKS",
		Run: func(c *cobra.Command, _ []string) {
			if err := c.Help(); err != nil {
				logger.Debug("ignoring cobra error %q", err.Error())
			}
		},
		SilenceUsage: true,
	}

	loggerLevel := rootCmd.PersistentFlags().IntP("verbose", "v", 3, "set log level, use 0 to silence, 4 for debugging and 5 for debugging with AWS debug logging")
	colorValue := rootCmd.PersistentFlags().StringP("color", "C", "true", "toggle colorized logs (valid options: true, false, fabulous)")
	dumpLogsValue := rootCmd.PersistentFlags().BoolP("dumpLogs", "d", false, "dump logs to disk on failure if set to true")

	logBuffer := new(bytes.Buffer)

	cobra.OnInitialize(func() {
		initLogger(*loggerLevel, *colorValue, logBuffer, *dumpLogsValue)
	})

	flagGrouping := cmdutils.NewGrouping()
	createCmd := create.Command(flagGrouping)
	rootCmd.AddCommand(createCmd)

	checkCommand(rootCmd)

	createArgs := []string{
		"create",
		"cluster",
		"--kubeconfig",
		c.KubeConfigFilePath,
		"-n",
		c.ClusterName,
		"--region",
		c.Region,
		"--managed",
	}

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
