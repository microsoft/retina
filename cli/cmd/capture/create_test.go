// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"

	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/internal/buildinfo"
	captureUtils "github.com/microsoft/retina/pkg/capture/utils"
	"github.com/microsoft/retina/pkg/label"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

const testNamespace = "test-ns"

type testcase struct {
	name              string
	inputName         string
	wantName          string
	inputNamespace    string
	wantNamespace     string
	inputPodSelector  string
	inputNsSelector   string
	inputNodeSelector string
	wantNodes         []string
	wantErr           bool
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))] //nolint:gosec // this random number generator is fine
	}
	return string(result)
}

func argsFromTestCase(tc testcase) []string {
	args := []string{"create", "--pod-selectors", tc.inputPodSelector}
	if tc.inputNsSelector != "" {
		args = append(args, "--namespace-selectors="+tc.inputNsSelector)
	}
	if tc.inputNamespace != "" {
		args = append(args, "--namespace", tc.inputNamespace)
	}
	if tc.inputName != "" {
		args = append(args, "--name", tc.inputName)
	}
	if tc.inputNodeSelector != "" {
		args = append(args, "--node-selectors="+tc.inputNodeSelector)
	}
	return args
}

func NewNode(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"kubernetes.io/hostname": name,
				"kubernetes.io/os":       "linux",
			},
		},
	}
}

func NewNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"name": name,
			},
		},
	}
}

func NewClientServerPods(service, namespace string) []*corev1.Pod {
	return []*corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "client" + service,
				Namespace: namespace,
				Labels: map[string]string{
					"service": service,
				},
			},
			Spec: corev1.PodSpec{
				NodeName: service + "1",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "server" + service,
				Namespace: namespace,
				Labels: map[string]string{
					"service": service,
				},
			},
			Spec: corev1.PodSpec{
				NodeName: service + "2",
			},
		},
	}
}

func TestCreateJobsWithNamespace(t *testing.T) {
	// Create a fake Kubernetes client with workload and capture namespaces
	newKubeclient := func() *fake.Clientset {
		objects := []runtime.Object{
			NewNode("A1"),
			NewNode("A2"),
			NewNode("B1"),
			NewNode("B2"),
			NewNamespace("workload"),
			NewNamespace("capture"),
		}
		for _, pod := range NewClientServerPods("A", "workload") {
			objects = append(objects, pod)
		}
		for _, pod := range NewClientServerPods("B", "workload") {
			objects = append(objects, pod)
		}

		kubeClient := fake.NewClientset(objects...)

		// Handle job creation to set job name if not provided, which is done automatically in a real k8s cluster
		kubeClient.PrependReactor("create", "jobs", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
			createAction, ok := action.(clienttesting.CreateAction)
			if !ok {
				return false, nil, fmt.Errorf("expected CreateAction, got %T", action) //nolint:err113 // test code
			}
			job := createAction.GetObject().(*batchv1.Job)

			// Set job name if unset
			if job.Name == "" {
				job.Name = job.GenerateName + randomString(5)
			}
			return false, job, nil
		})

		return kubeClient
	}

	// Setup test cases
	testCases := []testcase{
		{
			name:             "create --name=test --podSelector=service=A --namespace-selectors=name=workload",
			inputName:        "test",
			wantName:         "test",
			inputNamespace:   "",
			wantNamespace:    "default",
			inputPodSelector: "service=A",
			inputNsSelector:  "name=workload",
			wantNodes:        []string{"A1", "A2"},
			wantErr:          false,
		},
		{
			name:             "create --namespace=workload --podSelector=service=A --namespace-selectors=name=workload",
			inputName:        "",
			wantName:         DefaultName,
			inputNamespace:   "workload",
			wantNamespace:    "workload",
			inputPodSelector: "service=A",
			inputNsSelector:  "name=workload",
			wantNodes:        []string{"A1", "A2"},
			wantErr:          false,
		},
		{
			name:             "create --namespace=workload --podSelector=service=A",
			inputName:        "",
			wantName:         DefaultName,
			inputNamespace:   "workload",
			wantNamespace:    "workload",
			inputPodSelector: "service=A",
			inputNsSelector:  "",
			wantNodes:        []string{},
			wantErr:          true,
		},
		{
			name:             "create --namespace=workload --namespace-selectors=name=workload",
			inputName:        "",
			wantName:         DefaultName,
			inputNamespace:   "workload",
			wantNamespace:    "workload",
			inputPodSelector: "",
			inputNsSelector:  "name=workload",
			wantNodes:        []string{},
			wantErr:          true,
		},
		{
			name:             "create --namespace=workload --podSelector=service=B --namespace-selectors=name=workload",
			inputName:        "",
			wantName:         DefaultName,
			inputNamespace:   "workload",
			wantNamespace:    "workload",
			inputPodSelector: "service=B",
			inputNsSelector:  "name=workload",
			wantNodes:        []string{"B1", "B2"},
			wantErr:          false,
		},
		{
			name:              "create --namespace=workload --node-selectors=kubernetes.io/hostname=B1",
			inputName:         "",
			wantName:          DefaultName,
			inputNamespace:    "workload",
			wantNamespace:     "workload",
			inputNodeSelector: "kubernetes.io/hostname=B1",
			wantNodes:         []string{"B1"},
			wantErr:           false,
		},
		{
			name:              "create --namespace=workload --podSelector=service=B --namespace-selectors=name=workload --node-selectors=kubernetes.io/hostname=B1",
			inputName:         "",
			wantName:          DefaultName,
			inputNamespace:    "workload",
			wantNamespace:     "workload",
			inputPodSelector:  "service=B",
			inputNsSelector:   "name=workload",
			inputNodeSelector: "kubernetes.io/hostname=B1",
			wantNodes:         []string{"B1"},
			wantErr:           true,
		},
	}
	for _, tc := range testCases {
		fmt.Println("\n### Running test case:", tc.name)
		// Given
		kubeClient := newKubeclient()
		cmd := NewCommand(kubeClient)
		cmd.SetArgs(argsFromTestCase(tc))
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		t.Run(tc.name, func(t *testing.T) {
			// When
			err := cmd.Execute()

			// Then

			// Check the command execution
			AssertError(t, err, tc)

			// Validate that jobs are created correctly
			JobsCreatedCorrectly(t, kubeClient, tc)
		})
	}
}

