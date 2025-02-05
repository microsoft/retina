package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	retry "github.com/microsoft/retina/test/retry"
	corev1 "k8s.io/api/core/v1"
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

type LabelNodes struct {
	KubeConfigFilePath string
	Labels             map[string]string
}

func (l *LabelNodes) Prevalidate() error {
	return nil
}

func (l *LabelNodes) Run() error {
	config, err := clientcmd.BuildConfigFromFlags("", l.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	var nodes *corev1.NodeList

	retrier := retry.Retrier{Attempts: defaultRetryAttempts, Delay: defaultRetryDelay}
	err = retrier.Do(ctx, func() error {
		nodes, err = clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to get nodes: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("retrier failed: %w", err)
	}

	patch := []patchStringValue{}
	for k, v := range l.Labels {
		patch = append(patch, patchStringValue{
			Op:    "add",
			Path:  "/metadata/labels/" + k,
			Value: v,
		})
	}
	b, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal patch: %w", err)
	}

	for i := range nodes.Items {
		log.Println("Labeling node", nodes.Items[i].Name)
		err = retrier.Do(ctx, func() error {
			_, err = clientset.CoreV1().Nodes().Patch(ctx, nodes.Items[i].Name, types.JSONPatchType, b, metav1.PatchOptions{})
			if err != nil {
				return fmt.Errorf("failed to patch pod: %w", err)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("retrier failed: %w", err)
		}
	}

	return nil
}

func (l *LabelNodes) Stop() error {
	return nil
}
