package kubernetes

import (
	"context"
	"io"
	"log/slog"

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
	log := slog.With("step", p.String())
	log.Info("printing pod logs", "namespace", p.Namespace, "labelSelector", p.LabelSelector)

	clientset, err := kubernetes.NewForConfig(p.RestConfig)
	if err != nil {
		log.Error("creating clientset", "error", err)
	}

	PrintPodLogs(ctx, log, clientset, p.Namespace, p.LabelSelector)

	return nil
}

func PrintPodLogs(ctx context.Context, log *slog.Logger, clientset *kubernetes.Clientset, namespace, labelSelector string) {
	// List all the pods in the namespace
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		log.Error("listing pods", "error", err)
	}

	// Iterate over the pods and get the logs for each pod
	for i := range pods.Items {
		pod := pods.Items[i]

		// Get the logs for the pod
		req := clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{})
		podLogs, err := req.Stream(ctx)
		if err != nil {
			log.Error("getting logs for pod", "pod", pod.Name, "error", err)
		}

		// Read the logs
		buf, err := io.ReadAll(podLogs)
		if err != nil {
			log.Error("reading logs for pod", "pod", pod.Name, "error", err)
		}

		podLogs.Close()

		// Print the logs
		log.Info("pod logs", "pod", pod.Name, "logs", string(buf))
	}
}