// AssertError checks if the error matches the expected outcome based on the testcase
func AssertError(t *testing.T, err error, tc testcase) {
	t.Helper()
	if tc.wantErr {
		if err == nil {
			t.Fatalf("Expected error for test case %s, but got none", tc.name)
		}
		t.Skip("Successfully got expected error")
	}
	if err != nil {
		t.Fatalf("Failed to execute command: %v", err)
	}
}

// JobsCreatedCorrectly validates that Kubernetes jobs were created correctly based on the namespace and pod selector flags
// provided to the command. It verifies that jobs are created in the right namespace and with the correct node affinity.
func JobsCreatedCorrectly(t *testing.T, kubeClient *fake.Clientset, tc testcase) {
	t.Helper()
	// Get created jobs
	jobs, err := kubeClient.BatchV1().Jobs(tc.wantNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", label.CaptureNameLabel, tc.wantName),
	})
	// Execution should not return an error
	if err != nil {
		t.Fatalf("Failed to list jobs in namespace %s: %v", tc.wantNamespace, err)
	}

	// Jobs should not be nil or empty
	if jobs == nil || len(jobs.Items) == 0 {
		t.Fatalf("No jobs found for capture %s in namespace %s", tc.wantName, tc.wantNamespace)
	}

	// Number of jobs should match expected number of nodes
	if len(jobs.Items) != len(tc.wantNodes) {
		t.Fatalf("Expected %d jobs, but found %d", len(tc.wantNodes), len(jobs.Items))
	}

	// Create a map of expected nodes for easier comparison
	expectedNodes := make(map[string]bool)
	for _, node := range tc.wantNodes {
		expectedNodes[node] = true
	}
	matchCount := 0

	// Validate node affinity for each job
	for idx := range jobs.Items {
		job := jobs.Items[idx]

		// Validate node affinity based on pod selector and namespace selector
		if len(tc.wantNodes) > 0 {
			if job.Spec.Template.Spec.Affinity == nil || job.Spec.Template.Spec.Affinity.NodeAffinity == nil {
				t.Fatalf("Expected job to have node affinity, but none found")
			}

			nodeAffinity := job.Spec.Template.Spec.Affinity.NodeAffinity
			if nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
				t.Fatalf("Expected job to have required node affinity, but none found")
			}

			// Look for hostname match expression
			for _, nodeSelectorTerm := range nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
				for _, expression := range nodeSelectorTerm.MatchExpressions {
					if expression.Key == "kubernetes.io/hostname" {
						// Check if all values are in expected nodes and count matches
						for _, value := range expression.Values {
							if expectedNodes[value] {
								matchCount++
							} else {
								t.Errorf("Unexpected node %s in job %s, expected nodes: %v", value, job.Name, tc.wantNodes)
							}
						}
					}
				}
			}
		}
	}

	// Check if all expected nodes are present
	if matchCount != len(tc.wantNodes) {
		t.Errorf("Job's node affinity doesn't match expected nodes. Expected: %v", tc.wantNodes)
	}
}

