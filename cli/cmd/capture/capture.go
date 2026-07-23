// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
)

var (
	ErrInvalidVerbosityLevel  = errors.New("invalid verbosity level")
	ErrInvalidTimestampFormat = errors.New("invalid timestamp format")
	ErrInvalidPrintDataFormat = errors.New("invalid print data format")
	ErrBPFFilterEmpty         = errors.New("BPF filter cannot be empty or whitespace-only")
	ErrBPFFilterContainsFlag  = errors.New("BPF filter contains flag which is not allowed")
)

// VerbosityLevel represents the verbosity level for packet capture output
type VerbosityLevel string

const (
	VerbosityNormal  VerbosityLevel = ""        // Default, no extra verbosity
	VerbosityVerbose VerbosityLevel = "verbose" // tcpdump -v
	VerbosityExtra   VerbosityLevel = "extra"   // tcpdump -vv
	VerbosityMax     VerbosityLevel = "max"     // tcpdump -vvv
)

func (v VerbosityLevel) Validate() error {
	switch v {
	case VerbosityNormal, VerbosityVerbose, VerbosityExtra, VerbosityMax:
		return nil
	default:
		return fmt.Errorf("%w: %s (valid: verbose, extra, max)", ErrInvalidVerbosityLevel, v)
	}
}

// TimestampFormat represents the timestamp format for packet capture output
type TimestampFormat string

const (
	TimestampDefault         TimestampFormat = ""                  // Default formatted timestamp
	TimestampNone            TimestampFormat = "none"              // tcpdump -t
	TimestampUnformatted     TimestampFormat = "unformatted"       // tcpdump -tt (Unix epoch)
	TimestampDelta           TimestampFormat = "delta"             // tcpdump -ttt (delta between packets)
	TimestampDate            TimestampFormat = "date"              // tcpdump -tttt (with date)
	TimestampDeltaSinceFirst TimestampFormat = "delta-since-first" // tcpdump -ttttt (delta since first)
)

func (t TimestampFormat) Validate() error {
	switch t {
	case TimestampDefault, TimestampNone, TimestampUnformatted, TimestampDelta, TimestampDate, TimestampDeltaSinceFirst:
		return nil
	default:
		return fmt.Errorf("%w: %s (valid: none, unformatted, delta, date, delta-since-first)", ErrInvalidTimestampFormat, t)
	}
}

// PrintDataFormat represents the format for printing packet data
type PrintDataFormat string

const (
	PrintDataNone          PrintDataFormat = ""                // Default, no data printing
	PrintDataHex           PrintDataFormat = "hex"             // tcpdump -x (hex only)
	PrintDataHexWithLink   PrintDataFormat = "hex-with-link"   // tcpdump -xx (hex with link header)
	PrintDataASCII         PrintDataFormat = "ascii"           // tcpdump -A (ASCII only)
	PrintDataASCIIWithLink PrintDataFormat = "ascii-with-link" // tcpdump -AA (ASCII with link header)
)

func (p PrintDataFormat) Validate() error {
	switch p {
	case PrintDataNone, PrintDataHex, PrintDataHexWithLink, PrintDataASCII, PrintDataASCIIWithLink:
		return nil
	default:
		return fmt.Errorf("%w: %s (valid: hex, hex-with-link, ascii, ascii-with-link)", ErrInvalidPrintDataFormat, p)
	}
}

type Opts struct {
	genericclioptions.ConfigFlags
	Name               *string
	blobUpload         string
	cleanUpAfterUpload bool
	debug              bool
	duration           time.Duration
	excludeFilter      string
	fileCount          int
	hostPath           string
	hostPathBaseDir    string
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
	// tcpdumpFilter is deprecated and will be removed. Use captureOption.pcapFilter and captureOption boolean flags for display options.
	tcpdumpFilter      string
	pcapFilter         string
	noPromiscuous      bool
	packetBuffered     bool
	immediateMode      bool
	noResolveDNS       bool
	noResolvePort      bool
	verbosityLevel     VerbosityLevel
	timestampFormat    TimestampFormat
	printDataFormat    PrintDataFormat
	printLinkHeader    bool
	quietOutput        bool
	absoluteSeq        bool
	dontVerifyChecksum bool
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
