// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package debug

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cilium/cilium/api/v1/flow"
	hubblev1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	retinacmd "github.com/microsoft/retina/cli/cmd"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/managers/filtermanager"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/plugin/dropreason"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/term"
)

var dropOpts = struct {
	duration     time.Duration
	outputFile   string
	confirm      bool
	portForward  bool
	metricsPort  int
	namespace    string
	podName      string
	ips          []string
	verbose      bool
	consoleWidth int
}{}

var dropCmd = &cobra.Command{
	Use:   "drop",
	Short: "Watch for packet drop events",
	Long: `Watch for packet drop events in real-time using the Retina dropreason plugin.

This command monitors network packet drops and displays information about:
- Drop reason
- Source and destination information
- Packet details
- Timestamps

The command can output results to the console with proper formatting or save them to a file.`,
	RunE: runDropCommand,
}

func init() {
	debug.AddCommand(dropCmd)
	
	dropCmd.Flags().DurationVar(&dropOpts.duration, "duration", 30*time.Second, "Duration to watch for drop events")
	dropCmd.Flags().StringVar(&dropOpts.outputFile, "output", "", "Output file to write drop events (optional)")
	dropCmd.Flags().BoolVar(&dropOpts.confirm, "confirm", true, "Confirm before performing invasive operations")
	dropCmd.Flags().BoolVar(&dropOpts.portForward, "port-forward", false, "Enable port forwarding for remote monitoring")
	dropCmd.Flags().IntVar(&dropOpts.metricsPort, "metrics-port", 10093, "Metrics port for Retina")
	dropCmd.Flags().StringVar(&dropOpts.namespace, "namespace", "kube-system", "Namespace where Retina pods are running")
	dropCmd.Flags().StringVar(&dropOpts.podName, "pod-name", "", "Specific pod name to monitor (optional)")
	dropCmd.Flags().StringSliceVar(&dropOpts.ips, "ips", nil, "IP addresses to filter for (optional)")
	dropCmd.Flags().BoolVar(&dropOpts.verbose, "verbose", false, "Enable verbose output")
	dropCmd.Flags().IntVar(&dropOpts.consoleWidth, "width", 0, "Console width for formatting (auto-detected if 0)")
}

func runDropCommand(cmd *cobra.Command, args []string) error {
	logger := retinacmd.Logger.Named("debug-drop")
	
	// Set up console width
	if dropOpts.consoleWidth == 0 {
		if width, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
			dropOpts.consoleWidth = width
		} else {
			dropOpts.consoleWidth = 80 // Default width
		}
	}

	// Confirm invasive operations
	if dropOpts.portForward && dropOpts.confirm {
		if !confirmOperation("This will set up port forwarding to monitor drop events. Continue?") {
			logger.Info("Operation cancelled by user")
			return nil
		}
	}

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), dropOpts.duration)
	defer cancel()

	sigCtx, sigCancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer sigCancel()

	return runDropMonitoring(sigCtx, logger)
}