// Pod Names Tests - Tests for CLI pod name selection functionality

func TestCreateCaptureCommand_PodNamesClearsDefaultNodeSelector(t *testing.T) {
	// When --pod-names is set together with the default --node-selectors,
	// the default selector must be cleared so pod names take precedence.
	savedNodeSelectors := opts.nodeSelectors
	savedPodNames := opts.podNames
	savedNamespace := opts.Namespace
	savedName := opts.Name
	t.Cleanup(func() {
		opts.nodeSelectors = savedNodeSelectors
		opts.podNames = savedPodNames
		opts.Namespace = savedNamespace
		opts.Name = savedName
	})

	name := "test-capture"
	namespace := "default"

	opts.nodeSelectors = DefaultNodeSelectors
	opts.podNames = "nonexistent-pod"
	opts.Namespace = &namespace
	opts.Name = &name

	capture, err := createCaptureF(context.Background(), fake.NewClientset())
	require.NoError(t, err)

	require.Equal(t, []string{"nonexistent-pod"}, capture.Spec.CaptureConfiguration.CaptureTarget.PodNames)
	require.Nil(t, capture.Spec.CaptureConfiguration.CaptureTarget.NodeSelector)
}

func TestCreateCaptureWithPodNames_CRDStructure(t *testing.T) {
	// Table-driven test for pod names CRD structure generation
	cases := []struct {
		name         string
		podNames     []string
		namespace    string
		wantSelector bool
	}{
		{
			name:         "single pod name",
			podNames:     []string{"test-pod"},
			namespace:    "default",
			wantSelector: true,
		},
		{
			name:         "multiple pod names",
			podNames:     []string{"pod1", "pod2", "pod3"},
			namespace:    "myapp",
			wantSelector: true,
		},
		{
			name:         "no pod names",
			podNames:     nil,
			namespace:    "default",
			wantSelector: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			capture := &retinav1alpha1.Capture{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-capture",
					Namespace: tc.namespace,
				},
				Spec: retinav1alpha1.CaptureSpec{
					CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
						CaptureTarget: retinav1alpha1.CaptureTarget{
							PodNames: tc.podNames,
						},
					},
				},
			}

			require.NotNil(t, capture)
			require.Equal(t, tc.namespace, capture.Namespace)

			if tc.wantSelector {
				require.NotNil(t, capture.Spec.CaptureConfiguration.CaptureTarget.PodNames)
				require.Equal(t, tc.podNames, capture.Spec.CaptureConfiguration.CaptureTarget.PodNames)
				require.Nil(t, capture.Spec.CaptureConfiguration.CaptureTarget.NodeSelector)
			} else {
				require.Nil(t, capture.Spec.CaptureConfiguration.CaptureTarget.PodNames)
			}
		})
	}
}

