package scaletest

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/microsoft/retina/test/retry"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type DeleteAndReAddLabels struct {
	Ctx                   context.Context
	KubeConfigFilePath    string
	NumSharedLabelsPerPod int
	DeleteLabels          bool
	DeleteLabelsInterval  time.Duration
	DeleteLabelsTimes     int
	Namespace             string
}

// Useful when wanting to do parameter checking, for example
// if a parameter length is known to be required less than 80 characters,
// do this here so we don't find out later on when we run the step
// when possible, try to avoid making external calls, this should be fast and simple
func (d *DeleteAndReAddLabels) Prevalidate() error {
	return nil
}

// Primary step where test logic is executed
// Returning an error will cause the test to fail
func (d *DeleteAndReAddLabels) Run() error {
	if d.NumSharedLabelsPerPod <= 2 || !d.DeleteLabels {
		return nil
	}

	config, err := clientcmd.BuildConfigFromFlags("", d.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	ctx, cancel := context.WithTimeout(d.Ctx, 10*time.Second)
	defer cancel()

	labelsToDelete := `"shared-lab-00000": null, "shared-lab-00001": null, "shared-lab-00002": null`
	labelsToAdd := `"shared-lab-00000": "val", "shared-lab-00001": "val", "shared-lab-00002": "val"`

	var pods *corev1.PodList

	retrier := retry.Retrier{Attempts: defaultRetryAttempts, Delay: defaultRetryDelay}

	err = retrier.Do(ctx, func() error {
		pods, err = clientset.CoreV1().Pods(d.Namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list pods: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("retrier failed: %w", err)
	}

	for i := 0; i < d.DeleteLabelsTimes; i++ {
		log.Printf("Deleting labels. Round %d/%d", i+1, d.DeleteLabelsTimes)

		patch := fmt.Sprintf(`{"metadata": {"labels": {%s}}}`, labelsToDelete)

		err = d.deleteLabels(d.Ctx, clientset, pods, patch)
		if err != nil {
			return fmt.Errorf("error deleting labels: %w", err)
		}

		log.Printf("Sleeping for %s", d.DeleteLabelsInterval)
		time.Sleep(d.DeleteLabelsInterval)

		log.Printf("Re-adding labels. Round %d/%d", i+1, d.DeleteLabelsTimes)

		patch = fmt.Sprintf(`{"metadata": {"labels": {%s}}}`, labelsToAdd)

		err = d.addLabels(d.Ctx, clientset, pods, patch)
		if err != nil {
			return fmt.Errorf("error adding labels: %w", err)
		}

		log.Printf("Sleeping for %s", d.DeleteLabelsInterval)
		time.Sleep(d.DeleteLabelsInterval)
	}

	return nil
}

func (d *DeleteAndReAddLabels) addLabels(ctx context.Context, clientset *kubernetes.Clientset, pods *corev1.PodList, patch string) error {
	for _, pod := range pods.Items {
		log.Println("Labeling Pod", pod.Name)

		retrier := retry.Retrier{Attempts: defaultRetryAttempts, Delay: defaultRetryDelay}
		err := retrier.Do(ctx, func() error {
			_, err := clientset.CoreV1().Pods(d.Namespace).Patch(ctx, pod.Name, types.StrategicMergePatchType, []byte(patch), metav1.PatchOptions{})
			if err != nil {
				return fmt.Errorf("could not patch pod: %w", err)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("could not patch pod: %w", err)
		}
	}

	return nil
}

func (d *DeleteAndReAddLabels) deleteLabels(ctx context.Context, clientset *kubernetes.Clientset, pods *corev1.PodList, patch string) error {

	for _, pod := range pods.Items {
		log.Println("Deleting label from Pod", pod.Name)

		retrier := retry.Retrier{Attempts: defaultRetryAttempts, Delay: defaultRetryDelay}
		err := retrier.Do(ctx, func() error {
			_, err := clientset.CoreV1().Pods(d.Namespace).Patch(ctx, pod.Name, types.StrategicMergePatchType, []byte(patch), metav1.PatchOptions{})
			if err != nil {
				return fmt.Errorf("could not patch pod: %w", err)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("could not patch pod: %w", err)
		}
	}
	return nil
}

// Require for background steps
func (d *DeleteAndReAddLabels) Stop() error {
	return nil
}
