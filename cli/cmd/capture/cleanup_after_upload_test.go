// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/microsoft/retina/pkg/label"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func newFakeClientForCleanupTests() *fake.Clientset {
	objects := []runtime.Object{
		NewNode("node1"),
		NewNamespace("default"),
	}

	kubeClient := fake.NewClientset(objects...)

	// Handle job creation to set job name and quickly mark as completed
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
		// Mark job as completed immediately for cleanup tests
		now := metav1.Now()
		job.Status.CompletionTime = &now
		job.Status.Conditions = []batchv1.JobCondition{
			{
				Type:   batchv1.JobComplete,
				Status: corev1.ConditionTrue,
			},
		}
		return false, job, nil
	})

	// Handle secret creation to set name from GenerateName
	kubeClient.PrependReactor("create", "secrets", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		createAction, ok := action.(clienttesting.CreateAction)
		if !ok {
			return false, nil, fmt.Errorf("expected CreateAction, got %T", action) //nolint:err113 // test code
		}
		secret := createAction.GetObject().(*corev1.Secret)

		// Set secret name if unset (mimics real k8s behavior with GenerateName)
		if secret.Name == "" {
			secret.Name = secret.GenerateName + randomString(5)
		}
		return false, secret, nil
	})

	return kubeClient
}

func TestCleanupAfterUpload_RequiresRemoteStorage(t *testing.T) {
	// When --cleanup-after-upload is set without remote storage, it should fail
	kubeClient := newFakeClientForCleanupTests()
	cmd := NewCommand(kubeClient)

	cmd.SetArgs([]string{
		"create",
		"--name=test-cleanup",
		"--namespace=default",
		"--node-names=node1",
		"--host-path=/tmp/captures",
		"--cleanup-after-upload",
		"--duration=5s",
	})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--cleanup-after-upload requires remote storage (--blob-upload, --s3-bucket, or --pvc)")
}

func TestCleanupAfterUpload_WithBlobUpload(t *testing.T) {
	// When --cleanup-after-upload is set with blob upload, command should succeed
	// and jobs should be created (controller handles cleanup after upload)
	kubeClient := newFakeClientForCleanupTests()
	cmd := NewCommand(kubeClient)

	cmd.SetArgs([]string{
		"create",
		"--name=test-cleanup-blob",
		"--namespace=default",
		"--node-names=node1",
		"--blob-upload=https://testaccount.blob.core.windows.net/container?sv=2021-06-08&ss=b&srt=co&sp=rwdlacitfx&se=2099-01-01",
		"--cleanup-after-upload",
		"--duration=5s",
	})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify jobs were created (controller handles cleanup after upload completes)
	jobs, err := kubeClient.BatchV1().Jobs("default").List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", label.CaptureNameLabel, "test-cleanup-blob"),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, jobs.Items, "jobs should be created for capture")
}

func TestCleanupAfterUpload_WithS3Upload(t *testing.T) {
	// When --cleanup-after-upload is set with S3 upload, command should succeed
	kubeClient := newFakeClientForCleanupTests()
	cmd := NewCommand(kubeClient)

	cmd.SetArgs([]string{
		"create",
		"--name=test-cleanup-s3",
		"--namespace=default",
		"--node-names=node1",
		"--s3-bucket=test-bucket",
		"--s3-region=us-east-1",
		"--s3-access-key-id=AKIAIOSFODNN7EXAMPLE",
		"--s3-secret-access-key=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		"--cleanup-after-upload",
		"--duration=5s",
	})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify jobs were created (controller handles cleanup after upload completes)
	jobs, err := kubeClient.BatchV1().Jobs("default").List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", label.CaptureNameLabel, "test-cleanup-s3"),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, jobs.Items, "jobs should be created for capture")
}