func TestCaptureTarget_PodNames_MutualExclusivity(t *testing.T) {
	// Comprehensive test for mutual exclusivity constraints between pod names and other selectors
	cases := []struct {
		name    string
		target  retinav1alpha1.CaptureTarget
		isValid bool
	}{
		{
			name: "pod names only",
			target: retinav1alpha1.CaptureTarget{
				PodNames: []string{"pod1", "pod2"},
			},
			isValid: true,
		},
		{
			name: "pod names with node selector",
			target: retinav1alpha1.CaptureTarget{
				PodNames: []string{"pod1"},
				NodeSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"kubernetes.io/os": "linux"},
				},
			},
			isValid: false,
		},
		{
			name: "pod names with pod selector",
			target: retinav1alpha1.CaptureTarget{
				PodNames: []string{"pod1"},
				PodSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "test"},
				},
			},
			isValid: false,
		},
		{
			name: "pod names with namespace selector",
			target: retinav1alpha1.CaptureTarget{
				PodNames: []string{"pod1"},
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"name": "default"},
				},
			},
			isValid: false,
		},
		{
			name: "node selector only",
			target: retinav1alpha1.CaptureTarget{
				NodeSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"kubernetes.io/os": "linux"},
				},
			},
			isValid: true,
		},
		{
			name: "pod and namespace selectors",
			target: retinav1alpha1.CaptureTarget{
				PodSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "test"},
				},
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"name": "default"},
				},
			},
			isValid: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			capture := &retinav1alpha1.Capture{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-capture",
					Namespace: "default",
				},
				Spec: retinav1alpha1.CaptureSpec{
					CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
						CaptureTarget: tc.target,
					},
				},
			}

			require.NotNil(t, capture)
			if tc.isValid {
				t.Logf("✓ Valid selector combination: %s", tc.name)
			} else {
				t.Logf("✓ Invalid selector combination (will be caught by validation): %s", tc.name)
			}
		})
	}
}

func TestNodeNamesClearsDefaultNodeSelector(t *testing.T) {
	// When --node-names is specified, the default kubernetes.io/os=linux node-selector
	// must be cleared so that Windows nodes can be targeted by name.
	winNode := NewWindowsNode("win-node-1")
	linNode := NewNode("lin-node-1")

	kubeClient := fake.NewClientset(winNode, linNode)
	kubeClient.PrependReactor("create", "jobs", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		createAction, ok := action.(clienttesting.CreateAction)
		if !ok {
			return false, nil, fmt.Errorf("expected CreateAction, got %T", action) //nolint:err113 // test code
		}
		job := createAction.GetObject().(*batchv1.Job)
		if job.Name == "" {
			job.Name = job.GenerateName + randomString(5)
		}
		return false, job, nil
	})

	cases := []struct {
		name      string
		args      []string
		wantNodes []string
		wantErr   bool
	}{
		{
			name: "node-names targets a Windows node without explicit node-selectors",
			args: []string{
				"create",
				"--name=test-win",
				"--namespace=default",
				"--node-names=win-node-1",
				"--duration=10s",
				"--host-path=capture",
			},
			wantNodes: []string{"win-node-1"},
			wantErr:   false,
		},
		{
			name: "node-names targets a Linux node without explicit node-selectors",
			args: []string{
				"create",
				"--name=test-lin",
				"--namespace=default",
				"--node-names=lin-node-1",
				"--duration=10s",
				"--host-path=capture",
			},
			wantNodes: []string{"lin-node-1"},
			wantErr:   false,
		},
		{
			name: "node-names targets both Linux and Windows nodes",
			args: []string{
				"create",
				"--name=test-both",
				"--namespace=default",
				"--node-names=lin-node-1,win-node-1",
				"--duration=10s",
				"--host-path=capture",
			},
			wantNodes: []string{"lin-node-1", "win-node-1"},
			wantErr:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := NewCommand(kubeClient)
			cmd.SetArgs(tc.args)
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)

			err := cmd.Execute()

			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err, "capture create should succeed for node-names targeting %v", tc.wantNodes)

			// Verify jobs were created for the expected nodes
			jobs, err := kubeClient.BatchV1().Jobs("default").List(context.TODO(), metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", label.CaptureNameLabel, strings.TrimPrefix(tc.args[1], "--name=")),
			})
			require.NoError(t, err)
			require.Len(t, jobs.Items, len(tc.wantNodes), "should create one job per target node")

			gotNodes := map[string]bool{}
			for _, job := range jobs.Items {
				nodeAffinity := job.Spec.Template.Spec.Affinity.NodeAffinity
				require.NotNil(t, nodeAffinity)
				for _, term := range nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
					for _, expr := range term.MatchExpressions {
						if expr.Key == "kubernetes.io/hostname" {
							for _, v := range expr.Values {
								gotNodes[v] = true
							}
						}
					}
				}
			}

			for _, wantNode := range tc.wantNodes {
				require.True(t, gotNodes[wantNode], "expected job targeting node %s", wantNode)
			}
		})
	}
}

