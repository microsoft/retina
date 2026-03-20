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
	"k8s.io/client-go/rest"
)

const (
	RetryTimeoutPodsReady  = 5 * time.Minute
	RetryIntervalPodsReady = 5 * time.Second

	printInterval = 5 // print to stdout every 5 iterations
)

type WaitPodsReady struct {
	RestConfig    *rest.Config
	Namespace     string
	LabelSelector string
}

func (w *WaitPodsReady) String() string { return "wait-pods-ready" }

func (w *WaitPodsReady) Do(ctx context.Context) error {
	clientset, err := kubernetes.NewForConfig(w.RestConfig)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	return WaitForPodReady(ctx, clientset, w.Namespace, w.LabelSelector)
}

func WaitForPodReady(ctx context.Context, clientset *kubernetes.Clientset, namespace, labelSelector string) error {

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

		// check each individual pod to see if it's in Running state
		for i := range podList.Items {

			// Check the Pod phase
			if podList.Items[i].Status.Phase != corev1.PodRunning {
				if printIterator%printInterval == 0 {
					log.Printf("pod \"%s\" is not in Running state yet. Waiting...\n", podList.Items[i].Name)
				}
				return false, nil
			}

			// Check all container status.
			for j := range podList.Items[i].Status.ContainerStatuses {
				if !podList.Items[i].Status.ContainerStatuses[j].Ready {
					log.Printf("container \"%s\" in pod \"%s\" is not ready yet. Waiting...\n", podList.Items[i].Status.ContainerStatuses[j].Name, podList.Items[i].Name)
					return false, nil
				}
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
