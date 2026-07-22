// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package capture

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/label"
	"github.com/microsoft/retina/test/e2e/framework/generic"
	"github.com/microsoft/retina/test/retry"
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
	log.Print("Running retina capture create...")
	ctx := context.TODO()

	imageRegistry := os.Getenv(generic.DefaultImageRegistry)
	imageNamespace := os.Getenv(generic.DefaultImageNamespace)
	imageTag := os.Getenv(generic.DefaultTagEnv)

	os.Setenv("KUBECONFIG", v.KubeConfigPath)
	log.Printf("KUBECONFIG: %s\n", os.Getenv("KUBECONFIG"))

	cmd := exec.CommandContext(ctx, "kubectl", "retina", "capture", "create", "--namespace", v.CaptureNamespace, "--name", v.CaptureName, "--duration", v.Duration, "--debug") //#nosec
	cmd.Env = append(os.Environ(), "RETINA_AGENT_IMAGE="+filepath.Join(imageRegistry, imageNamespace, "retina-agent:"+imageTag))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "failed to execute create capture command: %s", string(output))
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

	retrier := retry.Retrier{Attempts: 5, Delay: 10 * time.Second, ExpBackoff: true}
	err = retrier.Do(ctx, func() error {
		e := v.verifyJobs(ctx, clientset)
		if e != nil {
			log.Printf("failed to verify capture jobs: %v, retrying...", e)
			return e
		}
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "failed to verify capture jobs were created")
	}

	err = v.downloadCapture(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to download and validate capture files")
	}

	err = v.deleteJobs(ctx, clientset)
	if err != nil {
		return errors.Wrap(err, "failed to delete capture jobs")
	}

	return nil
}

// Verify that capture jobs are created (with appropriate labels), and completed successfully
func (v *validateCapture) verifyJobs(ctx context.Context, clientset *kubernetes.Clientset) error {
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

	return nil
}

func (v *validateCapture) deleteJobs(ctx context.Context, clientset *kubernetes.Clientset) error {
	log.Printf("Running retina capture delete...")
	cmd := exec.CommandContext(ctx, "kubectl", "retina", "capture", "delete", "--namespace", v.CaptureNamespace, "--name", v.CaptureName) //#nosec
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "failed to execute delete command")
	}
	log.Printf("Delete command output: %s\n", output)

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

func (v *validateCapture) downloadCapture(ctx context.Context) error {
	log.Print("Downloading capture files...")

	outputDir := filepath.Join(".", v.CaptureName)

	// Run the download command
	cmd := exec.CommandContext(ctx, "kubectl", "retina", "capture", "download", "--namespace", v.CaptureNamespace, "--name", v.CaptureName) // #nosec
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "failed to execute download capture command: %s", string(output))
	}
	log.Printf("Download capture command output: %s\n", output)

	// List files in the output directory
	files, err := os.ReadDir(outputDir)
	if err != nil {
		return errors.Wrapf(err, "failed to list files in output directory %s", outputDir)
	}

	// Validate the number of files
	if len(files) == 0 {
		return errors.New("no capture files were downloaded")
	}
	log.Printf("Downloaded %d capture files", len(files))

	// Validate file names and content
	for _, file := range files {
		filePath := filepath.Join(outputDir, file.Name())

		// Check that the file has the expected tar.gz extension
		if !strings.HasSuffix(file.Name(), ".tar.gz") {
			return errors.Errorf("downloaded file %s does not have the expected .tar.gz extension", file.Name())
		}

		// Check that the file is not empty
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			return errors.Wrapf(err, "failed to get file info for %s", filePath)
		}

		if fileInfo.Size() == 0 {
			return errors.Errorf("downloaded file %s is empty", filePath)
		}

		log.Printf("Validated file: %s (Size: %d bytes)", file.Name(), fileInfo.Size())
	}

	return nil
}
