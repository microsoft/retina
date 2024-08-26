// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	pkgcapture "github.com/microsoft/retina/pkg/capture"
	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	managedOutputLocation "github.com/microsoft/retina/pkg/capture/outputlocation/managed"
	captureUtils "github.com/microsoft/retina/pkg/capture/utils"
	"github.com/microsoft/retina/pkg/common/apiretry"
	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
)

const (
	captureFinalizer = "kappio.io/capture-cleanup"

	captureErrorReasonExceedJobNumLimit  = "ExceedJobNumLimit"
	captureErrorReasonFindSecretFailed   = "FindSecretFailed"
	captureErrorReasonOthers             = "OtherError"
	captureErrorReasonCreateJobFailed    = "CreateJobFailed"
	captureErrorReasonCreateSecretFailed = "CreateSecretFailed"
	captureErrorReasonRunJobFailed       = "RunJobFailed"

	captureInPogressReason        = "JobsInProgress"
	captureCompleteReason         = "JobsCompleted"
	captureInPogressMessage       = "%d/%d Capture jobs are in progress, waiting for completion"
	captureFailedJobFailedMessage = "%d Capture jobs are in failed state"
	captureCompleteMessage        = "All %d Capture jobs are completed"
)

// CaptureReconciler reconciles a Capture object
type CaptureReconciler struct {
	client.Client
	scheme *runtime.Scheme

	logger *log.ZapLogger

	captureToPodTranslator *pkgcapture.CaptureToPodTranslator

	managedStorageAccountManager *managedOutputLocation.StorageAccountManager
}

func NewCaptureReconciler(c client.Client, scheme *runtime.Scheme, kubeClient kubernetes.Interface, captureConfig config.CaptureConfig) (*CaptureReconciler, error) {
	cr := &CaptureReconciler{
		Client: c,
		scheme: scheme,
		logger: log.Logger().Named("Capture"),
	}

	cr.captureToPodTranslator = pkgcapture.NewCaptureToPodTranslator(kubeClient, cr.logger, captureConfig)

	if captureConfig.EnableManagedStorageAccount {
		cr.logger.Info("Managed storage account is enabled")
		managedStorageAccountManager := managedOutputLocation.NewStorageAccountManager()
		cr.managedStorageAccountManager = managedStorageAccountManager
		if err := cr.managedStorageAccountManager.Init(captureConfig.AzureCredentialConfig); err != nil {
			return nil, fmt.Errorf("failed to initialize managed storage account manager: %w", err)
		}
	}

	return cr, nil
}

//+kubebuilder:rbac:groups=retina.sh,resources=captures,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=retina.sh,resources=captures/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=retina.sh,resources=captures/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete;deletecollection
//+kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (cr *CaptureReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	capture := retinav1alpha1.Capture{}
	captureRef := types.NamespacedName{
		Namespace: req.Namespace,
		Name:      req.Name,
	}

	startTime := time.Now()
	cr.logger.Info("Reconciliation starts", zap.String("Capture", captureRef.String()))

	defer func() {
		latency := time.Since(startTime).String()
		cr.logger.Info("Reconciliation ends", zap.String("Capture", captureRef.String()), zap.String("latency", latency))
	}()

	if err := cr.Get(ctx, captureRef, &capture); err != nil {
		cr.logger.Error("Failed to get Capture", zap.Error(err), zap.String("Capture", captureRef.String()))
		// We'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if capture.ObjectMeta.DeletionTimestamp != nil {
		return cr.handleDelete(ctx, &capture)
	}

	// Register finalizer
	if !controllerutil.ContainsFinalizer(&capture, captureFinalizer) {
		controllerutil.AddFinalizer(&capture, captureFinalizer)
		if err := cr.Client.Update(ctx, &capture); err != nil {
			cr.logger.Error("Failed to add capture finalizer", zap.Error(err), zap.String("Capture", captureRef.String()))
			return ctrl.Result{Requeue: true}, err
		}
	}

	return cr.handleUpdate(ctx, &capture)
}

