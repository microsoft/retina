// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/microsoft/retina/shell"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubectl/pkg/util/templates"
)

// Trace command flags - use separate variables to avoid conflicts with shell command
var (
	traceConfigFlags       *genericclioptions.ConfigFlags
	traceMatchVersionFlags *cmdutil.MatchVersionFlags

	// Image settings
	traceRetinaShellImageRepo    string
	traceRetinaShellImageVersion string

	// Filter settings (raw strings from CLI, validated before use)
	traceFilterIP   string
	traceFilterCIDR string

	// Output settings
	traceOutputFormat   string
	traceDuration       time.Duration
	traceStartupTimeout time.Duration

	// Event selection flags
	traceAll          bool
	traceDrops        bool
	traceRST          bool
	traceErrors       bool
	traceRetransmits  bool
	traceNfqueueDrops bool
)

// TraceOutputFormat represents validated output format options
type TraceOutputFormat string

const (
	TraceOutputTable TraceOutputFormat = "table"
	TraceOutputJSON  TraceOutputFormat = "json"
)

// Validation errors
var (
	errInvalidIP           = errors.New("invalid IP address")
	errInvalidCIDR         = errors.New("invalid CIDR notation")
	errInvalidOutputFormat = errors.New("invalid output format: must be 'table' or 'json'")
	errNodeOnly            = errors.New("bpftrace command only supports nodes, not pods")
)

// ValidateFilterIP validates an IP address string and returns the parsed IP.
// Returns nil IP and no error if input is empty (no filter).
// Returns error if input is non-empty but invalid.
func ValidateFilterIP(input string) (net.IP, error) {
	if input == "" {
		return nil, nil
	}
	ip := net.ParseIP(input)
	if ip == nil {
		return nil, fmt.Errorf("%w: %q", errInvalidIP, input)
	}
	return ip, nil
}

// ValidateFilterCIDR validates a CIDR string and returns the parsed IPNet.
// Returns nil and no error if input is empty (no filter).
// Returns error if input is non-empty but invalid.
func ValidateFilterCIDR(input string) (*net.IPNet, error) {
	if input == "" {
		return nil, nil
	}
	_, ipnet, err := net.ParseCIDR(input)
	if err != nil {
		return nil, fmt.Errorf("%w: %q: %w", errInvalidCIDR, input, err)
	}
	return ipnet, nil
}

// ValidateOutputFormat validates the output format string.
func ValidateOutputFormat(input string) (TraceOutputFormat, error) {
	switch input {
	case "table", "":
		return TraceOutputTable, nil
	case "json":
		return TraceOutputJSON, nil
	default:
		return "", fmt.Errorf("%w: got %q", errInvalidOutputFormat, input)
	}
}

var bpftraceCmd = &cobra.Command{
	Use:   "bpftrace NODE",
	Short: "[EXPERIMENTAL] Trace network issues on a node using bpftrace",
	Long: templates.LongDesc(`
	[EXPERIMENTAL] This is an experimental command. The flags and behavior may change in the future.

	Trace network issues (packet drops, TCP resets, connection errors) on a node in real-time
	using bpftrace.

	This creates a privileged pod on the target node that runs bpftrace to capture:
	* Packet drops (with drop reason: NETFILTER_DROP, NO_SOCKET, etc.) [--drops]
	* TCP RST sent/received (connection refused, reset by peer) [--rst]
	* Socket errors (ECONNREFUSED, ETIMEDOUT, etc.) [--errors]
	* TCP retransmissions (packet loss indicators) [--retransmits]
	* NFQUEUE drops (no consumer on iptables NFQUEUE target) [--nfqueue-drops]

	By default, all event types are traced. Use individual flags to trace specific events only.

	Use --ip or --cidr to focus on specific endpoints.
	The filter matches both source AND destination addresses.

	Note: Currently supports IPv4 only.
`),

	Example: templates.Examples(`
		# trace all network issues on a node (default)
		kubectl retina bpftrace node0001

		# trace only packet drops
		kubectl retina bpftrace node0001 --drops

		# trace drops and RSTs for a specific IP
		kubectl retina bpftrace node0001 --drops --rst --ip 10.244.1.15

		# trace retransmits for a subnet
		kubectl retina bpftrace node0001 --retransmits --cidr 10.244.0.0/16

		# trace for 60 seconds and exit
		kubectl retina bpftrace node0001 --duration 60s

		# output in JSON format (for scripting)
		kubectl retina bpftrace node0001 --output json

		# combine options
		kubectl retina bpftrace node0001 --ip 10.244.1.15 --duration 30s --output json
`),
	Args: cobra.ExactArgs(1),
	RunE: runBpftrace,
}

