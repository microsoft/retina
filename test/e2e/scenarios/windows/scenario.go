package windows

import (
	"context"
	"fmt"
	"time"

	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/microsoft/retina/test/e2e/framework/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	k8sclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const workloadDeleteTimeout = 5 * time.Minute

type Workload struct {
	KubeConfigFilePath string
	Year               string
	ContainerImage     string
}

func (w *Workload) Prevalidate() error {
	_, err := windowsBuild(w.Year)
	return err
}

func (w *Workload) Stop() error { return nil }

func (w *Workload) Run() error {
	build, err := windowsBuild(w.Year)
	if err != nil {
		return err
	}
	config, err := clientcmd.BuildConfigFromFlags("", w.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("building kubeconfig: %w", err)
	}
	client, err := k8sclient.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("creating Kubernetes client: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), workloadDeleteTimeout)
	defer cancel()
	if err := deleteWorkloadPod(ctx, client, "default", "non-hpc-pod-"+w.Year); err != nil {
		return err
	}
	step := &kubernetes.ApplyYamlConfig{
		KubeConfigFilePath: w.KubeConfigFilePath,
		YamlFilePath:       "yaml/windows/non-hpc-pod.yaml",
		ContainerImage:     w.ContainerImage,
		TemplateValues: map[string]string{
			"{{WINDOWS_YEAR}}":  w.Year,
			"{{WINDOWS_BUILD}}": build,
		},
	}
	if err := step.Run(); err != nil {
		return fmt.Errorf("applying WS%s workload: %w", w.Year, err)
	}
	return nil
}

// deleteWorkloadPod removes a workload left by an earlier phase so each phase runs a freshly
// created pod; updating the image of a running pod restarts its container and races readiness checks.
func deleteWorkloadPod(ctx context.Context, client k8sclient.Interface, namespace, name string) error {
	err := client.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("deleting previous workload %s/%s: %w", namespace, name, err)
	}
	err = wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		_, getErr := client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(getErr) {
			return true, nil
		}
		return false, getErr //nolint:wrapcheck // PollUntilContextCancel reports the condition error directly
	})
	if err != nil {
		return fmt.Errorf("waiting for previous workload %s/%s to be deleted: %w", namespace, name, err)
	}
	return nil
}

func ValidateWinBpfMetricScenario(year, image string) *types.Scenario {
	name := "Validate Windows " + year + " BPF Basic and Advanced Metrics"
	steps := []*types.StepWrapper{
		{
			Step: &Workload{
				KubeConfigFilePath: "./test.pem",
				Year:               year,
				ContainerImage:     image,
			},
		},
		{
			Step: &ValidateWinBpfMetric{
				RetinaDaemonSetNamespace:  common.KubeSystemNamespace,
				RetinaDaemonSetName:       "retina-agent-win",
				EbpfXdpDeamonSetNamespace: "install-ebpf-xdp",
				EbpfXdpDeamonSetName:      "install-ebpf-xdp",
				NonHpcAppNamespace:        "default",
				NonHpcAppName:             "non-hpc-" + year,
				NonHpcPodName:             "non-hpc-pod-" + year,
			},
		},
	}
	return types.NewScenario(name, steps...)
}

func ValidateWindowsBasicMetric(year string) *types.Scenario {
	name := "Windows " + year + " Metrics"
	steps := []*types.StepWrapper{
		{
			Step: &Workload{
				KubeConfigFilePath: "./test.pem",
				Year:               year,
				ContainerImage:     "mcr.microsoft.com/windows/servercore:ltsc" + year,
			},
		},
		{
			Step: &ValidateHNSMetric{
				RetinaDaemonSetNamespace: common.KubeSystemNamespace,
				RetinaDaemonSetName:      "retina-agent-win",
				NonHpcPodName:            "non-hpc-pod-" + year,
				NonHpcAppName:            "non-hpc-" + year,
			},
		},
	}
	return types.NewScenario(name, steps...)
}
