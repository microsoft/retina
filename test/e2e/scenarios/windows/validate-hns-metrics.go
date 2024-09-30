package windows

import (
	"context"
	"fmt"

	k8s "github.com/microsoft/retina/test/e2e/framework/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type ValidateHNSMetric struct {
	KubeConfigFilePath string
	RetinaPodNamespace    string
}

func (v *ValidateHNSMetric) Run() error {
	config, err := clientcmd.BuildConfigFromFlags("", v.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	daemonset, err := clientset.AppsV1().DaemonSets(v.RetinaPodNamespace).Get(context.TODO(), "retina-agent-win", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting daemonset: %w", err)
	}

	pods, err := clientset.CoreV1().Pods(daemonset.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "app=retina-agent-win",
	})
	if err != nil {
		return fmt.Errorf("error getting pods: %w", err)
	}

	pod := pods.Items[0]

	output, err := k8s.ExecPod(context.TODO(), clientset, config, pod.Namespace, pod.Name, "Get-Counter -Counter '\\Network Interface(*)\\Bytes Total/sec' | Format-Table -AutoSize")

	fmt.Println(output)
	return nil
}

func (v *ValidateHNSMetric) Prevalidate() error {
	return nil
}

func (v *ValidateHNSMetric) Stop() error {
	return nil
}
