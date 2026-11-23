// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"

	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/capture/file"
	captureUtils "github.com/microsoft/retina/pkg/capture/utils"
)

const nodeIPForTest = "10.224.0.4"

func podForTest(namespace, name string, podLabels map[string]string, hostNetwork bool, ip string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: podLabels},
		Spec:       corev1.PodSpec{NodeName: "node1", HostNetwork: hostNetwork},
		Status:     corev1.PodStatus{PodIP: ip, PodIPs: []corev1.PodIP{{IP: ip}}},
	}
}

// A hostNetwork Pod's IP is the node's IP, so a Pod target on it would capture the node's own traffic. Retina's
// own capture Pods are hostNetwork and live in the Capture's namespace, so a tenant can always select one.
func TestPodTargetsExcludeHostNetworkPods(t *testing.T) {
	ctx, cancel := TestContext(t)
	defer cancel()

	capturePodLabels := captureUtils.GetContainerLabelsFromCaptureName("earlier-capture")
	webLabels := map[string]string{"app": "web"}

	cases := []struct {
		name        string
		target      retinav1alpha1.CaptureTarget
		objects     []runtime.Object
		wantTargets *CaptureTargetsOnNode
		// wantErrPods lists the Pods named in the errHostNetworkPodTargets error; empty means no error.
		wantErrPods []string
	}{
		{
			name:        "podSelector matching only a Retina capture Pod is rejected",
			target:      retinav1alpha1.CaptureTarget{PodSelector: &metav1.LabelSelector{MatchLabels: capturePodLabels}},
			objects:     []runtime.Object{podForTest("tenant-a", "earlier-capture-x1", capturePodLabels, true, nodeIPForTest)},
			wantErrPods: []string{"tenant-a/earlier-capture-x1"},
		},
		{
			name:   "podSelector matching hostNetwork and regular Pods keeps only the regular Pod",
			target: retinav1alpha1.CaptureTarget{PodSelector: &metav1.LabelSelector{MatchLabels: webLabels}},
			objects: []runtime.Object{
				podForTest("tenant-a", "web-host", webLabels, true, nodeIPForTest),
				podForTest("tenant-a", "web-0", webLabels, false, "10.244.1.7"),
			},
			wantTargets: &CaptureTargetsOnNode{"node1": CaptureTarget{PodIpAddresses: []string{"10.244.1.7"}}},
		},
		{
			name: "podSelector with namespaceSelector excludes hostNetwork Pods in other namespaces",
			target: retinav1alpha1.CaptureTarget{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"team": "b"}},
				PodSelector:       &metav1.LabelSelector{MatchLabels: webLabels},
			},
			objects: []runtime.Object{
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "tenant-b", Labels: map[string]string{"team": "b"}}},
				podForTest("tenant-b", "web-host", webLabels, true, nodeIPForTest),
			},
			wantErrPods: []string{"tenant-b/web-host"},
		},
		{
			name:        "podNames naming only a hostNetwork Pod is rejected",
			target:      retinav1alpha1.CaptureTarget{PodNames: []string{"earlier-capture-x1"}},
			objects:     []runtime.Object{podForTest("tenant-a", "earlier-capture-x1", capturePodLabels, true, nodeIPForTest)},
			wantErrPods: []string{"tenant-a/earlier-capture-x1"},
		},
		{
			name:   "podNames naming hostNetwork and regular Pods keeps only the regular Pod",
			target: retinav1alpha1.CaptureTarget{PodNames: []string{"web-host", "web-0"}},
			objects: []runtime.Object{
				podForTest("tenant-a", "web-host", webLabels, true, nodeIPForTest),
				podForTest("tenant-a", "web-0", webLabels, false, "10.244.1.7"),
			},
			wantTargets: &CaptureTargetsOnNode{"node1": CaptureTarget{PodIpAddresses: []string{"10.244.1.7"}}},
		},
	}

	for i := range cases {
		tt := &cases[i]
		t.Run(tt.name, func(t *testing.T) {
			translator := NewCaptureToPodTranslatorForTest(fakeclientset.NewSimpleClientset(tt.objects...))
			got, err := translator.getCaptureTargetsOnNode(ctx, tt.target, "tenant-a")
			if len(tt.wantErrPods) > 0 {
				require.ErrorIs(t, err, errHostNetworkPodTargets)
				for _, pod := range tt.wantErrPods {
					require.ErrorContains(t, err, pod)
				}
				require.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantTargets, got)
		})
	}
}

