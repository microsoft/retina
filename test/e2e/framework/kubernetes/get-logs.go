package kubernetes

import (
	"context"
	"fmt"
	"io"
	"log"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func PrintPodLogs(kubeconfigpath string, namespace string, labelSelector string) {
	// Load the kubeconfig file to get the configuration to access the cluster
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigpath)
	if err != nil {
		log.Printf("error building kubeconfig: %s\n", err)
	}

	// Create a new clientset to interact with the cluster
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("error creating clientset: %s\n", err)
	}

	// List all the pods in the namespace
	pods, err := clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		log.Printf("error listing pods: %s\n", err)
	}

	// Iterate over the pods and get the logs for each pod
	for _, pod := range pods.Items {
		fmt.Printf("############################## logs for pod %s #########################\n", pod.Name)

		// Get the logs for the pod
		req := clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{})
		podLogs, err := req.Stream(context.Background())
		if err != nil {
			fmt.Printf("error getting logs for pod %s: %s\n", pod.Name, err)
		}
		defer podLogs.Close()

		// Read the logs
		buf, err := io.ReadAll(podLogs)
		if err != nil {
			log.Printf("error reading logs for pod %s: %s\n", pod.Name, err)
		}

		// Print the logs
		log.Println(string(buf))
	}
}
