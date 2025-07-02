// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/microsoft/retina/pkg/label"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

const (
	// Constants for creating test capture jobs
	defaultCaptureJobName      = "retina-capture-test"
	defaultCaptureJobNamespace = "workload"
)

type deleteTestCase struct {
	name            string
	inputName       string // name flag for delete
	inputNamespace  string // namespace flag for delete
	jobNamespace    string // namespace where the job is expected to be found
	jobPodSelectors string // pod selectors for the job
}

// newKubeclient creates a consistent fake Kubernetes client for all tests
func newKubeclient() *fake.Clientset {
	objects := []runtime.Object{
		NewNode("A1"),
		NewNode("A2"),
		NewNode("B1"),
		NewNode("B2"),
		NewNamespace("default"),
		NewNamespace("workload"),
	}

	// Add pods to the client
	for _, pod := range NewClientServerPods("A", "default") {
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

func deleteArgs(tc deleteTestCase) []string {
	args := []string{"delete"}
	if tc.inputNamespace != "" {
		args = append(args, "--namespace", tc.inputNamespace)
	}
	if tc.inputName != "" {
		args = append(args, "--name", tc.inputName)
	}
	return args
}

func createArgs(name, namespace, podSelectors string) []string {
	// Create a default set of arguments for the create command
	return []string{
		"create",
		"--name=" + name,
		"--namespace=" + namespace,
		"--pod-selectors=" + podSelectors,
		"--namespace-selectors=name=" + namespace,
	}
}

func createCapture(t *testing.T, kubeClient kubernetes.Interface, name, namespace, podSelectors string) {
	createCmd := NewCommand(kubeClient)

	createCmd.SetArgs(createArgs(name, namespace, podSelectors))

	buf := new(bytes.Buffer)
	createCmd.SetOut(buf)

	err := createCmd.Execute()
	if err != nil {
		t.Fatalf("Failed to create capture jobs: %v", err)
	}
}

// setupDeleteTest prepares the test environment and creates jobs if needed
func setupDeleteTest(t *testing.T, tc deleteTestCase) *fake.Clientset {
	// Create a Kubernetes client with test resources
	kubeClient := newKubeclient()

	createCapture(t, kubeClient, tc.inputName, tc.jobNamespace, tc.jobPodSelectors)
	createCapture(t, kubeClient, "do-not-delete", tc.jobNamespace, tc.jobPodSelectors)

	return kubeClient
}

func jobExists(t *testing.T, kubeClient kubernetes.Interface, name, namespace string) bool {
	jobs, err := kubeClient.BatchV1().Jobs(namespace).List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", label.CaptureNameLabel, name),
		},
	)
	if err != nil {
		t.Fatalf("Failed to list jobs in namespace %s: %v", namespace, err)
	}

	return len(jobs.Items) > 0
}

// jobDeletedCorrectly validates that Kubernetes jobs were deleted correctly
func jobDeletedCorrectly(t *testing.T, kubeClient *fake.Clientset, tc deleteTestCase) {
	t.Helper()

	// Check if the job was deleted as expected
	if jobExists(t, kubeClient, tc.inputName, tc.inputNamespace) {
		t.Errorf("Expected job %s to be deleted from namespace %s, but it still exists",
			tc.inputName, tc.inputNamespace)
	}

	// Check if the other job in namespace is still present
	if !jobExists(t, kubeClient, "do-not-delete", tc.inputNamespace) {
		t.Errorf("Expected job do-not-delete in namespace %s, but it was deleted",
			tc.inputNamespace)
	}
}

func TestDeleteCaptureJobs(t *testing.T) {
	testCases := []deleteTestCase{
		{
			name:            "delete providing name only",
			inputName:       defaultCaptureJobName,
			inputNamespace:  "",
			jobNamespace:    "default",
			jobPodSelectors: "service=A",
		},
		{
			name:            "delete providing name and namespace",
			inputName:       defaultCaptureJobName,
			inputNamespace:  defaultCaptureJobNamespace,
			jobNamespace:    defaultCaptureJobNamespace,
			jobPodSelectors: "service=B",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kubeClient := setupDeleteTest(t, tc)

			// Create a delete command
			deleteCmd := NewCommand(kubeClient)

			// Set command args
			deleteCmd.SetArgs(deleteArgs(tc))
			buf := new(bytes.Buffer)
			deleteCmd.SetOut(buf)

			// Execute the delete command
			err := deleteCmd.Execute()
			if err != nil {
				t.Fatalf("Failed to delete capture job: %v", err)
			}

			// Validate job is deleted correctly
			jobDeletedCorrectly(t, kubeClient, tc)
		})
	}
}
