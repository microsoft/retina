// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package capture

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/label"
	"github.com/microsoft/retina/test/retry"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	ErrNoCaptureJobsFound      = fmt.Errorf("no capture jobs found")
	ErrFoundNonZeroCaptureJobs = fmt.Errorf("found non-zero amount of capture jobs when expecting zero after deletion")
	ErrMissingEventOnCaptureJob = fmt.Errorf("missing SuccessfulCreate or Completed event on capture job")
	ErrCaptureJobFailed        = fmt.Errorf("capture job failed")
)

// ValidateCaptureStep runs the full kubectl retina capture lifecycle:
// create, verify jobs, download, validate files, and delete.
type ValidateCaptureStep struct {
	CaptureName      string
	CaptureNamespace string
	Duration         string
	KubeConfigPath   string
	RestConfig       *rest.Config
	ImageTag         string
	ImageRegistry    string
	ImageNamespace   string
}

func (v *ValidateCaptureStep) String() string { return "validate-capture" }

func (v *ValidateCaptureStep) Do(ctx context.Context) error {
	log := slog.With("step", v.String())
	log.Info("running retina capture create")

	imageRegistry := v.ImageRegistry
	imageNamespace := v.ImageNamespace
	imageTag := v.ImageTag

	os.Setenv("KUBECONFIG", v.KubeConfigPath) //nolint:errcheck // best effort
	log.Info("KUBECONFIG set", "path", os.Getenv("KUBECONFIG"))

	cmd := exec.CommandContext(ctx, "kubectl", "retina", "capture", "create", "--namespace", v.CaptureNamespace, "--name", v.CaptureName, "--duration", v.Duration, "--debug") //#nosec
	cmd.Env = append(os.Environ(), "RETINA_AGENT_IMAGE="+filepath.Join(imageRegistry, imageNamespace, "retina-agent:"+imageTag))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute create capture command: %s: %w", string(output), err)
	}
	log.Info("create capture command completed", "output", string(output))

	clientset, err := kubernetes.NewForConfig(v.RestConfig)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	retrier := retry.Retrier{Attempts: 5, Delay: 10 * time.Second, ExpBackoff: true}
	err = retrier.Do(ctx, func() error {
		e := v.verifyJobs(ctx, log, clientset)
		if e != nil {
			log.Warn("failed to verify capture jobs, retrying", "error", e)
			return e
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to verify capture jobs were created: %w", err)
	}

	if err := v.downloadCapture(ctx, log); err != nil {
		return fmt.Errorf("failed to download and validate capture files: %w", err)
	}
	defer func() {
		outputDir := filepath.Join(".", v.CaptureName)
		if err := os.RemoveAll(outputDir); err != nil {
			log.Warn("failed to clean up capture files", "dir", outputDir, "error", err)
		}
	}()

	if err := v.deleteJobs(ctx, log, clientset); err != nil {
		return fmt.Errorf("failed to delete capture jobs: %w", err)
	}

	return nil
}

func (v *ValidateCaptureStep) verifyJobs(ctx context.Context, log *slog.Logger, clientset *kubernetes.Clientset) error {
	captureJobSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			label.CaptureNameLabel: v.CaptureName,
			label.AppLabel:         captureConstants.CaptureAppname,
		},
	}
	labelSelector, err := labels.Parse(metav1.FormatLabelSelector(captureJobSelector))
	if err != nil {
		return fmt.Errorf("failed to parse label selector: %w", err)
	}

	jobList, err := clientset.BatchV1().Jobs(v.CaptureNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	})
	if err != nil {
		return fmt.Errorf("failed to list capture jobs: %w", err)
	}

	if len(jobList.Items) == 0 {
		return fmt.Errorf("with labels %s=%s and %s=%s: %w",
			label.CaptureNameLabel, v.CaptureName,
			label.AppLabel, captureConstants.CaptureAppname, ErrNoCaptureJobsFound)
	}

	log.Info("found capture jobs", "count", len(jobList.Items))

	for i := range jobList.Items {
		for _, condition := range jobList.Items[i].Status.Conditions {
			if condition.Type == "Complete" && condition.Status == "True" {
				log.Info("job completed", "job", jobList.Items[i].Name)
			}
			if condition.Type == "Failed" && condition.Status == "True" {
				return fmt.Errorf("%s: %w", jobList.Items[i].Name, ErrCaptureJobFailed)
			}
		}
	}

	events, err := clientset.CoreV1().Events(v.CaptureNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list events: %w", err)
	}
	for i := range jobList.Items {
		if err := v.checkJobEvents(jobList.Items[i].Name, events); err != nil {
			return fmt.Errorf("failed to verify events for job %s: %w", jobList.Items[i].Name, err)
		}
		log.Info("job has required events", "job", jobList.Items[i].Name)
	}

	return nil
}