// The error must surface from TranslateCaptureToJobs, which is what sets the Capture's error status in the operator
// and fails `kubectl retina capture create` in the CLI, and no Job may be rendered for the node.
func TestTranslateCaptureToJobsRejectsHostNetworkPodTarget(t *testing.T) {
	ctx, cancel := TestContext(t)
	defer cancel()

	capturePodLabels := captureUtils.GetContainerLabelsFromCaptureName("earlier-capture")
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name:   "node1",
		Labels: map[string]string{corev1.LabelHostname: "node1", corev1.LabelOSStable: "linux"},
	}}
	translator := NewCaptureToPodTranslatorForTest(fakeclientset.NewSimpleClientset(
		node, podForTest("tenant-a", "earlier-capture-x1", capturePodLabels, true, nodeIPForTest),
	))

	capture := &retinav1alpha1.Capture{
		ObjectMeta: metav1.ObjectMeta{Name: "node-via-pod", Namespace: "tenant-a"},
		Status:     retinav1alpha1.CaptureStatus{StartTime: file.Now()},
		Spec: retinav1alpha1.CaptureSpec{
			CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
				CaptureTarget: retinav1alpha1.CaptureTarget{PodSelector: &metav1.LabelSelector{MatchLabels: capturePodLabels}},
				CaptureOption: retinav1alpha1.CaptureOption{Duration: &metav1.Duration{Duration: time.Minute}},
			},
			OutputConfiguration: retinav1alpha1.OutputConfiguration{HostPath: ptr.To("capture")},
		},
	}

	jobs, err := translator.TranslateCaptureToJobs(ctx, capture)
	require.ErrorIs(t, err, errHostNetworkPodTargets)
	require.ErrorContains(t, err, "tenant-a/earlier-capture-x1")
	require.Empty(t, jobs)
}

