package kubernetes

import (
	"context"
	"fmt"
	"strings"
	"time"

	retry "github.com/microsoft/retina/test/retry"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type LoadAndPinWinBPF struct {
	KubeConfigFilePath                 string
	LoadAndPinWinBPFDeamonSetNamespace string
	LoadAndPinWinBPFDeamonSetName      string
}

func ExecCommandInWinPod(KubeConfigFilePath string, cmd string, Namespace string, LabelSelector string) (string, error) {
	defaultRetrier = retry.Retrier{Attempts: 15, Delay: 5 * time.Second}
	// Create a context with a timeout (e.g., 30 seconds)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	config, err := clientcmd.BuildConfigFromFlags("", KubeConfigFilePath)
	if err != nil {
		return "", fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	pods, err := clientset.CoreV1().Pods(Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: LabelSelector,
	})
	if err != nil {
		panic(err.Error())
	}

	var windowsPod *v1.Pod
	for pod := range pods.Items {
		if pods.Items[pod].Spec.NodeSelector["kubernetes.io/os"] == "windows" {
			windowsPod = &pods.Items[pod]
		}
	}

	if windowsPod == nil {
		return "", fmt.Errorf("no Windows Pod found in label %s", LabelSelector)
	}

	var outputBytes []byte
	err = defaultRetrier.Do(ctx, func() error {
		outputBytes, err = ExecPod(ctx, clientset, config, windowsPod.Namespace, windowsPod.Name, cmd)
		if err != nil {
			return fmt.Errorf("error executing command in windows pod: %w", err)
		}

		if len(outputBytes) == 0 {
			return fmt.Errorf("no output from command")
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	return string(outputBytes), nil
}

func (a *LoadAndPinWinBPF) Run() error {
	// Copy Event Writer into Node
	LoadAndPinWinBPFDLabelSelector := fmt.Sprintf("name=%s", a.LoadAndPinWinBPFDeamonSetName)
	_, err := ExecCommandInWinPod(a.KubeConfigFilePath, "move /Y .\\event-writer-helper.bat C:\\event-writer-helper.bat", a.LoadAndPinWinBPFDeamonSetNamespace, LoadAndPinWinBPFDLabelSelector)
	if err != nil {
		return err
	}

	_, err = ExecCommandInWinPod(a.KubeConfigFilePath, "C:\\event-writer-helper.bat EventWriter-Setup", a.LoadAndPinWinBPFDeamonSetNamespace, LoadAndPinWinBPFDLabelSelector)
	if err != nil {
		return err
	}

	// pin maps
	output, err := ExecCommandInWinPod(a.KubeConfigFilePath, "C:\\event-writer-helper.bat EventWriter-LoadAndPinPrgAndMaps", a.LoadAndPinWinBPFDeamonSetNamespace, LoadAndPinWinBPFDLabelSelector)
	if err != nil {
		return err
	}

	time.Sleep(5 * time.Second)
	fmt.Println(output)
	if strings.Contains(output, "error") || strings.Contains(output, "failed") || strings.Contains(output, "existing") {
		return fmt.Errorf("error in loading and pinning BPF maps and program: %s", output)
	}
	return nil
}

func (a *LoadAndPinWinBPF) Prevalidate() error {
	return nil
}

func (a *LoadAndPinWinBPF) Stop() error {
	return nil
}
