// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
)

type Opts struct {
	genericclioptions.ConfigFlags
	Name               *string
	blobUpload         string
	debug              bool
	duration           time.Duration
	excludeFilter      string
	hostPath           string
	includeFilter      string
	includeMetadata    bool
	interfaces         string
	jobNumLimit        int
	maxSize            int
	namespaceSelectors string
	nodeNames          string
	nodeSelectors      string
	nowait             bool
	packetSize         int
	podNames           string
	podSelectors       string
	pvc                string
	s3AccessKeyID      string
	s3Bucket           string
	s3Endpoint         string
	s3Path             string
	s3Region           string
	s3SecretAccessKey  string
	// tcpdumpFilter is Obsolete use captureOption.pcapFilter and captureOption boolean flags for display options.
	tcpdumpFilter        string
	pcapFilter           string
	noPromiscuous        bool
	packetBuffered       bool
	immediateMode        bool
	noResolveDNS         bool
	noResolvePort        bool
	verbose              bool
	extraVerbose         bool
	maxVerbose           bool
	printDataHex         bool
	printDataHexLink     bool
	printDataASCII       bool
	printDataASCIILink   bool
	printLinkHeader      bool
	quietOutput          bool
	absoluteSeq          bool
	noTimestamp          bool
	unformattedTimestamp bool
	deltaTimestamp       bool
	dateTimestamp        bool
	deltaSinceFirst      bool
	dontVerifyChecksum   bool
}

var opts = Opts{
	Name: new(string),
}

const DefaultName = "retina-capture"

func NewCommand(kubeClient kubernetes.Interface) *cobra.Command {
	capture := &cobra.Command{
		Use:   "capture",
		Short: "Capture network traffic",
		Long:  "Capture network traffic from pods in a Kubernetes cluster.",
	}

	opts.ConfigFlags = *genericclioptions.NewConfigFlags(true)
	opts.AddFlags(capture.PersistentFlags())
	capture.PersistentFlags().StringVar(opts.Name, "name", DefaultName, "The name of the Retina Capture")

	capture.AddCommand(NewCreateSubCommand(kubeClient))
	capture.AddCommand(NewDeleteSubCommand(kubeClient))
	capture.AddCommand(NewDownloadSubCommand())
	capture.AddCommand(NewListSubCommand())

	return capture
}