// A Pod target without IPs would leave the capture filter unrestricted, capturing the whole node. Pods that aren't
// scheduled or have no IP yet (e.g. Pending while ContainerCreating) must not become targets.
func TestPodTargetsExcludePodsWithoutIPs(t *testing.T) {
	ctx, cancel := TestContext(t)
	defer cancel()

	webLabels := map[string]string{"app": "web"}
	noIPPod := func(name, nodeName string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "tenant-a", Labels: webLabels},
			Spec:       corev1.PodSpec{NodeName: nodeName},
			Status:     corev1.PodStatus{Phase: corev1.PodPending},
		}
	}

	cases := []struct {
		name        string
		target      retinav1alpha1.CaptureTarget
		objects     []runtime.Object
		wantTargets *CaptureTargetsOnNode
		wantErr     error
		wantErrPods []string
	}{
		{
			name:        "podSelector matching only a scheduled Pod without IPs is rejected",
			target:      retinav1alpha1.CaptureTarget{PodSelector: &metav1.LabelSelector{MatchLabels: webLabels}},
			objects:     []runtime.Object{noIPPod("pinned", "node1")},
			wantErr:     errPodTargetsNotRunning,
			wantErrPods: []string{"tenant-a/pinned"},
		},
		{
			name:        "podSelector matching only an unscheduled Pod is rejected",
			target:      retinav1alpha1.CaptureTarget{PodSelector: &metav1.LabelSelector{MatchLabels: webLabels}},
			objects:     []runtime.Object{noIPPod("pending", "")},
			wantErr:     errPodTargetsNotRunning,
			wantErrPods: []string{"tenant-a/pending"},
		},
		{
			name:   "podSelector matching a Pod without a node is rejected even if it reports IPs",
			target: retinav1alpha1.CaptureTarget{PodSelector: &metav1.LabelSelector{MatchLabels: webLabels}},
			objects: []runtime.Object{&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "unbound", Namespace: "tenant-a", Labels: webLabels},
				Status:     corev1.PodStatus{PodIPs: []corev1.PodIP{{IP: "10.244.1.8"}}},
			}},
			wantErr:     errPodTargetsNotRunning,
			wantErrPods: []string{"tenant-a/unbound"},
		},
		{
			name:   "podSelector matching Pods with and without IPs keeps only the Pod with IPs",
			target: retinav1alpha1.CaptureTarget{PodSelector: &metav1.LabelSelector{MatchLabels: webLabels}},
			objects: []runtime.Object{
				noIPPod("pinned", "node2"),
				noIPPod("pending", ""),
				podForTest("tenant-a", "web-0", webLabels, false, "10.244.1.7"),
			},
			wantTargets: &CaptureTargetsOnNode{"node1": CaptureTarget{PodIpAddresses: []string{"10.244.1.7"}}},
		},
		{
			name:        "podNames naming only a Pod without IPs is rejected",
			target:      retinav1alpha1.CaptureTarget{PodNames: []string{"pinned"}},
			objects:     []runtime.Object{noIPPod("pinned", "node1")},
			wantErr:     errPodTargetsNotRunning,
			wantErrPods: []string{"tenant-a/pinned"},
		},
		{
			name:   "podNames naming Pods with and without IPs keeps only the Pod with IPs",
			target: retinav1alpha1.CaptureTarget{PodNames: []string{"pinned", "web-0"}},
			objects: []runtime.Object{
				noIPPod("pinned", "node2"),
				podForTest("tenant-a", "web-0", webLabels, false, "10.244.1.7"),
			},
			wantTargets: &CaptureTargetsOnNode{"node1": CaptureTarget{PodIpAddresses: []string{"10.244.1.7"}}},
		},
		{
			name:   "only hostNetwork and IP-less Pods reports the hostNetwork error",
			target: retinav1alpha1.CaptureTarget{PodSelector: &metav1.LabelSelector{MatchLabels: webLabels}},
			objects: []runtime.Object{
				noIPPod("pinned", "node1"),
				podForTest("tenant-a", "web-host", webLabels, true, nodeIPForTest),
			},
			wantErr:     errHostNetworkPodTargets,
			wantErrPods: []string{"tenant-a/web-host"},
		},
	}

	for i := range cases {
		tt := &cases[i]
		t.Run(tt.name, func(t *testing.T) {
			translator := NewCaptureToPodTranslatorForTest(fakeclientset.NewSimpleClientset(tt.objects...))
			got, err := translator.getCaptureTargetsOnNode(ctx, tt.target, "tenant-a")
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				for _, pod := range tt.wantErrPods {
					require.ErrorContains(t, err, pod)
				}
				require.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantTargets, got)
		})
	}
}

