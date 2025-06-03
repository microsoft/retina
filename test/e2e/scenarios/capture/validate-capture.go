// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package capture

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"time"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/label"
)

type validateCapture struct {
	CaptureName      string
	CaptureNamespace string
	Duration         string
	KubeConfigPath   string
}

var (
	ErrInvalidCaptureName       = errors.New("invalid capture name")
	ErrNoCaptureJobsFound       = errors.New("no capture jobs found")
	ErrFoundNonZeroCaptureJobs  = errors.New("found non-zero amount of capture jobs when expecting zero after deletion")
	ErrMissingEventOnCaptureJob = errors.New("missing SuccessfulCreate or Completed event on capture job")
	ErrCaptureJobFailed         = errors.New("capture job failed")
)

func (v *validateCapture) Run() error {
	duration, err := time.ParseDuration(v.Duration)
	if err != nil {
		return errors.Wrapf(err, "failed to parse duration: %s", v.Duration)
	}

	log.Print("Running retina capture create...")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "kubectl", "retina", "capture", "create", "--namespace", v.CaptureNamespace, "--name", v.CaptureName, "--duration", v.Duration) //#nosec
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "failed to execute create capture command")
	}
	log.Printf("Create capture command output: %s\n", output)

	config, err := clientcmd.BuildConfigFromFlags("", v.KubeConfigPath)
	if err != nil {
		return errors.Wrap(err, "failed to build kubeconfig")
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return errors.Wrap(err, "failed to create kubernetes clientset")
	}

	if err := v.verifyJobs(ctx, clientset, duration); err != nil {
		return errors.Wrap(err, "failed to verify capture jobs were created")
	}

	return nil
}

// Verify that capture jobs are created (with appropriate labels), and completed successfully
func (v *validateCapture) verifyJobs(ctx context.Context, clientset *kubernetes.Clientset, duration time.Duration) error {
	captureJobSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			label.CaptureNameLabel: v.CaptureName,
			label.AppLabel:         captureConstants.CaptureAppname,
		},
	}
	labelSelector, err := labels.Parse(metav1.FormatLabelSelector(captureJobSelector))
	if err != nil {
		return errors.Wrap(err, "failed to parse label selector")
	}

	// Wait for capture duration + buffer time to allow jobs to complete
	waitTime := duration + (10 * time.Second)
	log.Printf("Waiting %v for capture jobs to complete...", waitTime)
	time.Sleep(waitTime)

	jobList, err := clientset.BatchV1().Jobs(v.CaptureNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	})
	if err != nil {
		return errors.Wrap(err, "failed to list capture jobs")
	}

	if len(jobList.Items) == 0 {
		return errors.Wrap(ErrNoCaptureJobsFound, fmt.Sprintf("with labels %s=%s and %s=%s",
			label.CaptureNameLabel, v.CaptureName,
			label.AppLabel, captureConstants.CaptureAppname))
	}

	log.Printf("Found %d capture job(s) with appropriate labels.", len(jobList.Items))

	// Check if all jobs are completed successfully
	for i := range jobList.Items {
		for _, condition := range jobList.Items[i].Status.Conditions {
			if condition.Type == "Complete" && condition.Status == "True" {
				log.Printf("Job %s has condition: Complete - True", jobList.Items[i].Name)
			}
			if condition.Type == "Failed" && condition.Status == "True" {
				return errors.Wrap(ErrCaptureJobFailed, jobList.Items[i].Name)
			}
		}
	}

	// Check events for each job to verify SuccessfulCreate and Completed
	events, err := clientset.CoreV1().Events(v.CaptureNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list events: %w", err)
	}
	for i := range jobList.Items {
		if err = v.checkJobEvents(jobList.Items[i].Name, events); err != nil {
			return fmt.Errorf("failed to verify events for job %s: %w", jobList.Items[i].Name, err)
		}
		log.Printf("Job %s has both SuccessfulCreate and Completed events.", jobList.Items[i].Name)
	}

	// Cleanup
	log.Printf("Running retina capture delete...")
	cmd := exec.CommandContext(ctx, "kubectl", "retina", "capture", "delete", "--namespace", v.CaptureNamespace, "--name", v.CaptureName) //#nosec
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "failed to execute delete command")
	}
	log.Printf("Delete command output: %s\n", output)

	// Verify that jobs are deleted
	if err := v.verifyDelete(ctx, clientset, labelSelector); err != nil {
		return errors.Wrap(err, "failed to verify capture jobs were deleted")
	}

	return nil
}

func (v *validateCapture) checkJobEvents(jobName string, events *v1.EventList) error {
	var created, completed bool
	for i := range events.Items {
		if events.Items[i].InvolvedObject.Kind == "Job" && events.Items[i].InvolvedObject.Name == jobName {
			switch events.Items[i].Reason {
			case "SuccessfulCreate":
				created = true
			case "Completed":
				completed = true
			}
		}
	}

	if !created || !completed {
		return errors.Wrap(ErrMissingEventOnCaptureJob, jobName)
	}

	return nil
}

func (v *validateCapture) verifyDelete(ctx context.Context, clientset *kubernetes.Clientset, labelSelector labels.Selector) error {
	// Wait a moment for deletion to propagate
	time.Sleep(5 * time.Second)

	jobList, err := clientset.BatchV1().Jobs(v.CaptureNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	})
	if err != nil {
		return errors.Wrap(err, "failed to list jobs during delete verification")
	}

	if len(jobList.Items) > 0 {
		return ErrFoundNonZeroCaptureJobs
	}

	log.Printf("All relevant capture jobs have been successfully deleted.")
	return nil
}

func (v *validateCapture) Prevalidate() error {
	return nil
}

func (v *validateCapture) Stop() error {
	return nil
}
