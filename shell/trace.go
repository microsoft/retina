// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package shell

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// TraceConfig holds the validated configuration for network tracing.
// All fields are typed values - no raw user strings for security.
type TraceConfig struct {
	// Kubernetes configuration
	RestConfig       *rest.Config
	RetinaShellImage string

	// Filter configuration (validated, typed values only)
	FilterIPs   []net.IP     // Validated IP addresses to filter
	FilterCIDRs []*net.IPNet // Validated CIDRs to filter

	// Output configuration
	OutputJSON bool // true for JSON output, false for table

	// Feature flags
	EnableNetfilter bool // Enable netfilter table/chain enrichment (requires BTF)

	// Timing configuration
	TraceDuration time.Duration // How long to trace (0 = until Ctrl-C)
	Timeout       time.Duration // Pod startup timeout
}

// TraceCapabilities returns the required Linux capabilities for bpftrace.
// These are set automatically and not user-configurable.
func TraceCapabilities() []string {
	return []string{
		"SYS_ADMIN",    // Required for bpftrace
		"BPF",          // Load BPF programs
		"PERFMON",      // Perf events access
		"NET_ADMIN",    // Network tracing
		"SYS_PTRACE",   // Process tracing (for stack traces)
		"SYS_RESOURCE", // Increase rlimits for BPF maps
		"MKNOD",        // Required for bpftrace debugfs access
		"SYS_CHROOT",   // Required for bpftrace
	}
}

// RunTrace starts a network trace on a node.
// It creates a privileged pod on the target node, runs bpftrace, and streams output.
func RunTrace(ctx context.Context, config TraceConfig, nodeName, debugPodNamespace string) error {
	clientset, err := kubernetes.NewForConfig(config.RestConfig)
	if err != nil {
		return fmt.Errorf("error constructing kube clientset: %w", err)
	}

	// Validate node OS
	err = validateOperatingSystemSupportedForNode(ctx, clientset, nodeName)
	if err != nil {
		return fmt.Errorf("error validating operating system for node %s: %w", nodeName, err)
	}

	// Create the trace pod
	pod := hostNetworkPodForTrace(config, debugPodNamespace, nodeName)

	fmt.Printf("Creating trace pod %s/%s on node %s\n", debugPodNamespace, pod.Name, nodeName)
	createdPod, err := clientset.CoreV1().
		Pods(debugPodNamespace).
		Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("error creating trace pod %s in namespace %s: %w", pod.Name, debugPodNamespace, err)
	}

	// Ensure cleanup on exit (Ctrl-C, error, or normal termination)
	// Note: intentionally using context.Background() for cleanup so it runs even if ctx is canceled
	defer func() { //nolint:contextcheck // cleanup must run regardless of parent context state
		fmt.Printf("Cleaning up trace pod %s/%s\n", debugPodNamespace, createdPod.Name)
		deleteCtx := context.Background() // Use fresh context for cleanup
		deleteErr := clientset.CoreV1().
			Pods(debugPodNamespace).
			Delete(deleteCtx, createdPod.Name, metav1.DeleteOptions{})
		if deleteErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to delete trace pod %s: %v\n", createdPod.Name, deleteErr)
		}
	}()

	// Wait for pod to be running
	err = waitForContainerRunning(ctx, config.Timeout, clientset, debugPodNamespace, createdPod.Name, createdPod.Spec.Containers[0].Name)
	if err != nil {
		return fmt.Errorf("error waiting for trace pod to start: %w", err)
	}

	fmt.Printf("Trace pod ready, starting trace...\n")

	// First, fetch and display reason/state codes from kernel
	// These are kernel-version specific so we read them at runtime
	fmt.Printf("\n")

	// Display SKB drop reason codes (for DROP events)
	dropReasonsCommand := DropReasonsCommand()
	err = execInPod(ctx, config.RestConfig, clientset, debugPodNamespace, createdPod.Name, createdPod.Spec.Containers[0].Name, dropReasonsCommand, os.Stdout, os.Stderr)
	if err != nil {
		// Non-fatal: continue even if we can't get reason codes
		fmt.Fprintf(os.Stderr, "warning: could not fetch drop reason codes: %v\n", err)
	}
	fmt.Printf("\n")

	// Generate and run the bpftrace script
	gen := NewScriptGenerator(config)
	script := gen.Generate()

	// Run bpftrace with the generated script
	// SECURITY: The script is passed via -e flag, not interpolated into a shell command
	bpftraceCommand := []string{"bpftrace", "-e", script}

	err = execInPod(ctx, config.RestConfig, clientset, debugPodNamespace, createdPod.Name, createdPod.Spec.Containers[0].Name, bpftraceCommand, os.Stdout, os.Stderr)
	if err != nil {
		// If duration was specified and context was cancelled, it's expected behavior
		if config.TraceDuration > 0 && ctx.Err() != nil {
			fmt.Printf("\nTrace completed after %s\n", config.TraceDuration)
			return nil
		}
		return fmt.Errorf("error executing trace command: %w", err)
	}

	return nil
}

