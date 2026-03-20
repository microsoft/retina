package kubernetes

import (
	"context"
	"fmt"
	"io"
	"log"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type GetPodLogs struct {
	RestConfig    *rest.Config
	Namespace     string
	LabelSelector string
}

func (p *GetPodLogs) String() string { return "get-pod-logs" }

func (p *GetPodLogs) Do(ctx context.Context) error {
	fmt.Printf("printing pod logs for namespace: %s, labelselector: %s\n", p.Namespace, p.LabelSelector)

	clientset, err := kubernetes.NewForConfig(p.RestConfig)
	if err != nil {
		log.Printf("error creating clientset: %s\n", err)
	}

	PrintPodLogs(ctx, clientset, p.Namespace, p.LabelSelector)

	return nil
}

func PrintPodLogs(ctx context.Context, clientset *kubernetes.Clientset, namespace, labelSelector string) {
	// List all the pods in the namespace
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		log.Printf("error listing pods: %s\n", err)
	}

	// Iterate over the pods and get the logs for each pod
	for i := range pods.Items {
		pod := pods.Items[i]
		fmt.Printf("############################## logs for pod %s #########################\n", pod.Name)

		// Get the logs for the pod
		req := clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{})
		podLogs, err := req.Stream(ctx)
		if err != nil {
			fmt.Printf("error getting logs for pod %s: %s\n", pod.Name, err)
		}

		// Read the logs
		buf, err := io.ReadAll(podLogs)
		if err != nil {
			log.Printf("error reading logs for pod %s: %s\n", pod.Name, err)
		}

		podLogs.Close()

		// Print the logs
		log.Println(string(buf))
		fmt.Printf("#######################################################\n")
	}
}
