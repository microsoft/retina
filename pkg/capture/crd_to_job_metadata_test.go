// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"

	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/capture/file"
)

// jobEnvByNode returns each rendered Job's environment, keyed by the node it is pinned to.
func jobEnvByNode(t *testing.T, jobs []*batchv1.Job) map[string]map[string]string {
	t.Helper()

	byNode := make(map[string]map[string]string, len(jobs))
	for _, job := range jobs {
		env := map[string]string{}
		for _, e := range job.Spec.Template.Spec.Containers[0].Env {
			env[e.Name] = e.Value
		}
		byNode[env[captureConstants.NodeHostNameEnvKey]] = env
	}
	return byNode
}

// Network metadata describes the whole node (routes, iptables, all sockets, /proc/net on Linux; the collectlogs.ps1
// bundle on Windows), so a capture limited to Pods must not collect it, whatever the Capture requested.
func TestRenderJobCollectsMetadataOnlyForNodeTargets(t *testing.T) {
	ctx, cancel := TestContext(t)
	defer cancel()

	cases := []struct {
		name         string
		targets      *CaptureTargetsOnNode
		includeMeta  string
		wantMetaByNd map[string]string
	}{
		{
			name:         "linux Pod target with metadata requested",
			targets:      &CaptureTargetsOnNode{"node1": {OS: "linux", PodIpAddresses: []string{"10.244.1.7"}}},
			includeMeta:  "true",
			wantMetaByNd: map[string]string{"node1": "false"},
		},
		{
			name:         "windows Pod target with metadata requested",
			targets:      &CaptureTargetsOnNode{"winnode1": {OS: "windows", PodIpAddresses: []string{"10.244.2.7"}}},
			includeMeta:  "true",
			wantMetaByNd: map[string]string{"winnode1": "false"},
		},
		{
			name:         "linux node target with metadata requested",
			targets:      &CaptureTargetsOnNode{"node1": {OS: "linux", CaptureNodeInterface: true}},
			includeMeta:  "true",
			wantMetaByNd: map[string]string{"node1": "true"},
		},
		{
			name:         "windows node target with metadata requested",
			targets:      &CaptureTargetsOnNode{"winnode1": {OS: "windows", CaptureNodeInterface: true}},
			includeMeta:  "true",
			wantMetaByNd: map[string]string{"winnode1": "true"},
		},
		{
			name:         "node target with metadata not requested",
			targets:      &CaptureTargetsOnNode{"node1": {OS: "linux", CaptureNodeInterface: true}},
			includeMeta:  "false",
			wantMetaByNd: map[string]string{"node1": "false"},
		},
		{
			name: "decision is per target",
			targets: &CaptureTargetsOnNode{
				"node1": {OS: "linux", PodIpAddresses: []string{"10.244.1.7"}},
				"node2": {OS: "linux", CaptureNodeInterface: true},
			},
			includeMeta:  "true",
			wantMetaByNd: map[string]string{"node1": "false", "node2": "true"},
		},
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

			jobs, err := translator.renderJob(tt.targets, map[string]string{captureConstants.IncludeMetadataEnvKey: tt.includeMeta})
			require.NoError(t, err)

			envByNode := jobEnvByNode(t, jobs)
			require.Len(t, envByNode, len(tt.wantMetaByNd))
			for node, want := range tt.wantMetaByNd {
				require.Equal(t, want, envByNode[node][captureConstants.IncludeMetadataEnvKey], "node %s", node)
			}
		})
	}
}

// End to end through TranslateCaptureToJobs, which both the operator and `kubectl retina capture create` call.
func TestTranslateCaptureToJobsMetadataFollowsTarget(t *testing.T) {
	ctx, cancel := TestContext(t)
	defer cancel()

	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name:   "node1",
		Labels: map[string]string{corev1.LabelHostname: "node1", corev1.LabelOSStable: "linux"},
	}}
	webLabels := map[string]string{"app": "web"}
	web := podForTest("tenant-a", "web-0", webLabels, false, "10.244.1.7")
	web.Status.Phase = corev1.PodRunning

	cases := []struct {
		name     string
		target   retinav1alpha1.CaptureTarget
		wantMeta string
	}{
		{
			name:     "pod-scoped Capture with includeMetadata=true collects no metadata",
			target:   retinav1alpha1.CaptureTarget{PodSelector: &metav1.LabelSelector{MatchLabels: webLabels}},
			wantMeta: "false",
		},
		{
			name:     "podNames Capture with includeMetadata=true collects no metadata",
			target:   retinav1alpha1.CaptureTarget{PodNames: []string{"web-0"}},
			wantMeta: "false",
		},
		{
			name:     "node-wide Capture with includeMetadata=true collects metadata",
			target:   retinav1alpha1.CaptureTarget{NodeSelector: &metav1.LabelSelector{MatchLabels: map[string]string{corev1.LabelHostname: "node1"}}},
			wantMeta: "true",
		},
	}

	for i := range cases {
		tt := &cases[i]
		t.Run(tt.name, func(t *testing.T) {
			translator := NewCaptureToPodTranslatorForTest(fakeclientset.NewSimpleClientset(node, web))
			jobs, err := translator.TranslateCaptureToJobs(ctx, &retinav1alpha1.Capture{
				ObjectMeta: metav1.ObjectMeta{Name: "c", Namespace: "tenant-a"},
				Status:     retinav1alpha1.CaptureStatus{StartTime: file.Now()},
				Spec: retinav1alpha1.CaptureSpec{
					CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
						CaptureTarget:   tt.target,
						IncludeMetadata: true,
						CaptureOption:   retinav1alpha1.CaptureOption{Duration: &metav1.Duration{Duration: time.Minute}},
					},
					OutputConfiguration: retinav1alpha1.OutputConfiguration{HostPath: ptr.To("capture")},
				},
			})
			require.NoError(t, err)
			require.Len(t, jobs, 1)
			require.Equal(t, tt.wantMeta, jobEnvByNode(t, jobs)["node1"][captureConstants.IncludeMetadataEnvKey])
		})
	}
}