// execInPod executes a command inside a pod container without using a shell.
// SECURITY: The command is passed as an array directly to the container runtime,
// preventing shell injection attacks. No shell interpolation occurs.
//
// Parameters:
//   - ctx: Context for cancellation (e.g., Ctrl-C)
//   - restConfig: Kubernetes REST client config
//   - clientset: Kubernetes clientset
//   - namespace: Pod namespace
//   - podName: Pod name
//   - containerName: Container name
//   - command: Command and arguments as string array (NO SHELL - passed directly)
//   - stdout: Writer for stdout (typically os.Stdout)
//   - stderr: Writer for stderr (typically os.Stderr)
func execInPod(
	ctx context.Context,
	restConfig *rest.Config,
	clientset *kubernetes.Clientset,
	namespace, podName, containerName string,
	command []string,
	stdout, stderr io.Writer,
) error {
	// Build the exec request using the REST API directly
	// SECURITY: Command is passed as array in PodExecOptions, NOT through a shell
	req := clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Container: containerName,
			Command:   command, // Direct command array - no shell!
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	// Create the SPDY executor
	exec, err := remotecommand.NewSPDYExecutor(restConfig, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("error creating executor: %w", err)
	}

	// Stream the output
	// The Stream function blocks until the command completes or context is cancelled
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: stdout,
		Stderr: stderr,
	})
	if err != nil {
		// Check if it was a context cancellation (user pressed Ctrl-C)
		if ctx.Err() != nil {
			return fmt.Errorf("context error: %w", ctx.Err())
		}
		return fmt.Errorf("error streaming command output: %w", err)
	}

	return nil
}

// hostNetworkPodForTrace creates a pod manifest for network tracing.
// The pod runs with host network and required capabilities for bpftrace.
func hostNetworkPodForTrace(config TraceConfig, debugPodNamespace, nodeName string) *v1.Pod {
	// Use Args (not Command) to preserve the image entrypoint.
	// The entrypoint.sh in retina-shell image mounts debugfs/tracefs which bpftrace needs.
	args := []string{"sleep", "infinity"}

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      randomTraceContainerName(),
			Namespace: debugPodNamespace,
			Labels: map[string]string{
				"app":                         "retina-trace",
				"retina.sh/component":         "trace",
				"retina.sh/trace-target-node": nodeName,
			},
		},
		Spec: v1.PodSpec{
			NodeName:      nodeName,
			RestartPolicy: v1.RestartPolicyNever,
			Tolerations:   []v1.Toleration{{Operator: v1.TolerationOpExists}},
			HostNetwork:   true,
			HostPID:       true, // Required for full process visibility
			Containers: []v1.Container{
				{
					Name:  "retina-trace",
					Image: config.RetinaShellImage,
					Args:  args,  // Use Args to preserve entrypoint.sh
					Stdin: false, // Not interactive
					TTY:   false, // Not interactive
					SecurityContext: &v1.SecurityContext{
						Privileged: boolPtr(false), // Use capabilities instead
						Capabilities: &v1.Capabilities{
							Drop: []v1.Capability{"ALL"},
							Add:  stringSliceToCapabilities(TraceCapabilities()),
						},
						// Required for bpftrace (per shell.md documentation)
						SeccompProfile: &v1.SeccompProfile{
							Type: v1.SeccompProfileTypeUnconfined,
						},
						AppArmorProfile: &v1.AppArmorProfile{
							Type: v1.AppArmorProfileTypeUnconfined,
						},
					},
				},
			},
		},
	}

	return pod
}

// randomTraceContainerName generates a unique name for the trace pod.
func randomTraceContainerName() string {
	const randLen = 5
	return "retina-trace-" + utilrand.String(randLen)
}

// boolPtr returns a pointer to a bool value.
func boolPtr(b bool) *bool {
	return &b
}