// renderJob is the last line of defence: whatever the resolvers do, a Pod target without IPs must never become a
// Job, on Linux (tcpdump filter) or Windows (netsh filter). Node targets legitimately have no Pod IPs.
func TestRenderJobRejectsPodTargetWithoutIPs(t *testing.T) {
	ctx, cancel := TestContext(t)
	defer cancel()

	cases := []struct {
		name    string
		targets *CaptureTargetsOnNode
		wantErr bool
	}{
		{name: "linux Pod target without IPs", targets: &CaptureTargetsOnNode{"node1": {OS: "linux"}}, wantErr: true},
		{name: "windows Pod target without IPs", targets: &CaptureTargetsOnNode{"winnode1": {OS: "windows"}}, wantErr: true},
		{
			name: "one bad target among valid ones",
			targets: &CaptureTargetsOnNode{
				"node1": {OS: "linux", PodIpAddresses: []string{"10.244.1.7"}},
				"node2": {OS: "linux"},
			},
			wantErr: true,
		},
		{name: "linux Pod target with IPs", targets: &CaptureTargetsOnNode{"node1": {OS: "linux", PodIpAddresses: []string{"10.244.1.7"}}}},
		{name: "linux node target", targets: &CaptureTargetsOnNode{"node1": {OS: "linux", CaptureNodeInterface: true}}},
		{name: "windows node target", targets: &CaptureTargetsOnNode{"winnode1": {OS: "windows", CaptureNodeInterface: true}}},
	}

	for i := range cases {
		tt := &cases[i]
		t.Run(tt.name, func(t *testing.T) {
			translator := NewCaptureToPodTranslatorForTest(fakeclientset.NewSimpleClientset())
			require.NoError(t, translator.initJobTemplate(ctx, &retinav1alpha1.Capture{
				ObjectMeta: metav1.ObjectMeta{Name: "c", Namespace: "tenant-a"},
				Spec:       retinav1alpha1.CaptureSpec{OutputConfiguration: retinav1alpha1.OutputConfiguration{HostPath: ptr.To("capture")}},
				Status:     retinav1alpha1.CaptureStatus{StartTime: file.Now()},
			}, "/tmp/capture"))

			jobs, err := translator.renderJob(tt.targets, map[string]string{})
			if tt.wantErr {
				require.ErrorIs(t, err, errPodTargetWithoutIPs)
				require.Empty(t, jobs)
				return
			}
			require.NoError(t, err)
			require.Len(t, jobs, len(*tt.targets))
		})
	}
}

// A completed Pod may still report its last IP after the IP was released and reassigned to another Pod, possibly in
// another namespace, so terminal Pods must not become targets. Pending Pods that already have an IP are fine.
func TestPodTargetsExcludeTerminalPods(t *testing.T) {
	ctx, cancel := TestContext(t)
	defer cancel()

	webLabels := map[string]string{"app": "web"}
	podInPhase := func(name string, phase corev1.PodPhase, ip string) *corev1.Pod {
		pod := podForTest("tenant-a", name, webLabels, false, ip)
		pod.Status.Phase = phase
		return pod
	}
	running := podInPhase("web-0", corev1.PodRunning, "10.244.1.7")

	cases := []struct {
		name        string
		target      retinav1alpha1.CaptureTarget
		objects     []runtime.Object
		wantTargets *CaptureTargetsOnNode
		wantErrPods []string
	}{
		{
			name:        "podSelector matching only a Succeeded Pod with a stale IP is rejected",
			target:      retinav1alpha1.CaptureTarget{PodSelector: &metav1.LabelSelector{MatchLabels: webLabels}},
			objects:     []runtime.Object{podInPhase("job-done", corev1.PodSucceeded, "10.244.1.9")},
			wantErrPods: []string{"tenant-a/job-done"},
		},
		{
			name:        "podSelector matching only a Failed Pod with a stale IP is rejected",
			target:      retinav1alpha1.CaptureTarget{PodSelector: &metav1.LabelSelector{MatchLabels: webLabels}},
			objects:     []runtime.Object{podInPhase("crashed", corev1.PodFailed, "10.244.1.9")},
			wantErrPods: []string{"tenant-a/crashed"},
		},
		{
			name:   "podSelector matching terminal and running Pods keeps only the running Pod",
			target: retinav1alpha1.CaptureTarget{PodSelector: &metav1.LabelSelector{MatchLabels: webLabels}},
			objects: []runtime.Object{
				podInPhase("job-done", corev1.PodSucceeded, "10.244.1.9"),
				podInPhase("crashed", corev1.PodFailed, "10.244.1.10"),
				running,
			},
			wantTargets: &CaptureTargetsOnNode{"node1": CaptureTarget{PodIpAddresses: []string{"10.244.1.7"}}},
		},
		{
			name:        "podNames naming only a Succeeded Pod with a stale IP is rejected",
			target:      retinav1alpha1.CaptureTarget{PodNames: []string{"job-done"}},
			objects:     []runtime.Object{podInPhase("job-done", corev1.PodSucceeded, "10.244.1.9")},
			wantErrPods: []string{"tenant-a/job-done"},
		},
		{
			name:        "podNames naming terminal and running Pods keeps only the running Pod",
			target:      retinav1alpha1.CaptureTarget{PodNames: []string{"job-done", "web-0"}},
			objects:     []runtime.Object{podInPhase("job-done", corev1.PodSucceeded, "10.244.1.9"), running},
			wantTargets: &CaptureTargetsOnNode{"node1": CaptureTarget{PodIpAddresses: []string{"10.244.1.7"}}},
		},
		{
			name:        "Pending Pod that already has an IP is a valid target",
			target:      retinav1alpha1.CaptureTarget{PodNames: []string{"starting"}},
			objects:     []runtime.Object{podInPhase("starting", corev1.PodPending, "10.244.1.8")},
			wantTargets: &CaptureTargetsOnNode{"node1": CaptureTarget{PodIpAddresses: []string{"10.244.1.8"}}},
		},
	}

	for i := range cases {
		tt := &cases[i]
		t.Run(tt.name, func(t *testing.T) {
			translator := NewCaptureToPodTranslatorForTest(fakeclientset.NewSimpleClientset(tt.objects...))
			got, err := translator.getCaptureTargetsOnNode(ctx, tt.target, "tenant-a")
			if len(tt.wantErrPods) > 0 {
				require.ErrorIs(t, err, errPodTargetsNotRunning)
				for _, pod := range tt.wantErrPods {
					require.ErrorContains(t, err, pod)
				}
				require.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantTargets, got)
		})
	}
}

