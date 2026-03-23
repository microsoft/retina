package kubernetes

import (
	"context"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func GetPodIP(ctx context.Context, restConfig *rest.Config, namespace, podName string) (string, error) {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return "", errors.Wrapf(err, "error creating Kubernetes clientset")
	}

	pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", errors.Wrapf(err, "error getting pod %s in namespace %s", podName, namespace)
	}
	if pod.Status.PodIP == "" {
		return "", errors.Errorf("pod %s in namespace %s has no IP", podName, namespace)
	}
	return pod.Status.PodIP, nil
}
