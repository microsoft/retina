// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package shell

import (
	"net"
	"testing"

	v1 "k8s.io/api/core/v1"
)

func TestTraceCapabilities(t *testing.T) {
	caps := TraceCapabilities()

	// Must have at least these required capabilities for bpftrace
	requiredCaps := []string{
		"SYS_ADMIN",
		"BPF",
		"PERFMON",
		"NET_ADMIN",
	}

	capSet := make(map[string]bool)
	for _, c := range caps {
		capSet[c] = true
	}

	for _, required := range requiredCaps {
		if !capSet[required] {
			t.Errorf("TraceCapabilities() missing required capability: %s", required)
		}
	}
}

func TestTraceConfigTypedFields(t *testing.T) {
	// Verify TraceConfig only accepts typed values, not raw strings
	config := TraceConfig{
		FilterIPs:   []net.IP{net.ParseIP("10.0.0.1")},
		FilterCIDRs: []*net.IPNet{},
		OutputJSON:  true,
	}

	// FilterIPs should be net.IP, not string
	if len(config.FilterIPs) != 1 {
		t.Errorf("Expected 1 FilterIP, got %d", len(config.FilterIPs))
	}
	if !config.FilterIPs[0].Equal(net.ParseIP("10.0.0.1")) {
		t.Errorf("FilterIP not set correctly")
	}

	// Verify OutputJSON is bool, not string
	if !config.OutputJSON {
		t.Errorf("OutputJSON should be true")
	}
}

func TestHostNetworkPodForTrace(t *testing.T) {
	config := TraceConfig{
		RetinaShellImage: "mcr.microsoft.com/containernetworking/retina-shell:v1.0.0",
	}

	pod := hostNetworkPodForTrace(config, "kube-system", "node-001")

	// Test basic pod properties
	t.Run("namespace", func(t *testing.T) {
		if pod.Namespace != "kube-system" {
			t.Errorf("Expected namespace kube-system, got %s", pod.Namespace)
		}
	})

	t.Run("node selector", func(t *testing.T) {
		if pod.Spec.NodeName != "node-001" {
			t.Errorf("Expected nodeName node-001, got %s", pod.Spec.NodeName)
		}
	})

	t.Run("host network enabled", func(t *testing.T) {
		if !pod.Spec.HostNetwork {
			t.Error("Expected HostNetwork to be true")
		}
	})

	t.Run("host PID enabled", func(t *testing.T) {
		if !pod.Spec.HostPID {
			t.Error("Expected HostPID to be true for bpftrace")
		}
	})

	t.Run("tolerates all taints", func(t *testing.T) {
		found := false
		for _, tol := range pod.Spec.Tolerations {
			if tol.Operator == v1.TolerationOpExists {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected toleration with Operator=Exists to run on any node")
		}
	})

	t.Run("restart policy never", func(t *testing.T) {
		if pod.Spec.RestartPolicy != v1.RestartPolicyNever {
			t.Errorf("Expected RestartPolicy Never, got %s", pod.Spec.RestartPolicy)
		}
	})

	t.Run("image set correctly", func(t *testing.T) {
		if len(pod.Spec.Containers) != 1 {
			t.Fatalf("Expected 1 container, got %d", len(pod.Spec.Containers))
		}
		if pod.Spec.Containers[0].Image != config.RetinaShellImage {
			t.Errorf("Expected image %s, got %s", config.RetinaShellImage, pod.Spec.Containers[0].Image)
		}
	})

	t.Run("not interactive", func(t *testing.T) {
		container := pod.Spec.Containers[0]
		if container.Stdin {
			t.Error("Expected Stdin to be false for non-interactive trace")
		}
		if container.TTY {
			t.Error("Expected TTY to be false for non-interactive trace")
		}
	})

	t.Run("has trace labels", func(t *testing.T) {
		if pod.Labels["app"] != "retina-trace" {
			t.Errorf("Expected label app=retina-trace, got %s", pod.Labels["app"])
		}
		if pod.Labels["retina.sh/trace-target-node"] != "node-001" {
			t.Errorf("Expected node label, got %s", pod.Labels["retina.sh/trace-target-node"])
		}
	})
}

func TestHostNetworkPodForTraceSecurityContext(t *testing.T) {
	config := TraceConfig{
		RetinaShellImage: "test-image:v1",
	}

	pod := hostNetworkPodForTrace(config, "default", "test-node")
	container := pod.Spec.Containers[0]
	secCtx := container.SecurityContext

	t.Run("not privileged", func(t *testing.T) {
		if secCtx.Privileged != nil && *secCtx.Privileged {
			t.Error("Pod should use capabilities, not privileged mode")
		}
	})

	t.Run("drops all capabilities", func(t *testing.T) {
		if secCtx.Capabilities == nil {
			t.Fatal("Expected Capabilities to be set")
		}

		foundDropAll := false
		for _, drop := range secCtx.Capabilities.Drop {
			if string(drop) == "ALL" {
				foundDropAll = true
				break
			}
		}
		if !foundDropAll {
			t.Error("Expected to drop ALL capabilities first")
		}
	})

	t.Run("adds required capabilities", func(t *testing.T) {
		if secCtx.Capabilities == nil {
			t.Fatal("Expected Capabilities to be set")
		}

		addedCaps := make(map[string]bool)
		for _, cap := range secCtx.Capabilities.Add {
			addedCaps[string(cap)] = true
		}

		requiredCaps := TraceCapabilities()
		for _, required := range requiredCaps {
			if !addedCaps[required] {
				t.Errorf("Missing required capability: %s", required)
			}
		}
	})

	t.Run("seccomp unconfined", func(t *testing.T) {
		if secCtx.SeccompProfile == nil {
			t.Fatal("Expected SeccompProfile to be set")
		}
		if secCtx.SeccompProfile.Type != v1.SeccompProfileTypeUnconfined {
			t.Errorf("Expected Seccomp Unconfined, got %s", secCtx.SeccompProfile.Type)
		}
	})
}

func TestRandomTraceContainerName(t *testing.T) {
	name1 := randomTraceContainerName()
	name2 := randomTraceContainerName()

	t.Run("has prefix", func(t *testing.T) {
		if len(name1) < 14 || name1[:13] != "retina-trace-" {
			t.Errorf("Expected prefix 'retina-trace-', got %s", name1)
		}
	})

	t.Run("unique names", func(t *testing.T) {
		if name1 == name2 {
			t.Errorf("Expected unique names, got %s and %s", name1, name2)
		}
	})
}

func TestBoolPtr(t *testing.T) {
	truePtr := boolPtr(true)
	falsePtr := boolPtr(false)

	if truePtr == nil || *truePtr != true {
		t.Error("boolPtr(true) should return pointer to true")
	}
	if falsePtr == nil || *falsePtr != false {
		t.Error("boolPtr(false) should return pointer to false")
	}
}