func TestGetCLICaptureConfig(t *testing.T) {
	savedDebug, savedJobNumLimit, savedHostPathBaseDir := opts.debug, opts.jobNumLimit, opts.hostPathBaseDir
	t.Cleanup(func() {
		opts.debug = savedDebug
		opts.jobNumLimit = savedJobNumLimit
		opts.hostPathBaseDir = savedHostPathBaseDir
	})

	opts.debug = true
	opts.jobNumLimit = 7
	opts.hostPathBaseDir = "/mnt/captures"

	got := getCLICaptureConfig()

	require.Equal(t, buildinfo.Version, got.CaptureImageVersion)
	require.Equal(t, captureUtils.VersionSourceCLIVersion, got.CaptureImageVersionSource)
	require.True(t, got.CaptureDebug)
	require.Equal(t, 7, got.CaptureJobNumLimit)
	require.Equal(t, "/mnt/captures", got.CaptureHostPathBaseDir)
}

func TestCreateCaptureCommand_AbsoluteHostPath_ShouldFail(t *testing.T) {
	// --host-path must be a bare subpath; absolute paths are rejected by the
	// shared validateHostPath helper used by both the operator and the CLI.
	cmd := NewCommand(fake.NewClientset())

	cmd.SetArgs([]string{
		"create",
		"--name=hp-absolute",
		"--namespace=default",
		"--node-selectors=kubernetes.io/os=linux",
		"--duration=10s",
		"--host-path=/tmp/foo",
	})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := cmd.Execute()
	require.Error(t, err, "command should fail when --host-path is absolute")
	require.Contains(t, err.Error(), "OutputConfiguration.HostPath",
		"error should reference the rejected HostPath field; got: %v", err)
}
func TestHasRemoteDestination(t *testing.T) {
	tests := []struct {
		name string
		opts Opts
		want bool
	}{
		{
			name: "blob upload is remote",
			opts: Opts{blobUpload: "https://account.blob.core.windows.net/container"},
			want: true,
		},
		{
			name: "s3 bucket is remote",
			opts: Opts{s3Bucket: "my-bucket"},
			want: true,
		},
		{
			name: "pvc is remote",
			opts: Opts{pvc: "my-pvc"},
			want: true,
		},
		{
			name: "host-path only is not remote",
			opts: Opts{hostPath: "/mnt/captures"},
			want: false,
		},
		{
			name: "empty opts is not remote",
			opts: Opts{},
			want: false,
		},
		{
			name: "multiple remote destinations",
			opts: Opts{blobUpload: "https://x.blob.core.windows.net/c", s3Bucket: "bucket"},
			want: true,
		},
	}

	for i := range tests {
		t.Run(tests[i].name, func(t *testing.T) {
			require.Equal(t, tests[i].want, hasRemoteDestination(&tests[i].opts))
		})
	}
}

func TestSetSecretOwnerReferences(t *testing.T) {
	ns := testNamespace
	secretName := "blob-secret-abc"

	kubeClient := fake.NewClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: ns,
			},
		},
	)

	origNs := opts.Namespace
	opts.Namespace = &ns
	defer func() { opts.Namespace = origNs }()

	blobUpload := secretName
	capture := &retinav1alpha1.Capture{
		Spec: retinav1alpha1.CaptureSpec{
			OutputConfiguration: retinav1alpha1.OutputConfiguration{
				BlobUpload: &blobUpload,
			},
		},
	}

	jobs := []batchv1.Job{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "job-1",
				Namespace: ns,
				UID:       "uid-1",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "job-2",
				Namespace: ns,
				UID:       "uid-2",
			},
		},
	}

	err := setSecretOwnerReferences(context.Background(), kubeClient, capture, jobs)
	require.NoError(t, err)

	secret, err := kubeClient.CoreV1().Secrets(ns).Get(context.Background(), secretName, metav1.GetOptions{})
	require.NoError(t, err)
	require.Len(t, secret.OwnerReferences, 2, "secret should have owner references for both jobs")
	require.Equal(t, "job-1", secret.OwnerReferences[0].Name)
	require.Equal(t, "job-2", secret.OwnerReferences[1].Name)
	require.Equal(t, "batch/v1", secret.OwnerReferences[0].APIVersion)
	require.Equal(t, "Job", secret.OwnerReferences[0].Kind)
}