// Capture status condition types are mutually exclusive, and there can be only one condition in given time.
func (cr *CaptureReconciler) updateCaptureStatusFromJobs(ctx context.Context, capture *retinav1alpha1.Capture, captureJobs []batchv1.Job) (ctrl.Result, error) {
	captureRef := types.NamespacedName{
		Namespace: capture.Namespace,
		Name:      capture.Name,
	}

	isJobFinished := func(job *batchv1.Job) (bool, batchv1.JobConditionType) {
		for _, c := range job.Status.Conditions {
			if (c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed) && c.Status == corev1.ConditionTrue {
				return true, c.Type
			}
		}

		return false, ""
	}

	// update status of the capture depending on the status of the jobs
	// if any job failed, the capture is failed
	// if all jobs succeeded, the capture is succeeded
	var activeJobs []*batchv1.Job
	var successfulJobs []*batchv1.Job
	var failedJobs []*batchv1.Job
	for i, job := range captureJobs {
		_, finishedType := isJobFinished(&job)
		switch finishedType {
		case "": // ongoing
			activeJobs = append(activeJobs, &captureJobs[i])
		case batchv1.JobFailed:
			failedJobs = append(failedJobs, &captureJobs[i])
		case batchv1.JobComplete:
			successfulJobs = append(successfulJobs, &captureJobs[i])
		}
	}

	capture.Status.Active = int32(len(activeJobs))
	capture.Status.Failed = int32(len(failedJobs))
	capture.Status.Succeeded = int32(len(successfulJobs))
	// Once we detect jobs are in failed state, we'll update the status of the Capture to error, meanwhile we keep
	// updating the status of the Capture to inProgress if there are still active jobs.
	if len(failedJobs) != 0 {
		cr.logger.Error("Failed to run the Capture job", zap.String("Capture", captureRef.String()))

		meta.SetStatusCondition(&capture.Status.Conditions, metav1.Condition{
			Type:    string(retinav1alpha1.CaptureError),
			Status:  metav1.ConditionTrue,
			Reason:  captureErrorReasonRunJobFailed,
			Message: fmt.Sprintf(captureFailedJobFailedMessage, len(failedJobs)),
		})
	}
	// Update Capture inProgress status if there are still active jobs.
	if len(successfulJobs) != len(captureJobs) {
		meta.SetStatusCondition(&capture.Status.Conditions, metav1.Condition{
			Type:    string(retinav1alpha1.CaptureComplete),
			Status:  metav1.ConditionFalse,
			Reason:  captureInPogressReason,
			Message: fmt.Sprintf(captureInPogressMessage, len(activeJobs), len(captureJobs)),
		})

		return cr.updateStatus(ctx, capture)
	}

	// Update status of Capture to complete when all jobs are completed.
	meta.SetStatusCondition(&capture.Status.Conditions, metav1.Condition{
		Type:    string(retinav1alpha1.CaptureComplete),
		Status:  metav1.ConditionTrue,
		Reason:  captureCompleteReason,
		Message: fmt.Sprintf(captureCompleteMessage, len(successfulJobs)),
	})

	lastCompleteTime := metav1.Now()
	for _, job := range captureJobs {
		if job.Status.CompletionTime != nil && job.Status.CompletionTime.After(lastCompleteTime.Time) {
			lastCompleteTime = *job.Status.CompletionTime
		}
	}
	capture.Status.CompletionTime = &lastCompleteTime
	return cr.updateStatus(ctx, capture)
}

