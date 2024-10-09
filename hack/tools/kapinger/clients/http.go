package clients

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

type TargetType string

const (
	Service TargetType = "service"
	Pod     TargetType = "pod"

	envTargetType = "TARGET_TYPE"
)

type KapingerHTTPClient struct {
	client        http.Client
	clientset     *kubernetes.Clientset
	labelselector string
	ips           []string
	port          int
	targettype    TargetType
}

func NewKapingerHTTPClient(clientset *kubernetes.Clientset, labelselector string, httpPort int) (*KapingerHTTPClient, error) {
	k := KapingerHTTPClient{
		client: http.Client{
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
			Timeout: 3 * time.Second,
		},
		labelselector: labelselector,
		clientset:     clientset,
		port:          httpPort,
	}

	targettype := os.Getenv(envTargetType)
	if targettype != "" {
		k.targettype = TargetType(targettype)
	} else {
		k.targettype = Service
	}

	err := k.getIPS()
	if err != nil {
		return nil, fmt.Errorf("error getting IPs: %w", err)
	}

	return &k, nil
}
func (k *KapingerHTTPClient) MakeRequests(ctx context.Context, volume int, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ctx.Done():
			log.Printf("HTTP client context done")
			return nil
		case <-ticker.C:
			go func() {
				for i := 0; i < volume; i++ {
					err := k.makeRequest()
					if err != nil {
						log.Printf("error making request: %v", err)
					}
				}
			}()
		}
	}
}

func (k *KapingerHTTPClient) makeRequest() error {
	for _, ip := range k.ips {
		url := fmt.Sprintf("http://%s:%d", ip, k.port)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}

		// Set the "Connection" header to "close"
		req.Header.Set("Connection", "close")

		// Send the request
		resp, err := k.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("Error reading response body from %s: %v", url, err)
			return err
		}
		log.Printf("Response from %s: %s\n", url, string(body))
	}
	return nil
}

func (k *KapingerHTTPClient) getIPS() error {
	ips := []string{}

	switch k.targettype {
	case Service:
		services, err := k.clientset.CoreV1().Services(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{
			LabelSelector: k.labelselector,
		})
		if err != nil {
			return fmt.Errorf("error getting services: %w", err)
		}

		// Extract the Service cluster IP addresses

		for _, svc := range services.Items {
			ips = append(ips, svc.Spec.ClusterIP)
		}
		log.Println("using service IPs:", ips)

	case Pod:
		err := waitForPodsRunning(k.clientset, k.labelselector)
		if err != nil {
			return fmt.Errorf("error waiting for pods to be in Running state: %w", err)
		}

		// Get all pods in the cluster with label app=agnhost
		pods, err := k.clientset.CoreV1().Pods(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{
			LabelSelector: k.labelselector,
		})
		if err != nil {
			return fmt.Errorf("error getting pods: %w", err)
		}

		for _, pod := range pods.Items {
			ips = append(ips, pod.Status.PodIP)
		}

		log.Printf("using pod IPs: %v", ips)
	default:
		return fmt.Errorf("env TARGET_TYPE must be \"service\" or \"pod\"")
	}

	k.ips = ips
	return nil
}

// waitForPodsRunning waits for all pods with the specified label to be in the Running phase
func waitForPodsRunning(clientset *kubernetes.Clientset, labelSelector string) error {
	return wait.ExponentialBackoff(wait.Backoff{
		Duration: 5 * time.Second,
		Factor:   1.5,
	}, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			log.Printf("error getting pods: %v", err)
			return false, nil
		}

		for _, pod := range pods.Items {
			if pod.Status.Phase != corev1.PodRunning {
				log.Printf("waiting for pod %s to be in Running state (currently %s)", pod.Name, pod.Status.Phase)
				return false, nil
			}
		}

		return true, nil
	})
}