func TestCleanupAfterUpload_RespectsNoWait(t *testing.T) {
	// When --cleanup-after-upload is set with --no-wait=true (default),
	// the CLI should not block. TTL + owner refs handle cleanup.
	kubeClient := newFakeClientForCleanupTests()
	cmd := NewCommand(kubeClient)

	cmd.SetArgs([]string{
		"create",
		"--name=test-cleanup-nowait",
		"--namespace=default",
		"--node-names=node1",
		"--blob-upload=https://testaccount.blob.core.windows.net/container?sv=2021-06-08&ss=b&srt=co&sp=rwdlacitfx&se=2099-01-01",
		"--cleanup-after-upload",
		"--no-wait=true",
		"--duration=5s",
	})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify jobs still exist (CLI returns immediately in no-wait mode,
	// TTL and owner references handle automatic cleanup)
	jobs, err := kubeClient.BatchV1().Jobs("default").List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", label.CaptureNameLabel, "test-cleanup-nowait"),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, jobs.Items, "jobs should exist because CLI returned immediately in no-wait mode")
}

func TestCleanupAfterUpload_DefaultIsFalse(t *testing.T) {
	// Without --cleanup-after-upload flag, the default should be false
	assert.False(t, DefaultCleanUpAfterUpload)
}

func TestCleanupAfterUpload_FlagRegistered(t *testing.T) {
	// Verify the flag is properly registered on the create subcommand
	kubeClient := fake.NewClientset()
	cmd := NewCommand(kubeClient)

	// Find the create subcommand
	var createCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "create" {
			createCmd = sub
			break
		}
	}
	require.NotNil(t, createCmd, "create subcommand should exist")

	flag := createCmd.Flags().Lookup("cleanup-after-upload")
	require.NotNil(t, flag, "cleanup-after-upload flag should be registered")
	assert.Equal(t, "false", flag.DefValue)
	assert.Contains(t, flag.Usage, "clean up capture jobs")
}

func TestCleanupAfterUpload_TTLSetInNoWaitMode(t *testing.T) {
	// When --cleanup-after-upload with --no-wait=true and remote destination,
	// jobs should have TTLSecondsAfterFinished set.
	kubeClient := newFakeClientForCleanupTests()

	var createdJob *batchv1.Job
	kubeClient.PrependReactor("create", "jobs", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		createAction := action.(clienttesting.CreateAction)
		job := createAction.GetObject().(*batchv1.Job)
		if job.Name == "" {
			job.Name = job.GenerateName + "test3"
		}
		now := metav1.Now()
		job.Status.CompletionTime = &now
		createdJob = job
		return false, job, nil
	})

	cmd := NewCommand(kubeClient)
	cmd.SetArgs([]string{
		"create",
		"--name=test-ttl",
		"--namespace=default",
		"--node-names=node1",
		"--blob-upload=https://testaccount.blob.core.windows.net/container?sv=2021-06-08",
		"--cleanup-after-upload",
		"--no-wait=true",
		"--duration=5s",
	})

	err := cmd.Execute()
	require.NoError(t, err)
	require.NotNil(t, createdJob)

	require.NotNil(t, createdJob.Spec.TTLSecondsAfterFinished, "TTL should be set in no-wait mode with remote destination")
	assert.Equal(t, JobTTLSecondsAfterFinished, *createdJob.Spec.TTLSecondsAfterFinished)
}

func TestCleanupAfterUpload_NoTTLWhenHostPathOnly(t *testing.T) {
	// When only host-path is configured (no remote), TTL should NOT be set
	// even in no-wait mode, because the user needs the job to find their capture file.
	kubeClient := newFakeClientForCleanupTests()

	var createdJob *batchv1.Job
	kubeClient.PrependReactor("create", "jobs", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		createAction := action.(clienttesting.CreateAction)
		job := createAction.GetObject().(*batchv1.Job)
		if job.Name == "" {
			job.Name = job.GenerateName + "test4"
		}
		now := metav1.Now()
		job.Status.CompletionTime = &now
		createdJob = job
		return false, job, nil
	})

	cmd := NewCommand(kubeClient)
	cmd.SetArgs([]string{
		"create",
		"--name=test-no-ttl",
		"--namespace=default",
		"--node-names=node1",
		"--host-path=/tmp/captures",
		"--no-wait=true",
		"--duration=5s",
	})

	err := cmd.Execute()
	require.NoError(t, err)
	require.NotNil(t, createdJob)

	assert.Nil(t, createdJob.Spec.TTLSecondsAfterFinished, "TTL should NOT be set when only host-path is configured")
}
