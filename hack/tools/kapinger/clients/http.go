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
	urls          []string
	port          int
	targettype    TargetType
	volume        int
	interval      time.Duration
}

func NewKapingerHTTPClient(clientset *kubernetes.Clientset, labelselector string, volume int, interval time.Duration, httpPort int) (*KapingerHTTPClient, error) {
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
		volume:        volume,
		interval:      interval,
	}

	targettype := os.Getenv(envTargetType)
	if targettype != "" {
		k.targettype = TargetType(targettype)
	} else {
		k.targettype = Service
	}

	var err error
	switch k.targettype {
	case Service:
		k.urls, err = k.getServiceURLs()
		if err != nil {
			return nil, fmt.Errorf("error getting service URLs: %w", err)
		}

	case Pod:
		k.urls, err = k.getPodURLs()

	default:
		return nil, fmt.Errorf("env TARGET_TYPE must be \"service\" or \"pod\"")
	}
	if err != nil {
		return nil, fmt.Errorf("error getting IPs: %w", err)
	}

	return &k, nil
}

func (k *KapingerHTTPClient) MakeRequests(ctx context.Context) error {
	ticker := time.NewTicker(k.interval)
	for {
		select {
		case <-ctx.Done():
			log.Printf("HTTP client context done")
			return nil
		case <-ticker.C:
			go func() {
				for i := 0; i < k.volume; i++ {
					for _, url := range k.urls {
						body, err := k.makeRequest(ctx, url)
						if err != nil {
							log.Printf("error making request: %v", err)
						} else {
							log.Printf("response from %s: %s\n", url, string(body))
						}
					}
				}
			}()
		}
	}
}

func (k *KapingerHTTPClient) makeRequest(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
	if err != nil {
		return nil, err
	}

	// Set the "Connection" header to "close"
	req.Header.Set("Connection", "close")

	// Send the request
	resp, err := k.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error reading response body from %s: %v", url, err)
		return nil, err
	}

	return body, nil
}

func (k *KapingerHTTPClient) getServiceURLs() ([]string, error) {
	urls := []string{}
	services, err := k.clientset.CoreV1().Services(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{
		LabelSelector: k.labelselector,
	})
	if err != nil {
		return urls, fmt.Errorf("error getting services: %w", err)
	}

	// Extract the Service cluster IP addresses

	for svc := range services.Items {
		urls = append(urls, fmt.Sprintf("http://%s.%s.svc.cluster.local:%d/", services.Items[svc].Name, services.Items[svc].Namespace, k.port))
	}
	log.Printf("using service URLs: %+v", urls)
	return urls, nil
}

func (k *KapingerHTTPClient) getPodURLs() ([]string, error) {
	urls := []string{}
	err := waitForPodsRunning(k.clientset, k.labelselector)
	if err != nil {
		return nil, fmt.Errorf("error waiting for pods to be in Running state: %w", err)
	}

	// Get all pods in the cluster with label app=agnhost
	pods, err := k.clientset.CoreV1().Pods(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{
		LabelSelector: k.labelselector,
	})
	if err != nil {
		return nil, fmt.Errorf("error getting pods: %w", err)
	}

	for _, pod := range pods.Items {
		urls = append(urls, fmt.Sprintf("http://%s:%d", pod.Status.PodIP, k.port))
	}
	log.Printf("using pod URL's: %+v", urls)
	return urls, nil
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
