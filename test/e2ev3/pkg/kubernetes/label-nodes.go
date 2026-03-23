package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	retry "github.com/microsoft/retina/test/retry"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type patchStringValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

type LabelNodes struct {
	RestConfig *rest.Config
	Labels     map[string]string
}

func (l *LabelNodes) String() string { return "label-nodes" }

func (l *LabelNodes) Do(ctx context.Context) error {
	log := slog.With("step", l.String())
	clientset, err := kubernetes.NewForConfig(l.RestConfig)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

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
		log.Info("labeling node", "node", nodes.Items[i].Name)
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
