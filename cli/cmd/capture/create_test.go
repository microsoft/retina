// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/microsoft/retina/pkg/label"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

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