func runBpftrace(_ *cobra.Command, args []string) error {
	// Validate image version
	if traceRetinaShellImageVersion == "" {
		return errMissingRequiredRetinaShellImageVersionArg
	}

	// === SECURITY: Validate all user inputs BEFORE any use ===

	// Validate IP filter (strict parsing)
	filterIP, err := ValidateFilterIP(traceFilterIP)
	if err != nil {
		return fmt.Errorf("invalid --ip: %w", err)
	}

	// Validate CIDR filter (strict parsing)
	filterCIDR, err := ValidateFilterCIDR(traceFilterCIDR)
	if err != nil {
		return fmt.Errorf("invalid --cidr: %w", err)
	}

	// Validate output format (whitelist)
	outputFormat, err := ValidateOutputFormat(traceOutputFormat)
	if err != nil {
		return err
	}

	// Get namespace
	namespace, explicitNamespace, err := traceMatchVersionFlags.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return fmt.Errorf("error retrieving namespace arg: %w", err)
	}

	// Parse node argument (only nodes supported, not pods)
	r := resource.NewBuilder(traceConfigFlags).
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		FilenameParam(explicitNamespace, &resource.FilenameOptions{}).
		NamespaceParam(namespace).DefaultNamespace().ResourceNames("nodes", args[0]).
		Do()
	if rerr := r.Err(); rerr != nil {
		return fmt.Errorf("error constructing resource builder: %w", rerr)
	}

	// Get REST config
	restConfig, err := traceMatchVersionFlags.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("error constructing REST config: %w", err)
	}

	// Visit the resource (should be a node)
	return r.Visit(func(info *resource.Info, err error) error { //nolint:wrapcheck // visitor pattern returns errors as-is
		if err != nil {
			return err
		}

		switch obj := info.Object.(type) {
		case *v1.Node:
			nodeName := obj.Name
			podNamespace := namespace

			// Determine which events to trace
			// If no individual flags set, or --all is set, enable all events
			enableAll := traceAll || (!traceDrops && !traceRST && !traceErrors && !traceRetransmits && !traceNfqueueDrops)

			// Build TraceConfig with validated, typed values only
			traceConfig := shell.TraceConfig{
				RestConfig:         restConfig,
				RetinaShellImage:   fmt.Sprintf("%s:%s", traceRetinaShellImageRepo, traceRetinaShellImageVersion),
				FilterIPs:          nil,
				FilterCIDRs:        nil,
				OutputJSON:         outputFormat == TraceOutputJSON,
				TraceDuration:      traceDuration,
				Timeout:            traceStartupTimeout,
				EnableDrops:        enableAll || traceDrops,
				EnableRST:          enableAll || traceRST,
				EnableErrors:       enableAll || traceErrors,
				EnableRetransmits:  enableAll || traceRetransmits,
				EnableNfqueueDrops: enableAll || traceNfqueueDrops,
			}

			// Add validated IP filter (already typed as net.IP)
			if filterIP != nil {
				traceConfig.FilterIPs = append(traceConfig.FilterIPs, filterIP)
			}

			// Add validated CIDR filter (already typed as *net.IPNet)
			if filterCIDR != nil {
				traceConfig.FilterCIDRs = append(traceConfig.FilterCIDRs, filterCIDR)
			}

			// Create context with cancellation for Ctrl-C handling
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Handle Ctrl-C gracefully
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigCh
				fmt.Println("\nReceived interrupt, cleaning up...")
				cancel()
			}()

			// Apply duration timeout if specified
			if traceDuration > 0 {
				var timeoutCancel context.CancelFunc
				ctx, timeoutCancel = context.WithTimeout(ctx, traceDuration)
				defer timeoutCancel()
			}

			return shell.RunTrace(ctx, traceConfig, nodeName, podNamespace)

		case *v1.Pod:
			return errNodeOnly

		default:
			gvk := obj.GetObjectKind().GroupVersionKind()
			return fmt.Errorf("unsupported resource %s/%s: %w", gvk.GroupVersion(), gvk.Kind, errUnsupportedResourceType)
		}
	})
}

