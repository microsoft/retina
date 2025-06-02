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

func (v *validateCapture) Run() error {
	duration, err := time.ParseDuration(v.Duration)
	if err != nil {
		return errors.Wrapf(err, "failed to parse duration: %s", v.Duration)
	}

	log.Print("Running retina capture create...")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "kubectl", "retina", "capture", "create", "--namespace", v.CaptureNamespace, "--name", v.CaptureName, "--duration", v.Duration)
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
		return fmt.Errorf("no capture jobs found with labels %s=%s and %s=%s",
			label.CaptureNameLabel, v.CaptureName,
			label.AppLabel, captureConstants.CaptureAppname)
	}

	log.Printf("Found %d capture job(s) with appropriate labels.", len(jobList.Items))

	// Check if all jobs are completed successfully
	for _, job := range jobList.Items {
		for _, condition := range job.Status.Conditions {
			if condition.Type == "Complete" && condition.Status == "True" {
				log.Printf("Job %s has condition: Complete - True", job.Name)
			}
			if condition.Type == "Failed" && condition.Status == "True" {
				return fmt.Errorf("job %s failed: %s", job.Name, condition.Message)
			}
		}
	}

	// Check events for each job to verify SuccessfulCreate and Completed
	events, err := clientset.CoreV1().Events(v.CaptureNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list events: %w", err)
	}
	for _, job := range jobList.Items {
		if err := v.checkJobEvents(ctx, clientset, job.Name, events); err != nil {
			return fmt.Errorf("failed to verify events for job %s: %w", job.Name, err)
		}
		log.Printf("Job %s has both SuccessfulCreate and Completed events.", job.Name)
	}

	// Cleanup
	log.Printf("Running retina capture delete...")
	cmd := exec.CommandContext(ctx, "kubectl", "retina", "capture", "delete", "--namespace", v.CaptureNamespace, "--name", v.CaptureName)
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

func (v *validateCapture) checkJobEvents(ctx context.Context, clientset *kubernetes.Clientset, jobName string, events *v1.EventList) error {
	var created, completed bool
	for _, event := range events.Items {
		if event.InvolvedObject.Kind == "Job" && event.InvolvedObject.Name == jobName {
			switch event.Reason {
			case "SuccessfulCreate":
				created = true
			case "Completed":
				completed = true
			}
		}
	}

	if !created || !completed {
		return fmt.Errorf("failed to find SuccessfulCreate/Completed events for job %s", jobName)
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
		return fmt.Errorf("expected 0 jobs remaining, but found more than 0:")
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
