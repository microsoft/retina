// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

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

	flow "github.com/Azure/go-workflow"
	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/label"
	"github.com/microsoft/retina/test/e2ev3/config"
	"github.com/microsoft/retina/test/retry"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Workflow runs the capture validation workflow.
type Workflow struct {
	Cfg *config.E2EConfig
}

func (w *Workflow) String() string { return "capture" }

func (w *Workflow) Do(ctx context.Context) error {
	p := w.Cfg
	kubeConfigFilePath := p.Cluster.KubeConfigPath()
	testPodNamespace := "default"
	imgCfg := &p.Image

	wf := new(flow.Workflow)

	captureName := "retina-capture-e2e-" + rand.String(5)

	installPlugin := &InstallRetinaPluginStep{}
	validateCap := &ValidateCaptureStep{
		CaptureName:      captureName,
		CaptureNamespace: testPodNamespace,
		Duration:         "5s",
		KubeConfigPath:   kubeConfigFilePath,
		RestConfig:       p.Cluster.RestConfig(),
		ImageTag:         imgCfg.Tag,
		ImageRegistry:    imgCfg.Registry,
		ImageNamespace:   imgCfg.Namespace,
	}

	wf.Add(flow.Pipe(installPlugin, validateCap))

	return wf.Do(ctx)
}



const (
	// InstallRetinaBinaryDir is the directory where the kubectl-retina binary will be installed.
	InstallRetinaBinaryDir = "/tmp/retina-bin"
)

// InstallRetinaPluginStep builds and installs the kubectl-retina plugin
// to allow e2e tests to run kubectl retina commands.
type InstallRetinaPluginStep struct{}

func (i *InstallRetinaPluginStep) String() string { return "install-retina-plugin" }

func (i *InstallRetinaPluginStep) Do(ctx context.Context) error {
	log.Print("Building kubectl-retina plugin...")

	if err := os.MkdirAll(InstallRetinaBinaryDir, 0o755); err != nil {
		return fmt.Errorf("failed to create binary directory: %w", err)
	}

	binaryName := "kubectl-retina"

	cmd := exec.Command("git", "rev-parse", "--show-toplevel") // #nosec
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to detect git repository root: %w", err)
	}
	retinaRepoRoot := strings.TrimSpace(string(output))
	log.Printf("Auto-detected repository root: %s", retinaRepoRoot)

	if _, err := os.Stat(retinaRepoRoot); err != nil {
		return fmt.Errorf("invalid RetinaRepoRoot path: %w", err)
	}

	if _, err := os.Stat(filepath.Join(retinaRepoRoot, "cli", "main.go")); err != nil {
		return fmt.Errorf("cli/main.go not found in repository root: %w", err)
	}

	buildCmd := exec.Command("go", "build", "-o",
		filepath.Join(InstallRetinaBinaryDir, binaryName),
		filepath.Join(retinaRepoRoot, "cli", "main.go")) // #nosec
	buildCmd.Dir = retinaRepoRoot
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to build kubectl-retina: %s: %w", buildOutput, err)
	}
	log.Printf("Successfully built kubectl-retina: %s", buildOutput)

	currentPath := os.Getenv("PATH")
	if !strings.Contains(currentPath, InstallRetinaBinaryDir) {
		newPath := fmt.Sprintf("%s:%s", InstallRetinaBinaryDir, currentPath)
		if err := os.Setenv("PATH", newPath); err != nil {
			return fmt.Errorf("failed to update PATH environment variable: %w", err)
		}
		log.Printf("Added %s to PATH", InstallRetinaBinaryDir)
	}

	verifyCmd := exec.Command("kubectl", "plugin", "list") // #nosec
	verifyOutput, err := verifyCmd.CombinedOutput()
	if err != nil {
		log.Printf("Warning: kubectl plugin list command failed: %v. Output: %s", err, verifyOutput)
	} else {
		log.Printf("kubectl plugin list output: %s", verifyOutput)
		if !strings.Contains(string(verifyOutput), "retina") {
			log.Printf("Warning: retina plugin not found in kubectl plugin list output")
		}
	}

	return nil
}

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
	log.Print("Running retina capture create...")

	imageRegistry := v.ImageRegistry
	imageNamespace := v.ImageNamespace
	imageTag := v.ImageTag

	os.Setenv("KUBECONFIG", v.KubeConfigPath) //nolint:errcheck // best effort
	log.Printf("KUBECONFIG: %s\n", os.Getenv("KUBECONFIG"))

	cmd := exec.CommandContext(ctx, "kubectl", "retina", "capture", "create", "--namespace", v.CaptureNamespace, "--name", v.CaptureName, "--duration", v.Duration, "--debug") //#nosec
	cmd.Env = append(os.Environ(), "RETINA_AGENT_IMAGE="+filepath.Join(imageRegistry, imageNamespace, "retina-agent:"+imageTag))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute create capture command: %s: %w", string(output), err)
	}
	log.Printf("Create capture command output: %s\n", output)

	clientset, err := kubernetes.NewForConfig(v.RestConfig)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes clientset: %w", err)
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
		return fmt.Errorf("failed to verify capture jobs were created: %w", err)
	}

	if err := v.downloadCapture(ctx); err != nil {
		return fmt.Errorf("failed to download and validate capture files: %w", err)
	}
	defer func() {
		outputDir := filepath.Join(".", v.CaptureName)
		if err := os.RemoveAll(outputDir); err != nil {
			log.Printf("warning: failed to clean up capture files in %s: %v", outputDir, err)
		}
	}()

	if err := v.deleteJobs(ctx, clientset); err != nil {
		return fmt.Errorf("failed to delete capture jobs: %w", err)
	}

	return nil
}

