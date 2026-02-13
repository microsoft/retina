// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package shell

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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
	defer func() {
		fmt.Printf("Cleaning up trace pod %s/%s\n", debugPodNamespace, createdPod.Name)
		deleteCtx := context.Background() // Use fresh context for cleanup
		err := clientset.CoreV1().
			Pods(debugPodNamespace).
			Delete(deleteCtx, createdPod.Name, metav1.DeleteOptions{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to delete trace pod %s: %v\n", createdPod.Name, err)
		}
	}()

	// Wait for pod to be running
	err = waitForContainerRunning(ctx, config.Timeout, clientset, debugPodNamespace, createdPod.Name, createdPod.Spec.Containers[0].Name)
	if err != nil {
		return fmt.Errorf("error waiting for trace pod to start: %w", err)
	}

	fmt.Printf("Trace pod ready, starting trace...\n")

	// TODO: Step 3 will implement execInPod to run bpftrace
	// For now, just demonstrate the pod lifecycle works
	return fmt.Errorf("trace execution not yet implemented - Step 3 will add execInPod()")
}

// hostNetworkPodForTrace creates a pod manifest for network tracing.
// The pod runs with host network and required capabilities for bpftrace.
func hostNetworkPodForTrace(config TraceConfig, debugPodNamespace, nodeName string) *v1.Pod {
	// Use a long sleep command - we'll exec into it
	// The entrypoint.sh in retina-shell image mounts debugfs/tracefs
	command := []string{"sleep", "infinity"}

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
					Name:    "retina-trace",
					Image:   config.RetinaShellImage,
					Command: command,
					Stdin:   false, // Not interactive
					TTY:     false, // Not interactive
					SecurityContext: &v1.SecurityContext{
						Privileged: boolPtr(false), // Use capabilities instead
						Capabilities: &v1.Capabilities{
							Drop: []v1.Capability{"ALL"},
							Add:  stringSliceToCapabilities(TraceCapabilities()),
						},
						// Required for bpftrace
						SeccompProfile: &v1.SeccompProfile{
							Type: v1.SeccompProfileTypeUnconfined,
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