// The targeting errors end up in the Capture's status condition message, which the CRD caps at 32768 characters;
// a longer message would make the status update fail, so the error would never be recorded.
func TestSkippedPodErrorsStayWithinStatusMessageLimit(t *testing.T) {
	ctx, cancel := TestContext(t)
	defer cancel()

	const statusMessageMaxLength = 32768
	const podCount = 2000
	webLabels := map[string]string{"app": "web"}

	for _, tc := range []struct {
		name    string
		mkPod   func(i int) *corev1.Pod
		wantErr error
	}{
		{
			name: "hostNetwork Pods",
			mkPod: func(i int) *corev1.Pod {
				return podForTest("tenant-a", fmt.Sprintf("kube-proxy-%05d", i), webLabels, true, nodeIPForTest)
			},
			wantErr: errHostNetworkPodTargets,
		},
		{
			name: "not running Pods",
			mkPod: func(i int) *corev1.Pod {
				pod := podForTest("tenant-a", fmt.Sprintf("job-done-%05d", i), webLabels, false, "10.244.1.9")
				pod.Status.Phase = corev1.PodSucceeded
				return pod
			},
			wantErr: errPodTargetsNotRunning,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			objects := make([]runtime.Object, 0, podCount)
			for i := range podCount {
				objects = append(objects, tc.mkPod(i))
			}
			translator := NewCaptureToPodTranslatorForTest(fakeclientset.NewSimpleClientset(objects...))

			_, err := translator.getCaptureTargetsOnNode(ctx, retinav1alpha1.CaptureTarget{
				PodSelector: &metav1.LabelSelector{MatchLabels: webLabels},
			}, "tenant-a")
			require.ErrorIs(t, err, tc.wantErr)
			require.Less(t, len(err.Error()), statusMessageMaxLength)
			require.ErrorContains(t, err, fmt.Sprintf(", and %d more", podCount-maxPodsInError))
		})
	}
}

func TestSummarizePods(t *testing.T) {
	pods := func(n int) []string {
		out := make([]string, 0, n)
		for i := range n {
			out = append(out, fmt.Sprintf("ns/p%d", i))
		}
		return out
	}
	require.Equal(t, "ns/p0", summarizePods(pods(1)))
	require.Equal(t, "ns/p0, ns/p1, ns/p2, ns/p3, ns/p4, ns/p5, ns/p6, ns/p7, ns/p8, ns/p9", summarizePods(pods(maxPodsInError)))
	require.Equal(t, "ns/p0, ns/p1, ns/p2, ns/p3, ns/p4, ns/p5, ns/p6, ns/p7, ns/p8, ns/p9, and 1 more", summarizePods(pods(maxPodsInError+1)))
}
