package main

import (
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/microsoft/retina/hack/tools/kapinger/clients"
	"github.com/microsoft/retina/hack/tools/kapinger/servers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	delay = 500 * time.Millisecond
)

func main() {
	log.Printf("starting kapinger...")
	clientset, err := getKubernetesClientSet()
	if err != nil {
		log.Fatal(err)
	}

	httpPort, err := strconv.Atoi(os.Getenv(servers.EnvHTTPPort))
	if err != nil {
		httpPort = servers.HTTPPort
		log.Printf("HTTP_PORT not set, defaulting to port %d\n", servers.HTTPPort)
	}

	go servers.StartAll()

	// Create an HTTP client with the custom Transport
	client, err := clients.NewKapingerHTTPClient(clientset, "app=kapinger", httpPort)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize the random number generator with a seed based on the current time
	rand.New(rand.NewSource(time.Now().UnixNano()))

	// Generate a random number between 1 and 1000 for delay jitter
	jitter := rand.Intn(100) + 1
	time.Sleep(time.Duration(jitter) * time.Millisecond)

	for {
		err := client.MakeRequest()
		if err != nil {
			log.Printf("error making request: %v", err)
		}
		time.Sleep(delay)
	}
}

func getKubernetesClientSet() (*kubernetes.Clientset, error) {
	// Use the in-cluster configuration
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("error getting in-cluster config: %v", err)
	}

	// Create a Kubernetes clientset using the in-cluster configuration
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("error creating clientset: %v", err)
	}
	return clientset, err
}