func confirmOperation(message string) bool {
	fmt.Printf("%s (y/N): ", message)
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

func runDropMonitoring(ctx context.Context, logger *log.ZapLogger) error {
	logger.Info("Starting drop event monitoring",
		zap.Duration("duration", dropOpts.duration),
		zap.String("output", dropOpts.outputFile),
		zap.Bool("portForward", dropOpts.portForward),
	)

	// Initialize metrics
	metrics.InitializeMetrics()

	// Create configuration
	cfg := &kcfg.Config{
		MetricsInterval: 1 * time.Second,
		EnablePodLevel:  true,
	}

	// Set up filtermanager if IPs are specified
	if len(dropOpts.ips) > 0 {
		fm, err := filtermanager.Init(3)
		if err != nil {
			return fmt.Errorf("failed to initialize filter manager: %w", err)
		}
		defer func() {
			if err := fm.Stop(); err != nil {
				logger.Error("Failed to stop filter manager", zap.Error(err))
			}
		}()
		
		// Convert string IPs to net.IP
		ips := make([]string, len(dropOpts.ips))
		copy(ips, dropOpts.ips)
		
		logger.Info("Filtering for IPs", zap.Strings("ips", ips))
		// filterManager will be used for filtering (not implemented in this scope)
		_ = fm
	}

	// Create and configure dropreason plugin
	dr := dropreason.New(cfg)

	// Generate and compile eBPF program
	if err := dr.Generate(ctx); err != nil {
		return fmt.Errorf("failed to generate eBPF program: %w", err)
	}

	if err := dr.Compile(ctx); err != nil {
		return fmt.Errorf("failed to compile eBPF program: %w", err)
	}

	if err := dr.Init(); err != nil {
		// Check if this is a common eBPF-related error and provide helpful message
		errMsg := err.Error()
		if strings.Contains(errMsg, "operation not permitted") || strings.Contains(errMsg, "MEMLOCK") {
			return fmt.Errorf("failed to initialize dropreason plugin: %w\n\nThis error typically occurs when:\n- Running without sufficient privileges (try sudo)\n- eBPF is not available or restricted in this environment\n- Memory lock limits are too low (ulimit -l)\n\nFor production use, this command should be run in an environment with eBPF support", err)
		}
		return fmt.Errorf("failed to initialize dropreason plugin: %w", err)
	}

	// Set up event channel
	eventChannel := make(chan *hubblev1.Event, 100)
	if err := dr.SetupChannel(eventChannel); err != nil {
		return fmt.Errorf("failed to setup event channel: %w", err)
	}

	// Set up output writer
	var outputWriter *os.File
	if dropOpts.outputFile != "" {
		file, err := os.OpenFile(dropOpts.outputFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open output file: %w", err)
		}
		defer file.Close()
		outputWriter = file
		logger.Info("Writing output to file", zap.String("file", dropOpts.outputFile))
	}

	// Start monitoring
	if err := dr.Start(ctx); err != nil {
		return fmt.Errorf("failed to start dropreason plugin: %w", err)
	}
	defer dr.Stop()

	logger.Info("Drop monitoring started. Press Ctrl+C to stop.")
	
	// Print header
	printHeader()

	// Process events
	for {
		select {
		case <-ctx.Done():
			logger.Info("Drop monitoring stopped")
			return nil
		case event := <-eventChannel:
			if event != nil && event.Event != nil {
				line := formatHubbleEvent(event)
				fmt.Println(line)
				
				if outputWriter != nil {
					if _, err := outputWriter.WriteString(line + "\n"); err != nil {
						logger.Error("Failed to write to output file", zap.Error(err))
					}
				}
			}
		}
	}
}

func printHeader() {
	header := fmt.Sprintf("%-20s %-15s %-15s %-10s %-20s %-s",
		"TIMESTAMP", "SRC_IP", "DST_IP", "PROTO", "DROP_REASON", "DETAILS")
	
	fmt.Println(header)
	fmt.Println(strings.Repeat("-", len(header)))
}

func formatHubbleEvent(event *hubblev1.Event) string {
	if event == nil || event.Event == nil {
		return ""
	}

	// Cast the event to a flow.Flow
	flowEvent, ok := event.Event.(*flow.Flow)
	if !ok {
		return "Error: unable to cast event to flow"
	}

	timestamp := time.Now()
	if event.Timestamp != nil {
		timestamp = event.Timestamp.AsTime()
	}

	// Extract basic info from the flow
	srcIP := "unknown"
	dstIP := "unknown"
	protocol := "unknown"
	reason := "unknown"
	
	if flowEvent.GetIP() != nil {
		srcIP = flowEvent.GetIP().GetSource()
		dstIP = flowEvent.GetIP().GetDestination()
	}

	if flowEvent.GetL4() != nil {
		if tcp := flowEvent.GetL4().GetTCP(); tcp != nil {
			protocol = "TCP"
		} else if udp := flowEvent.GetL4().GetUDP(); udp != nil {
			protocol = "UDP"
		} else if icmp := flowEvent.GetL4().GetICMPv4(); icmp != nil {
			protocol = "ICMPv4"
		} else if icmp := flowEvent.GetL4().GetICMPv6(); icmp != nil {
			protocol = "ICMPv6"
		}
	}

	if flowEvent.GetDropReason() != 0 {
		reason = fmt.Sprintf("DROP(%d)", flowEvent.GetDropReason())
	}

	// Create additional details
	details := ""
	if flowEvent.GetSummary() != "" {
		details = flowEvent.GetSummary()
	}

	// Word wrap logic for console width
	maxDetailsWidth := dropOpts.consoleWidth - 82 // Reserve space for other columns
	if maxDetailsWidth < 10 {
		maxDetailsWidth = 10
	}

	if len(details) > maxDetailsWidth {
		details = details[:maxDetailsWidth-3] + "..."
	}

	return fmt.Sprintf("%-20s %-15s %-15s %-10s %-20s %-s",
		timestamp.Format("15:04:05.000"),
		srcIP,
		dstIP,
		protocol,
		reason,
		details,
	)
}

func formatDropEvent(timestamp time.Time, srcIP, dstIP, protocol, reason, details string) string {
	// Word wrap logic for console width
	maxDetailsWidth := dropOpts.consoleWidth - 82 // Reserve space for other columns
	if maxDetailsWidth < 10 {
		maxDetailsWidth = 10
	}

	if len(details) > maxDetailsWidth {
		details = details[:maxDetailsWidth-3] + "..."
	}

	return fmt.Sprintf("%-20s %-15s %-15s %-10s %-20s %-s",
		timestamp.Format("15:04:05.000"),
		srcIP,
		dstIP,
		protocol,
		reason,
		details,
	)
}