func (v *ValidateCaptureStep) checkJobEvents(jobName string, events *v1.EventList) error {
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
		return fmt.Errorf("%s: %w", jobName, ErrMissingEventOnCaptureJob)
	}

	return nil
}

func (v *ValidateCaptureStep) deleteJobs(ctx context.Context, log *slog.Logger, clientset *kubernetes.Clientset) error {
	log.Info("running retina capture delete")
	cmd := exec.CommandContext(ctx, "kubectl", "retina", "capture", "delete", "--namespace", v.CaptureNamespace, "--name", v.CaptureName) //#nosec
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute delete command: %w", err)
	}
	log.Info("delete command completed", "output", string(output))

	captureJobSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			label.CaptureNameLabel: v.CaptureName,
			label.AppLabel:         captureConstants.CaptureAppname,
		},
	}
	labelSelector, err := labels.Parse(metav1.FormatLabelSelector(captureJobSelector))
	if err != nil {
		return fmt.Errorf("failed to parse label selector: %w", err)
	}

	pollRetrier := retry.Retrier{Attempts: 10, Delay: 1 * time.Second, ExpBackoff: true}
	err = pollRetrier.Do(ctx, func() error {
		jobList, listErr := clientset.BatchV1().Jobs(v.CaptureNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector.String(),
		})
		if listErr != nil {
			return fmt.Errorf("failed to list jobs during delete verification: %w", listErr)
		}
		if len(jobList.Items) > 0 {
			return ErrFoundNonZeroCaptureJobs
		}
		return nil
	})
	if err != nil {
		return err
	}

	log.Info("all relevant capture jobs deleted")
	return nil
}

func (v *ValidateCaptureStep) downloadCapture(ctx context.Context, log *slog.Logger) error {
	log.Info("downloading capture files")

	outputDir := filepath.Join(".", v.CaptureName)

	cmd := exec.CommandContext(ctx, "kubectl", "retina", "capture", "download", "--namespace", v.CaptureNamespace, "--name", v.CaptureName) // #nosec
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute download capture command: %s: %w", string(output), err)
	}
	log.Info("download capture command completed", "output", string(output))

	files, err := os.ReadDir(outputDir)
	if err != nil {
		return fmt.Errorf("failed to list files in output directory %s: %w", outputDir, err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no capture files were downloaded")
	}
	log.Info("downloaded capture files", "count", len(files))

	for _, file := range files {
		filePath := filepath.Join(outputDir, file.Name())

		if !strings.HasSuffix(file.Name(), ".tar.gz") {
			return fmt.Errorf("downloaded file %s does not have the expected .tar.gz extension", file.Name())
		}

		fileInfo, err := os.Stat(filePath)
		if err != nil {
			return fmt.Errorf("failed to get file info for %s: %w", filePath, err)
		}

		if fileInfo.Size() == 0 {
			return fmt.Errorf("downloaded file %s is empty", filePath)
		}

		log.Info("validated file", "file", file.Name(), "size", fileInfo.Size())
	}

	return nil
}
