package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	retry "github.com/microsoft/retina/test/retry"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	ErrNoWindowsPodFound = errors.New("no Windows Pod found in label")
	ErrNoCommandOutput   = errors.New("no output from command")
	ErrLoadPinBPFFailed  = errors.New("error in loading and pinning BPF maps and program")
)

type LoadAndPinWinBPF struct {
	KubeConfigFilePath                 string
	LoadAndPinWinBPFDeamonSetNamespace string
	LoadAndPinWinBPFDeamonSetName      string
}

func isRunningWindowsPodOnNode(pod *v1.Pod, targetNodeName string) bool {
	return pod.Spec.NodeSelector["kubernetes.io/os"] == "windows" &&
		pod.Status.Phase == v1.PodRunning &&
		(targetNodeName == "" || pod.Spec.NodeName == targetNodeName)
}

func kubernetesClient(kubeConfigFilePath string) (*rest.Config, *kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigFilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	return config, clientset, nil
}

func runningWindowsPods(ctx context.Context, clientset kubernetes.Interface, namespace, labelSelector string) ([]v1.Pod, error) {
	podList, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("error listing pods: %w", err)
	}

	pods := make([]v1.Pod, 0, len(podList.Items))
	for i := range podList.Items {
		if isRunningWindowsPodOnNode(&podList.Items[i], "") {
			pods = append(pods, podList.Items[i])
		}
	}

	return pods, nil
}

func GetPodNodeName(kubeConfigFilePath, namespace, podName string) (string, error) {
	_, clientset, err := kubernetesClient(kubeConfigFilePath)
	if err != nil {
		return "", err
	}

	pod, err := clientset.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("error getting pod %s in namespace %s: %w", podName, namespace, err)
	}
	if pod.Spec.NodeName == "" {
		return "", fmt.Errorf("pod %s in namespace %s is not assigned to a node", podName, namespace)
	}

	return pod.Spec.NodeName, nil
}

func WaitForPodReadyWithTimeOut(ctx context.Context, kubeConfigFilePath, namespace, labelSelector string, timeout time.Duration) error {
	_, clientset, err := kubernetesClient(kubeConfigFilePath)
	if err != nil {
		return err
	}

	timeoutCtx, cancelFunc := context.WithTimeout(ctx, timeout)
	defer cancelFunc()

	return WaitForPodReady(timeoutCtx, clientset, namespace, labelSelector)
}

func ExecCommandInWinPod(kubeConfigFilePath, cmd, namespace, labelSelector, targetNodeName string, expecNonEmptyOutput bool) (string, error) {
	defaultRetrier = retry.Retrier{Attempts: 15, Delay: 5 * time.Second}
	// Create a context with a timeout (e.g., 120 seconds)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	config, clientset, err := kubernetesClient(kubeConfigFilePath)
	if err != nil {
		return "", err
	}

	pods, err := runningWindowsPods(ctx, clientset, namespace, labelSelector)
	if err != nil {
		return "", err
	}

	var windowsPod *v1.Pod
	for i := range pods {
		pod := &pods[i]
		if isRunningWindowsPodOnNode(pod, targetNodeName) {
			// Optionally, check for Ready condition here
			windowsPod = pod
			break
		}
	}

	if windowsPod == nil {
		return "", fmt.Errorf("%w: label %q on node %q", ErrNoWindowsPodFound, labelSelector, targetNodeName)
	}

	var outputBytes []byte
	err = defaultRetrier.Do(ctx, func() error {
		outputBytes, err = ExecPod(ctx, clientset, config, windowsPod.Namespace, windowsPod.Name, cmd)
		if err != nil {
			fmt.Printf("error executing command in windows pod: %v\n", err)
			return fmt.Errorf("error executing command in windows pod: %w", err)
		}

		if len(outputBytes) == 0 && expecNonEmptyOutput {
			return ErrNoCommandOutput
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	return string(outputBytes), nil
}

func runningWindowsPodNodeNames(kubeConfigFilePath, namespace, labelSelector string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	_, clientset, err := kubernetesClient(kubeConfigFilePath)
	if err != nil {
		return nil, err
	}

	pods, err := runningWindowsPods(ctx, clientset, namespace, labelSelector)
	if err != nil {
		return nil, err
	}

	nodeNames := make([]string, 0, len(pods))
	for i := range pods {
		pod := &pods[i]
		if pod.Spec.NodeName != "" {
			nodeNames = append(nodeNames, pod.Spec.NodeName)
		}
	}
	if len(nodeNames) == 0 {
		return nil, fmt.Errorf("%w: label %q", ErrNoWindowsPodFound, labelSelector)
	}

	return nodeNames, nil
}

func (a *LoadAndPinWinBPF) Run() error {
	// Copy Event Writer into Node
	LoadAndPinWinBPFDLabelSelector := "name=" + a.LoadAndPinWinBPFDeamonSetName
	nodeNames, err := runningWindowsPodNodeNames(a.KubeConfigFilePath, a.LoadAndPinWinBPFDeamonSetNamespace, LoadAndPinWinBPFDLabelSelector)
	if err != nil {
		return err
	}

	for _, nodeName := range nodeNames {
		_, err = ExecCommandInWinPod(a.KubeConfigFilePath, "copy /Y .\\event-writer-helper.bat C:\\event-writer-helper.bat", a.LoadAndPinWinBPFDeamonSetNamespace, LoadAndPinWinBPFDLabelSelector, nodeName, true)
		if err != nil {
			return err
		}

		_, err = ExecCommandInWinPod(a.KubeConfigFilePath, "C:\\event-writer-helper.bat EventWriter-Setup", a.LoadAndPinWinBPFDeamonSetNamespace, LoadAndPinWinBPFDLabelSelector, nodeName, true)
		if err != nil {
			return err
		}

		// pin maps
		output, execErr := ExecCommandInWinPod(a.KubeConfigFilePath, "C:\\event-writer-helper.bat EventWriter-LoadAndPinPrgAndMaps", a.LoadAndPinWinBPFDeamonSetNamespace, LoadAndPinWinBPFDLabelSelector, nodeName, false)
		if execErr != nil {
			return execErr
		}

		fmt.Println(output)
		if strings.Contains(output, "error") || strings.Contains(output, "failed") || strings.Contains(output, "existing") {
			return fmt.Errorf("%w on node %s: %s", ErrLoadPinBPFFailed, nodeName, output)
		}
	}

	return nil
}

func (a *LoadAndPinWinBPF) Prevalidate() error {
	return nil
}

func (a *LoadAndPinWinBPF) Stop() error {
	return nil
}
