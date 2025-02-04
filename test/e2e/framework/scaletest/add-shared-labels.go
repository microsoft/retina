package scaletest

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type patchStringValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

type AddSharedLabelsToAllPods struct {
	Ctx                   context.Context
	KubeConfigFilePath    string
	NumSharedLabelsPerPod int
	Namespace             string
}

// Useful when wanting to do parameter checking, for example
// if a parameter length is known to be required less than 80 characters,
// do this here so we don't find out later on when we run the step
// when possible, try to avoid making external calls, this should be fast and simple
func (a *AddSharedLabelsToAllPods) Prevalidate() error {
	return nil
}

// Primary step where test logic is executed
// Returning an error will cause the test to fail
func (a *AddSharedLabelsToAllPods) Run() error {

	if a.NumSharedLabelsPerPod < 1 {
		return nil
	}

	config, err := clientcmd.BuildConfigFromFlags("", a.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	resources, err := clientset.CoreV1().Pods(a.Namespace).List(a.Ctx, metav1.ListOptions{})

	patchBytes, err := getSharedLabelsPatch(a.NumSharedLabelsPerPod)
	if err != nil {
		return fmt.Errorf("error getting label patch: %w", err)
	}

	for _, resource := range resources.Items {
		err = patchLabel(a.Ctx, clientset, a.Namespace, resource.Name, patchBytes)
		if err != nil {
			log.Printf("Error adding shared labels to pod %s: %s\n", resource.Name, err)
		}
	}

	return nil
}

// Require for background steps
func (a *AddSharedLabelsToAllPods) Stop() error {
	return nil
}

func patchLabel(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName string, patchBytes []byte) error {
	log.Println("Labeling Pod", podName)
	_, err := clientset.CoreV1().Pods(namespace).Patch(ctx, podName,
		types.JSONPatchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to patch pod: %w", err)
	}

	return nil
}

func getSharedLabelsPatch(numLabels int) ([]byte, error) {
	patch := []patchStringValue{}
	for i := 0; i < numLabels; i++ {
		patch = append(patch, patchStringValue{
			Op:    "add",
			Path:  "/metadata/labels/shared-lab-" + fmt.Sprintf("%05d", i),
			Value: "val",
		})
	}
	b, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("error marshalling patch: %w", err)
	}

	return b, nil
}

func contextToLabelAllPods() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 120*time.Minute)
}