func (cr *CaptureReconciler) createJobsFromCapture(ctx context.Context, capture *retinav1alpha1.Capture) (ctrl.Result, error) {
	captureRef := types.NamespacedName{
		Namespace: capture.Namespace,
		Name:      capture.Name,
	}

	jobs, err := cr.captureToPodTranslator.TranslateCaptureToJobs(capture)
	if err != nil {
		cr.logger.Error("Failed to translate Capture to jobs", zap.Error(err), zap.String("Capture", captureRef.String()))
		var errorReason string
		switch err.(type) {
		case pkgcapture.CaptureJobNumExceedLimitError:
			errorReason = captureErrorReasonExceedJobNumLimit
			cr.logger.Error("Job number exceed limited", zap.Error(err), zap.String("Capture", captureRef.String()))
		case pkgcapture.SecretNotFoundError:
			errorReason = captureErrorReasonFindSecretFailed
			cr.logger.Error("Failed to find Capture secret", zap.Error(err), zap.String("Capture", captureRef.String()))
		default:
			errorReason = captureErrorReasonOthers
			cr.logger.Error("Failed to translate Capture to jobs", zap.Error(err), zap.String("Capture", captureRef.String()))
		}
		// Update status of capture to error
		meta.SetStatusCondition(&capture.Status.Conditions, metav1.Condition{
			Type:    string(retinav1alpha1.CaptureError),
			Status:  metav1.ConditionTrue,
			Reason:  errorReason,
			Message: err.Error(),
		})

		return cr.updateStatus(ctx, capture)
	}

	for _, job := range jobs {
		if opRet, err := controllerutil.CreateOrUpdate(ctx, cr.Client, job, func() error {
			// Set capture as the owner of the above job
			if err := controllerutil.SetControllerReference(capture, job, cr.scheme); err != nil {
				return err
			}
			return nil
		}); err != nil {
			// TODO(mainred): should we delete the created jobs when creating job failed?
			cr.logger.Error("Failed to create Capture job", zap.Error(err), zap.String("Capture", captureRef.String()), zap.String("operation result", string(opRet)))
			// Update status of Capture to error
			meta.SetStatusCondition(&capture.Status.Conditions, metav1.Condition{
				Type:    string(retinav1alpha1.CaptureError),
				Status:  metav1.ConditionTrue,
				Reason:  captureErrorReasonCreateJobFailed,
				Message: fmt.Sprintf("Failed to create Capture job %s/%s", job.Name, job.Namespace),
			})

			return cr.updateStatus(ctx, capture)
		}
		cr.logger.Info("Capture job is created", zap.String("namespace", capture.Namespace), zap.String("Capture job", job.Name))
	}

	// Update the status of Capture to inProgress after all jobs are created successfully.
	meta.SetStatusCondition(&capture.Status.Conditions, metav1.Condition{
		Type:    string(retinav1alpha1.CaptureComplete),
		Status:  metav1.ConditionFalse,
		Reason:  captureInPogressReason,
		Message: fmt.Sprintf(captureInPogressMessage, len(jobs), len(jobs)),
	})

	return cr.updateStatus(ctx, capture)
}

