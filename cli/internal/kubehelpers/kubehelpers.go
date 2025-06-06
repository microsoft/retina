package kubehelpers

import (
	"context"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Helper to get a Kubernetes client
func getKubeClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			kubeconfig = os.ExpandEnv("$HOME/.kube/config")
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	}
	return kubernetes.NewForConfig(config)
}

// Generic function to get labels for a resource type
func GetResourceLabels(resourceType, namespace string) (labels []string, labelToName map[string][]string, err error) {
	clientset, err := getKubeClient()
	if err != nil {
		return nil, nil, err
	}
	var items []metav1.Object
	switch resourceType {
	case "pod":
		pods, err := clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return nil, nil, err
		}
		for i := range pods.Items {
			items = append(items, &pods.Items[i])
		}
	case "deployment":
		deploys, err := clientset.AppsV1().Deployments(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return nil, nil, err
		}
		for i := range deploys.Items {
			items = append(items, &deploys.Items[i])
		}
	case "daemonset":
		daemonsets, err := clientset.AppsV1().DaemonSets(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return nil, nil, err
		}
		for i := range daemonsets.Items {
			items = append(items, &daemonsets.Items[i])
		}
	}
	labelSet := make(map[string][]string)
	for _, obj := range items {
		for k, v := range obj.GetLabels() {
			label := k + "=" + v
			labelSet[label] = append(labelSet[label], obj.GetName())
		}
	}
	labels = make([]string, 0, len(labelSet))
	for label := range labelSet {
		labels = append(labels, label)
	}
	return labels, labelSet, nil
}

// Get namespace labels
func GetNamespaceLabels() (labels []string, labelToNS map[string][]string, err error) {
	clientset, err := getKubeClient()
	if err != nil {
		return nil, nil, err
	}
	nsList, err := clientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}
	labelSet := make(map[string][]string)
	for _, ns := range nsList.Items {
		for k, v := range ns.Labels {
			label := k + "=" + v
			labelSet[label] = append(labelSet[label], ns.Name)
		}
	}
	labels = make([]string, 0, len(labelSet))
	for label := range labelSet {
		labels = append(labels, label)
	}
	return labels, labelSet, nil
}