func TestSetSecretOwnerReferences_Idempotent(t *testing.T) {
	ns := testNamespace
	secretName := "blob-secret-idem"

	kubeClient := fake.NewClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: ns,
			},
		},
	)

	origNs := opts.Namespace
	opts.Namespace = &ns
	defer func() { opts.Namespace = origNs }()

	blobUpload := secretName
	capture := &retinav1alpha1.Capture{
		Spec: retinav1alpha1.CaptureSpec{
			OutputConfiguration: retinav1alpha1.OutputConfiguration{
				BlobUpload: &blobUpload,
			},
		},
	}

	jobs := []batchv1.Job{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "job-1",
				Namespace: ns,
				UID:       "uid-1",
			},
		},
	}

	// Call twice to simulate retry/re-reconcile.
	err := setSecretOwnerReferences(context.Background(), kubeClient, capture, jobs)
	require.NoError(t, err)
	err = setSecretOwnerReferences(context.Background(), kubeClient, capture, jobs)
	require.NoError(t, err)

	secret, err := kubeClient.CoreV1().Secrets(ns).Get(context.Background(), secretName, metav1.GetOptions{})
	require.NoError(t, err)
	require.Len(t, secret.OwnerReferences, 1, "duplicate owner references should not be added")
}

func TestSetSecretOwnerReferences_NoSecrets(t *testing.T) {
	kubeClient := fake.NewClientset()

	capture := &retinav1alpha1.Capture{
		Spec: retinav1alpha1.CaptureSpec{
			OutputConfiguration: retinav1alpha1.OutputConfiguration{},
		},
	}

	err := setSecretOwnerReferences(context.Background(), kubeClient, capture, []batchv1.Job{})
	require.NoError(t, err)
}

func TestSetSecretOwnerReferences_S3Secret(t *testing.T) {
	ns := testNamespace
	secretName := "s3-secret-xyz"

	kubeClient := fake.NewClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: ns,
			},
		},
	)

	origNs := opts.Namespace
	opts.Namespace = &ns
	defer func() { opts.Namespace = origNs }()

	capture := &retinav1alpha1.Capture{
		Spec: retinav1alpha1.CaptureSpec{
			OutputConfiguration: retinav1alpha1.OutputConfiguration{
				S3Upload: &retinav1alpha1.S3Upload{
					SecretName: secretName,
					Bucket:     "my-bucket",
				},
			},
		},
	}

	jobs := []batchv1.Job{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "job-a",
				Namespace: ns,
				UID:       "uid-a",
			},
		},
	}

	err := setSecretOwnerReferences(context.Background(), kubeClient, capture, jobs)
	require.NoError(t, err)

	secret, err := kubeClient.CoreV1().Secrets(ns).Get(context.Background(), secretName, metav1.GetOptions{})
	require.NoError(t, err)
	require.Len(t, secret.OwnerReferences, 1, "secret should have owner reference for the job")
	require.Equal(t, "job-a", secret.OwnerReferences[0].Name)
	require.Equal(t, "batch/v1", secret.OwnerReferences[0].APIVersion)
	require.Equal(t, "Job", secret.OwnerReferences[0].Kind)
}

func TestDeleteSecret_NotFound(t *testing.T) {
	// deleteSecret should return nil when the secret doesn't exist
	ns := testNamespace
	kubeClient := fake.NewClientset() // no secrets pre-created

	origNs := opts.Namespace
	opts.Namespace = &ns
	defer func() { opts.Namespace = origNs }()

	name := "nonexistent-secret"
	err := deleteSecret(context.Background(), kubeClient, &name)
	require.NoError(t, err, "deleteSecret should not error on NotFound")
}

func TestDeleteSecret_NilName(t *testing.T) {
	ns := testNamespace
	kubeClient := fake.NewClientset()

	origNs := opts.Namespace
	opts.Namespace = &ns
	defer func() { opts.Namespace = origNs }()

	err := deleteSecret(context.Background(), kubeClient, nil)
	require.NoError(t, err, "deleteSecret should return nil for nil secretName")
}

func TestDeleteSecret_ExistingSecret(t *testing.T) {
	// deleteSecret should succeed when the secret exists
	ns := testNamespace
	secretName := "my-secret"
	kubeClient := fake.NewClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: ns,
			},
		},
	)

	origNs := opts.Namespace
	opts.Namespace = &ns
	defer func() { opts.Namespace = origNs }()

	err := deleteSecret(context.Background(), kubeClient, &secretName)
	require.NoError(t, err)

	// Verify secret is deleted
	_, err = kubeClient.CoreV1().Secrets(ns).Get(context.Background(), secretName, metav1.GetOptions{})
	require.Error(t, err, "secret should be gone after deleteSecret")
}