// handleUpdate creates the capture jobs if not found, otherwise update the status of the capture when jobs' status change
func (cr *CaptureReconciler) handleUpdate(ctx context.Context, capture *retinav1alpha1.Capture) (ctrl.Result, error) {
	captureRef := types.NamespacedName{
		Namespace: capture.Namespace,
		Name:      capture.Name,
	}

	// create resources if not found
	captureJobList := &batchv1.JobList{}
	if err := apiretry.Do(
		func() error {
			return cr.Client.List(ctx, captureJobList, client.InNamespace(capture.Namespace), client.MatchingLabels(captureUtils.GetJobLabelsFromCaptureName(capture.Name)))
		},
	); err != nil {
		cr.logger.Error("Failed to list Capture jobs", zap.Error(err), zap.String("Capture", captureRef.String()))
		return ctrl.Result{}, fmt.Errorf("failed to list Capture jobs: %w", err)
	}

	// Once the jobs are created, we'll update the status of the Capture according to the status of the jobs.
	if len(captureJobList.Items) != 0 {
		return cr.updateCaptureStatusFromJobs(ctx, capture, captureJobList.Items)
	}

	// create SAS URL and then secret for the Capture if managed storage account is enabled.
	// Don't repeat the process if the secret already exists.
	if cr.managedStorageAccountEnabled() {
		if capture.Spec.OutputConfiguration.BlobUpload == nil {
			cr.logger.Info("Creating a secret from managed storage account", zap.String("Capture", captureRef.String()))

			sasURL, err := cr.managedStorageAccountManager.CreateContainerSASURL(ctx, capture.Namespace, capture.Spec.CaptureConfiguration.CaptureOption.Duration.Duration)
			if err != nil {
				cr.logger.Error("Failed to create Capture SAS URL", zap.Error(err), zap.String("Capture", captureRef.String()))
				return ctrl.Result{}, fmt.Errorf("failed to create Capture SAS URL: %w", err)
			}

			secret := getSecretFromCapture(capture, sasURL)
			var opRet controllerutil.OperationResult
			if opRet, err = controllerutil.CreateOrUpdate(ctx, cr.Client, &secret, func() error {
				return nil
			}); err != nil {
				cr.logger.Error("Failed to create secret", zap.Error(err), zap.String("Capture", captureRef.String()), zap.String("operation result", string(opRet)))
				// Update status of Capture to error
				meta.SetStatusCondition(&capture.Status.Conditions, metav1.Condition{
					Type:    string(retinav1alpha1.CaptureError),
					Status:  metav1.ConditionTrue,
					Reason:  captureErrorReasonCreateSecretFailed,
					Message: fmt.Sprintf("Failed to create secret %s/%s", secret.Name, secret.Namespace),
				})

				return cr.updateStatus(ctx, capture)
			}
			cr.logger.Info("Secret is created", zap.String("secret name", secret.Name), zap.String("secret namespace", secret.Namespace), zap.String("Capture", captureRef.String()))

			// set secret in the blob upload configuration
			// TODO(mainred): update Capture with container/blob info to simply the following blob download
			capture.Spec.OutputConfiguration.BlobUpload = to.Ptr(secret.Name)
			if err = cr.Client.Update(ctx, capture); err != nil {
				cr.logger.Error("Failed to update capture with managed secret", zap.Error(err), zap.String("secret", secret.Name), zap.String("Capture", captureRef.String()))
				return ctrl.Result{}, fmt.Errorf("failed to update capture with managed secret: %w", err)
			}
			cr.logger.Info("Use the existing secret", zap.Error(err), zap.String("Capture", captureRef.String()), zap.String("secret", *capture.Spec.OutputConfiguration.BlobUpload))
		}
	}

	return cr.createJobsFromCapture(ctx, capture)
}

func (cr *CaptureReconciler) managedStorageAccountEnabled() bool {
	return cr.managedStorageAccountManager != nil
}

