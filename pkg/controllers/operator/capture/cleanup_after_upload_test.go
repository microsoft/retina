// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/log"
)

func newScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = retinav1alpha1.AddToScheme(s)
	_ = batchv1.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	return s
}

func newTestReconciler(objects ...runtime.Object) *CaptureReconciler {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	scheme := newScheme()

	clientObjects := make([]client.Object, 0, len(objects))
	for _, obj := range objects {
		clientObjects = append(clientObjects, obj.(client.Object))
	}

	fakeClient := fakeclient.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(clientObjects...).
		WithStatusSubresource(&retinav1alpha1.Capture{}, &batchv1.Job{}).
		Build()

	return &CaptureReconciler{
		Client: fakeClient,
		scheme: scheme,
		logger: log.Logger().Named("capture-test"),
	}
}

func TestCleanUpAfterUpload_AllJobsSucceeded_WithBlobUpload(t *testing.T) {
	secretName := "test-secret"
	capture := &retinav1alpha1.Capture{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-capture",
			Namespace:  "default",
			Finalizers: []string{captureFinalizer},
		},
		Spec: retinav1alpha1.CaptureSpec{
			CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
				CaptureTarget: retinav1alpha1.CaptureTarget{
					NodeSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"kubernetes.io/role": "agent",
						},
					},
				},
			},
			OutputConfiguration: retinav1alpha1.OutputConfiguration{
				BlobUpload: &secretName,
			},
			CleanUpAfterUpload: true,
		},
	}

	completionTime := metav1.Now()
	jobs := []batchv1.Job{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-capture-job-1",
				Namespace: "default",
			},
			Status: batchv1.JobStatus{
				CompletionTime: &completionTime,
				Conditions: []batchv1.JobCondition{
					{
						Type:   batchv1.JobComplete,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
	}

	reconciler := newTestReconciler(capture)
	ctx := context.Background()

	_, err := reconciler.updateCaptureStatusFromJobs(ctx, capture, jobs)
	require.NoError(t, err)

	// The capture should have been deleted (triggering the finalizer cleanup)
	deletedCapture := &retinav1alpha1.Capture{}
	err = reconciler.Client.Get(ctx, types.NamespacedName{Name: "test-capture", Namespace: "default"}, deletedCapture)
	assert.True(t, err != nil || deletedCapture.DeletionTimestamp != nil,
		"capture should be deleted or marked for deletion when CleanUpAfterUpload is true and all jobs succeeded")
}

func TestCleanUpAfterUpload_AllJobsSucceeded_WithS3Upload(t *testing.T) {
	capture := &retinav1alpha1.Capture{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-capture-s3",
			Namespace:  "default",
			Finalizers: []string{captureFinalizer},
		},
		Spec: retinav1alpha1.CaptureSpec{
			CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
				CaptureTarget: retinav1alpha1.CaptureTarget{
					NodeSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"kubernetes.io/role": "agent",
						},
					},
				},
			},
			OutputConfiguration: retinav1alpha1.OutputConfiguration{
				S3Upload: &retinav1alpha1.S3Upload{
					Bucket:     "test-bucket",
					SecretName: "test-s3-secret",
					Region:     "us-east-1",
				},
			},
			CleanUpAfterUpload: true,
		},
	}

	completionTime := metav1.Now()
	jobs := []batchv1.Job{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-capture-s3-job-1",
				Namespace: "default",
			},
			Status: batchv1.JobStatus{
				CompletionTime: &completionTime,
				Conditions: []batchv1.JobCondition{
					{
						Type:   batchv1.JobComplete,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
	}

	reconciler := newTestReconciler(capture)
	ctx := context.Background()

	_, err := reconciler.updateCaptureStatusFromJobs(ctx, capture, jobs)
	require.NoError(t, err)

	// The capture should have been deleted
	deletedCapture := &retinav1alpha1.Capture{}
	err = reconciler.Client.Get(ctx, types.NamespacedName{Name: "test-capture-s3", Namespace: "default"}, deletedCapture)
	assert.True(t, err != nil || deletedCapture.DeletionTimestamp != nil,
		"capture should be deleted when CleanUpAfterUpload is true with S3 upload and all jobs succeeded")
}

func TestCleanUpAfterUpload_NotTriggeredWhenFalse(t *testing.T) {
	secretName := "test-secret"
	capture := &retinav1alpha1.Capture{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-capture-no-cleanup",
			Namespace:  "default",
			Finalizers: []string{captureFinalizer},
		},
		Spec: retinav1alpha1.CaptureSpec{
			CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
				CaptureTarget: retinav1alpha1.CaptureTarget{
					NodeSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"kubernetes.io/role": "agent",
						},
					},
				},
			},
			OutputConfiguration: retinav1alpha1.OutputConfiguration{
				BlobUpload: &secretName,
			},
			CleanUpAfterUpload: false,
		},
	}

	completionTime := metav1.Now()
	jobs := []batchv1.Job{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-capture-no-cleanup-job-1",
				Namespace: "default",
			},
			Status: batchv1.JobStatus{
				CompletionTime: &completionTime,
				Conditions: []batchv1.JobCondition{
					{
						Type:   batchv1.JobComplete,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
	}

	reconciler := newTestReconciler(capture)
	ctx := context.Background()

	_, err := reconciler.updateCaptureStatusFromJobs(ctx, capture, jobs)
	require.NoError(t, err)

	// Capture should still exist since CleanUpAfterUpload is false
	existingCapture := &retinav1alpha1.Capture{}
	err = reconciler.Client.Get(ctx, types.NamespacedName{Name: "test-capture-no-cleanup", Namespace: "default"}, existingCapture)
	require.NoError(t, err)
	assert.Nil(t, existingCapture.DeletionTimestamp, "capture should NOT be deleted when CleanUpAfterUpload is false")
}

func TestCleanUpAfterUpload_NotTriggeredOnFailedJobs(t *testing.T) {
	secretName := "test-secret"
	capture := &retinav1alpha1.Capture{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-capture-failed",
			Namespace:  "default",
			Finalizers: []string{captureFinalizer},
		},
		Spec: retinav1alpha1.CaptureSpec{
			CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
				CaptureTarget: retinav1alpha1.CaptureTarget{
					NodeSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"kubernetes.io/role": "agent",
						},
					},
				},
			},
			OutputConfiguration: retinav1alpha1.OutputConfiguration{
				BlobUpload: &secretName,
			},
			CleanUpAfterUpload: true,
		},
	}

	jobs := []batchv1.Job{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-capture-failed-job-1",
				Namespace: "default",
			},
			Status: batchv1.JobStatus{
				Conditions: []batchv1.JobCondition{
					{
						Type:   batchv1.JobFailed,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
	}

	reconciler := newTestReconciler(capture)
	ctx := context.Background()

	_, err := reconciler.updateCaptureStatusFromJobs(ctx, capture, jobs)
	require.NoError(t, err)

	// Capture should still exist since jobs failed
	existingCapture := &retinav1alpha1.Capture{}
	err = reconciler.Client.Get(ctx, types.NamespacedName{Name: "test-capture-failed", Namespace: "default"}, existingCapture)
	require.NoError(t, err)
	assert.Nil(t, existingCapture.DeletionTimestamp, "capture should NOT be deleted when jobs failed")
}

func TestCleanUpAfterUpload_NotTriggeredWithoutRemoteStorage(t *testing.T) {
	hostPath := "/mnt/captures"
	capture := &retinav1alpha1.Capture{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-capture-local",
			Namespace:  "default",
			Finalizers: []string{captureFinalizer},
		},
		Spec: retinav1alpha1.CaptureSpec{
			CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
				CaptureTarget: retinav1alpha1.CaptureTarget{
					NodeSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"kubernetes.io/role": "agent",
						},
					},
				},
			},
			OutputConfiguration: retinav1alpha1.OutputConfiguration{
				HostPath: &hostPath,
			},
			CleanUpAfterUpload: true,
		},
	}

	completionTime := metav1.Now()
	jobs := []batchv1.Job{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-capture-local-job-1",
				Namespace: "default",
			},
			Status: batchv1.JobStatus{
				CompletionTime: &completionTime,
				Conditions: []batchv1.JobCondition{
					{
						Type:   batchv1.JobComplete,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
	}

	reconciler := newTestReconciler(capture)
	ctx := context.Background()

	_, err := reconciler.updateCaptureStatusFromJobs(ctx, capture, jobs)
	require.NoError(t, err)

	// Capture should still exist since there's no remote storage
	existingCapture := &retinav1alpha1.Capture{}
	err = reconciler.Client.Get(ctx, types.NamespacedName{Name: "test-capture-local", Namespace: "default"}, existingCapture)
	require.NoError(t, err)
	assert.Nil(t, existingCapture.DeletionTimestamp, "capture should NOT be deleted when only local storage is used")
}

func TestCleanUpAfterUpload_AllJobsSucceeded_WithPVC(t *testing.T) {
	pvcName := "test-pvc"
	capture := &retinav1alpha1.Capture{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-capture-pvc",
			Namespace:  "default",
			Finalizers: []string{captureFinalizer},
		},
		Spec: retinav1alpha1.CaptureSpec{
			CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
				CaptureTarget: retinav1alpha1.CaptureTarget{
					NodeSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"kubernetes.io/role": "agent",
						},
					},
				},
			},
			OutputConfiguration: retinav1alpha1.OutputConfiguration{
				PersistentVolumeClaim: &pvcName,
			},
			CleanUpAfterUpload: true,
		},
	}

	completionTime := metav1.Now()
	jobs := []batchv1.Job{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-capture-pvc-job-1",
				Namespace: "default",
			},
			Status: batchv1.JobStatus{
				CompletionTime: &completionTime,
				Conditions: []batchv1.JobCondition{
					{
						Type:   batchv1.JobComplete,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
	}

	reconciler := newTestReconciler(capture)
	ctx := context.Background()

	_, err := reconciler.updateCaptureStatusFromJobs(ctx, capture, jobs)
	require.NoError(t, err)

	// The capture should have been deleted
	deletedCapture := &retinav1alpha1.Capture{}
	err = reconciler.Client.Get(ctx, types.NamespacedName{Name: "test-capture-pvc", Namespace: "default"}, deletedCapture)
	assert.True(t, err != nil || deletedCapture.DeletionTimestamp != nil,
		"capture should be deleted when CleanUpAfterUpload is true with PVC and all jobs succeeded")
}