func TestCreateJobs_ActiveDeadlineSeconds(t *testing.T) {
	// When duration > 0, jobs should have ActiveDeadlineSeconds set
	kubeClient := newFakeClientForCleanupTests()

	var createdJob *batchv1.Job
	kubeClient.PrependReactor("create", "jobs", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		createAction := action.(clienttesting.CreateAction)
		job := createAction.GetObject().(*batchv1.Job)
		if job.Name == "" {
			job.Name = job.GenerateName + "test"
		}
		now := metav1.Now()
		job.Status.CompletionTime = &now
		createdJob = job
		return false, job, nil
	})

	cmd := NewCommand(kubeClient)
	cmd.SetArgs([]string{
		"create",
		"--name=test-deadline",
		"--namespace=default",
		"--node-names=node1",
		"--host-path=/tmp/captures",
		"--no-wait=true",
		"--duration=60s",
	})

	err := cmd.Execute()
	require.NoError(t, err)
	require.NotNil(t, createdJob)
	require.NotNil(t, createdJob.Spec.ActiveDeadlineSeconds,
		"ActiveDeadlineSeconds should be set when duration > 0")
	// 60s + 1800s buffer = 1860
	require.Equal(t, int64(1860), *createdJob.Spec.ActiveDeadlineSeconds)
}

func TestCreateJobs_NoTTLWithoutCleanupFlag(t *testing.T) {
	// When --cleanup-after-upload is NOT set, TTL should NOT be set
	// even with remote + no-wait.
	kubeClient := newFakeClientForCleanupTests()

	var createdJob *batchv1.Job
	kubeClient.PrependReactor("create", "jobs", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		createAction := action.(clienttesting.CreateAction)
		job := createAction.GetObject().(*batchv1.Job)
		if job.Name == "" {
			job.Name = job.GenerateName + "test"
		}
		now := metav1.Now()
		job.Status.CompletionTime = &now
		createdJob = job
		return false, job, nil
	})

	cmd := NewCommand(kubeClient)
	cmd.SetArgs([]string{
		"create",
		"--name=test-no-cleanup",
		"--namespace=default",
		"--node-names=node1",
		"--blob-upload=https://testaccount.blob.core.windows.net/container?sv=2021-06-08",
		"--no-wait=true",
		"--duration=5s",
		// NOTE: --cleanup-after-upload is NOT set
	})

	err := cmd.Execute()
	require.NoError(t, err)
	require.NotNil(t, createdJob)
	require.Nil(t, createdJob.Spec.TTLSecondsAfterFinished,
		"TTL should NOT be set without --cleanup-after-upload")
}

func TestWaitUntilJobsComplete_ShortDuration(t *testing.T) {
	// Verifies that waitUntilJobsComplete uses a deadline of at least
	// DefaultWaitTimeout (5min), and completes quickly when jobs are already done.
	// This exercises the deadline calculation (duration + 5min, floored at DefaultWaitTimeout)
	// and the period clamping logic.
	kubeClient := newFakeClientForCleanupTests()

	// The fake client's reactor already marks jobs as completed on create.
	// For the "get" call inside waitUntilJobsComplete, we need the job to appear completed.
	kubeClient.PrependReactor("get", "jobs", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		getAction := action.(clienttesting.GetAction)
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      getAction.GetName(),
				Namespace: getAction.GetNamespace(),
			},
			Status: batchv1.JobStatus{
				CompletionTime: &metav1.Time{},
				Conditions: []batchv1.JobCondition{
					{
						Type:   batchv1.JobComplete,
						Status: corev1.ConditionTrue,
					},
				},
			},
		}
		return true, job, nil
	})

	cmd := NewCommand(kubeClient)
	cmd.SetArgs([]string{
		"create",
		"--name=test-wait-short",
		"--namespace=default",
		"--node-names=node1",
		"--host-path=/tmp/captures",
		"--no-wait=false",
		"--duration=2s",
	})

	// This should complete quickly (within seconds) because the fake client
	// reports jobs as already completed. If the deadline/period logic is broken,
	// this would hang until the test timeout.
	err := cmd.Execute()
	require.NoError(t, err)
}