func (cr *CaptureReconciler) handleDelete(ctx context.Context, capture *retinav1alpha1.Capture) (ctrl.Result, error) {
	captureRef := types.NamespacedName{
		Namespace: capture.Namespace,
		Name:      capture.Name,
	}
	// The capture is being deleted
	if !controllerutil.ContainsFinalizer(capture, captureFinalizer) {
		cr.logger.Info("Capture is being deleted", zap.String("Capture", captureRef.String()))
		return ctrl.Result{}, nil
	}

	cr.logger.Info("Removing Capture", zap.String("Capture", captureRef.String()))

	deletePropagationBackground := metav1.DeletePropagationBackground
	if err := apiretry.Do(
		func() error {
			return cr.Client.DeleteAllOf(ctx, &batchv1.Job{}, client.InNamespace(capture.Namespace), &client.DeleteAllOfOptions{
				ListOptions: client.ListOptions{
					LabelSelector: labels.SelectorFromSet(labels.Set(captureUtils.GetJobLabelsFromCaptureName(capture.Name))),
				},
				DeleteOptions: client.DeleteOptions{
					PropagationPolicy: &deletePropagationBackground,
				},
			})
		},
	); err != nil {
		cr.logger.Error("Failed to delete Capture jobs", zap.Error(err), zap.String("Capture", captureRef.String()))
		return ctrl.Result{}, fmt.Errorf("failed to delete Capture jobs: %w", err)
	}
	cr.logger.Info("Capture jobs are removed", zap.String("Capture", captureRef.String()))

	// Remove the secret if the secret is created by the operator when the managed storage account is enabled.
	if cr.managedStorageAccountEnabled() {
		managedSecret := getSecretFromCapture(capture, "")
		// Delete the secret only when the secret is created by the operator.
		if *capture.Spec.OutputConfiguration.BlobUpload == managedSecret.Name {
			if err := apiretry.Do(
				func() error {
					return cr.Client.Delete(ctx, &managedSecret) //nolint:wrapcheck // no wrapped, detailed explanation is required for the internal error
				},
			); err != nil && !apierrors.IsNotFound(err) {
				cr.logger.Error("Failed to delete secret", zap.Error(err), zap.String("Capture", captureRef.String()))
				return ctrl.Result{}, fmt.Errorf("failed to delete secret: %w", err)
			}
			cr.logger.Info("Capture secret is removed", zap.String("Capture", captureRef.String()), zap.String("Secret", managedSecret.Name))
		}
	}

	controllerutil.RemoveFinalizer(capture, captureFinalizer)
	if err := cr.Client.Update(ctx, capture); err != nil {
		cr.logger.Error("Failed to remove Capture finalizer", zap.Error(err), zap.String("Capture", captureRef.String()))
		return ctrl.Result{}, fmt.Errorf("failed to remove Capture finalizer: %w", err)
	}
	return ctrl.Result{}, nil
}

func getSecretFromCapture(capture *retinav1alpha1.Capture, sasURL string) corev1.Secret {
	secretName := managedSecretName(capture.Name)
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: capture.Namespace,
			Labels:    captureUtils.GetSerectLabelsFromCaptureName(capture.Name),
		},
		Data: map[string][]byte{
			captureConstants.CaptureOutputLocationBlobUploadSecretKey: []byte(sasURL),
		},
		Type: corev1.SecretTypeOpaque,
	}
	return secret
}

// SetupWithManager sets up the controller with the Manager.
func (cr *CaptureReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&retinav1alpha1.Capture{}).
		Owns(&batchv1.Job{}). // Once the job owned by capture is created /deleted/updated, the capture will be reconciled.
		Complete(cr)
}

// get latest version of the capture before updating its status to avoid update conflicts
// reference: https://github.com/operator-framework/operator-sdk/issues/3968
func (cr *CaptureReconciler) updateStatus(ctx context.Context, capture *retinav1alpha1.Capture) (ctrl.Result, error) {
	captureRef := types.NamespacedName{
		Namespace: capture.Namespace,
		Name:      capture.Name,
	}

	latestCapture := &retinav1alpha1.Capture{}
	if err := cr.Client.Get(ctx, captureRef, latestCapture); err != nil {
		cr.logger.Error("Failed to get Capture", zap.Error(err), zap.String("Capture", captureRef.String()))
		return ctrl.Result{}, fmt.Errorf("failed to get Capture: %w", err)
	}
	if reflect.DeepEqual(capture.Status, latestCapture.Status) {
		return ctrl.Result{}, nil
	}
	latestCapture.Status = capture.Status
	capture = latestCapture
	if err := cr.Client.Status().Update(ctx, latestCapture); err != nil {
		cr.logger.Error("Failed to update status of Capture", zap.Error(err), zap.String("Capture", captureRef.String()))
		return ctrl.Result{}, fmt.Errorf("failed to update status of Capture: %w", err)
	}
	return ctrl.Result{}, nil
}

func managedSecretName(captureName string) string {
	return fmt.Sprintf("managed-%s", captureName)
}