func (v *ValidateCaptureStep) verifyJobs(ctx context.Context, clientset *kubernetes.Clientset) error {
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

	log.Printf("Found %d capture job(s) with appropriate labels.", len(jobList.Items))

	for i := range jobList.Items {
		for _, condition := range jobList.Items[i].Status.Conditions {
			if condition.Type == "Complete" && condition.Status == "True" {
				log.Printf("Job %s has condition: Complete - True", jobList.Items[i].Name)
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
		log.Printf("Job %s has both SuccessfulCreate and Completed events.", jobList.Items[i].Name)
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

func (v *ValidateCaptureStep) deleteJobs(ctx context.Context, clientset *kubernetes.Clientset) error {
	log.Printf("Running retina capture delete...")
	cmd := exec.CommandContext(ctx, "kubectl", "retina", "capture", "delete", "--namespace", v.CaptureNamespace, "--name", v.CaptureName) //#nosec
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute delete command: %w", err)
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
		return fmt.Errorf("failed to parse label selector: %w", err)
	}

	// Poll until jobs are gone instead of sleeping a fixed duration.
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

	log.Printf("All relevant capture jobs have been successfully deleted.")
	return nil
}

func (v *ValidateCaptureStep) downloadCapture(ctx context.Context) error {
	log.Print("Downloading capture files...")

	outputDir := filepath.Join(".", v.CaptureName)

	cmd := exec.CommandContext(ctx, "kubectl", "retina", "capture", "download", "--namespace", v.CaptureNamespace, "--name", v.CaptureName) // #nosec
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute download capture command: %s: %w", string(output), err)
	}
	log.Printf("Download capture command output: %s\n", output)

	files, err := os.ReadDir(outputDir)
	if err != nil {
		return fmt.Errorf("failed to list files in output directory %s: %w", outputDir, err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no capture files were downloaded")
	}
	log.Printf("Downloaded %d capture files", len(files))

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

		log.Printf("Validated file: %s (Size: %d bytes)", file.Name(), fileInfo.Size())
	}

	return nil
}