func init() {
	Retina.AddCommand(bpftraceCmd)

	bpftraceCmd.PersistentPreRun = func(cmd *cobra.Command, _ []string) {
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true

		// Allow setting image repo and version via environment variables
		if !cmd.Flags().Changed("retina-shell-image-repo") {
			if envRepo := os.Getenv("RETINA_SHELL_IMAGE_REPO"); envRepo != "" {
				traceRetinaShellImageRepo = envRepo
			}
		}
		if !cmd.Flags().Changed("retina-shell-image-version") {
			if envVersion := os.Getenv("RETINA_SHELL_IMAGE_VERSION"); envVersion != "" {
				traceRetinaShellImageVersion = envVersion
			}
		}
	}

	// Image flags (same as shell command)
	bpftraceCmd.Flags().StringVar(&traceRetinaShellImageRepo, "retina-shell-image-repo",
		defaultRetinaShellImageRepo, "The container registry repository for the retina-shell image")
	bpftraceCmd.Flags().StringVar(&traceRetinaShellImageVersion, "retina-shell-image-version",
		defaultRetinaShellImageVersion, "The version (tag) of the retina-shell image")

	// Filter flags
	bpftraceCmd.Flags().StringVar(&traceFilterIP, "ip", "",
		"Filter by IP address (matches source OR destination)")
	bpftraceCmd.Flags().StringVar(&traceFilterCIDR, "cidr", "",
		"Filter by CIDR (matches source OR destination)")

	// Event selection flags
	bpftraceCmd.Flags().BoolVar(&traceAll, "all", false,
		"Enable all event types (default behavior when no event flags specified)")
	bpftraceCmd.Flags().BoolVar(&traceDrops, "drops", false,
		"Enable packet drop events (kfree_skb tracepoint)")
	bpftraceCmd.Flags().BoolVar(&traceRST, "rst", false,
		"Enable TCP RST events (tcp_send_reset/tcp_receive_reset)")
	bpftraceCmd.Flags().BoolVar(&traceErrors, "errors", false,
		"Enable socket error events (inet_sk_error_report)")
	bpftraceCmd.Flags().BoolVar(&traceRetransmits, "retransmits", false,
		"Enable TCP retransmit events (tcp_retransmit_skb)")
	bpftraceCmd.Flags().BoolVar(&traceNfqueueDrops, "nfqueue-drops", false,
		"Enable NFQUEUE drop events (fexit:vmlinux:__nf_queue, requires BTF)")

	// Output flags
	bpftraceCmd.Flags().StringVarP(&traceOutputFormat, "output", "o", "table",
		"Output format: 'table' (human-readable) or 'json' (machine-readable)")
	bpftraceCmd.Flags().DurationVar(&traceDuration, "duration", 0,
		"How long to trace (e.g., 30s, 5m). 0 means until Ctrl-C.")
	bpftraceCmd.Flags().DurationVar(&traceStartupTimeout, "startup-timeout", defaultTimeout,
		"Timeout for starting the trace pod")

	// Kubernetes config flags
	traceConfigFlags = genericclioptions.NewConfigFlags(true)
	traceConfigFlags.AddFlags(bpftraceCmd.PersistentFlags())
	traceMatchVersionFlags = cmdutil.NewMatchVersionFlags(traceConfigFlags)
	traceMatchVersionFlags.AddFlags(bpftraceCmd.PersistentFlags())
}
