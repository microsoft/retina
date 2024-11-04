package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/microsoft/retina/hack/tools/kapinger/clients"
	"github.com/microsoft/retina/hack/tools/kapinger/config"
	"github.com/microsoft/retina/hack/tools/kapinger/servers"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	log.Printf("starting kapinger...")
	clientset, err := getKubernetesClientSet()
	if err != nil {
		log.Fatal(err)
	}

	cfg := config.LoadConfigFromEnv()

	ctx := context.Background()
	go servers.StartAll(ctx, cfg)

	var kapingerClients []clients.Client

	// Create an HTTP httpclient with the custom Transport
	httpclient, err := clients.NewKapingerHTTPClient(clientset, "app=kapinger", cfg.BurstVolume, cfg.BurstInterval, cfg.HTTPPort)
	if err != nil {
		log.Fatal(err)
	}
	kapingerClients = append(kapingerClients, httpclient)

	// create and append a DNS client
	dnsclient := clients.NewKapingerDNSClient(cfg.BurstVolume, cfg.BurstInterval)
	kapingerClients = append(kapingerClients, dnsclient)

	// Initialize the random number generator with a seed based on the current time
	rand.New(rand.NewSource(time.Now().UnixNano()))

	// Generate a random number between 1 and 1000 for delay jitter
	jitter := rand.Intn(100) + 1
	time.Sleep(time.Duration(jitter) * time.Millisecond)

	g, gCtx := errgroup.WithContext(ctx)

	for _, client := range kapingerClients {
		client := client
		g.Go(func() error {
			err = client.MakeRequests(gCtx)
			if err != nil {
				return fmt.Errorf("error making request: %w", err)
			}
			return nil
		})
	}
	err = g.Wait()
	if err != nil {
		log.Fatalf("error making request: %v", err)
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
