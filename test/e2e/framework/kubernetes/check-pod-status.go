package kubernetes

import (
	"context"
	"fmt"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	RetryTimeoutPodsReady  = 5 * time.Minute
	RetryIntervalPodsReady = 5 * time.Second

	printInterval = 5 // print to stdout every 5 iterations
)

type WaitPodsReady struct {
	KubeConfigFilePath string
	Namespace          string
	LabelSelector      string
}

// Useful when wanting to do parameter checking, for example
// if a parameter length is known to be required less than 80 characters,
// do this here so we don't find out later on when we run the step
// when possible, try to avoid making external calls, this should be fast and simple
func (w *WaitPodsReady) Prevalidate() error {
	return nil
}

// Primary step where test logic is executed
// Returning an error will cause the test to fail
func (w *WaitPodsReady) Run() error {

	config, err := clientcmd.BuildConfigFromFlags("", w.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutSeconds*time.Second)
	defer cancel()

	return WaitForPodReady(ctx, clientset, w.Namespace, w.LabelSelector)
}

// Require for background steps
func (w *WaitPodsReady) Stop() error {
	return nil
}

func WaitForPodReady(ctx context.Context, clientset *kubernetes.Clientset, namespace, labelSelector string) error {
	podReadyMap := make(map[string]bool)

	printIterator := 0
	conditionFunc := wait.ConditionWithContextFunc(func(context.Context) (bool, error) {
		defer func() {
			printIterator++
		}()
		var podList *corev1.PodList
		podList, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			return false, fmt.Errorf("error listing Pods: %w", err)
		}

		if len(podList.Items) == 0 {
			log.Printf("no pods found in namespace \"%s\" with label \"%s\"", namespace, labelSelector)
			return false, nil
		}

		// check each indviidual pod to see if it's in Running state
		for i := range podList.Items {
			var pod *corev1.Pod
			pod, err = clientset.CoreV1().Pods(namespace).Get(ctx, podList.Items[i].Name, metav1.GetOptions{})
			if err != nil {
				return false, fmt.Errorf("error getting Pod: %w", err)
			}

			// Check the Pod phase
			if pod.Status.Phase != corev1.PodRunning {
				if printIterator%printInterval == 0 {
					log.Printf("pod \"%s\" is not in Running state yet. Waiting...\n", pod.Name)
				}
				return false, nil
			}

			// Check all container status.
			for _, containerStatus := range pod.Status.ContainerStatuses {
				if !containerStatus.Ready {
					log.Printf("container \"%s\" in pod \"%s\" is not ready yet. Waiting...\n", containerStatus.Name, pod.Name)
					return false, nil
				}
			}

			if !podReadyMap[pod.Name] {
				log.Printf("pod \"%s\" is in Running state\n", pod.Name)
				podReadyMap[pod.Name] = true
			}
		}
		log.Printf("all pods in namespace \"%s\" with label \"%s\" are in Running state\n", namespace, labelSelector)
		return true, nil
	})

	err := wait.PollUntilContextCancel(ctx, RetryIntervalPodsReady, true, conditionFunc)
	if err != nil {
		PrintPodLogs(ctx, clientset, namespace, labelSelector)
		return fmt.Errorf("error waiting for pods in namespace \"%s\" with label \"%s\" to be in Running state: %w", namespace, labelSelector, err)
	}
	return nil
}

func CheckContainerRestart(ctx context.Context, clientset *kubernetes.Clientset, namespace, labelSelector string) error {
	var podList *corev1.PodList
	podList, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return fmt.Errorf("error listing Pods: %w", err)
	}

	for _, pod := range podList.Items {
		for istatus := range pod.Status.ContainerStatuses {
			status := &pod.Status.ContainerStatuses[istatus]
			if status.RestartCount > 0 {
				return fmt.Errorf("pod %s has %d container restarts: status: %+v: %w", pod.Name, status.RestartCount, status, ErrPodCrashed)
			}
		}
	}
	return nil
}
