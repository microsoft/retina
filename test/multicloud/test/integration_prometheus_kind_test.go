package test

import (
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/terraform"
)

func TestPrometheusKindIntegration(t *testing.T) {
	t.Parallel()

	opts := &terraform.Options{
		TerraformDir: examplesPath + "integration/prometheus-kind",

		Vars: map[string]interface{}{
			"prefix": "test-integration",
		},
	}

	// clean up at the end of the test
	defer terraform.Destroy(t, opts)
	terraform.InitAndApply(t, opts)

	// get outputs
	caCert := fetchSensitiveOutput(t, opts, "cluster_ca_certificate")
	clientCert := fetchSensitiveOutput(t, opts, "client_certificate")
	clientKey := fetchSensitiveOutput(t, opts, "client_key")
	host := fetchSensitiveOutput(t, opts, "host")

	// build the REST config
	restConfig := createRESTConfigWithClientCert(caCert, clientCert, clientKey, host)

	// create a Kubernetes clientset
	clientSet, err := buildClientSet(restConfig)
	if err != nil {
		t.Fatalf("Failed to create Kubernetes clientset: %v", err)
	}

	// test the cluster is accessible
	testClusterAccess(t, clientSet)

	podSelector := PodSelector{
		Namespace:     "default",
		LabelSelector: "app.kubernetes.io/instance=prometheus-kube-prometheus-prometheus",
		ContainerName: "prometheus",
	}

	timeOut := time.Duration(60) * time.Second
	// check the prometheus pods are running
	result, err := arePodsRunning(clientSet, podSelector, timeOut)
	if !result {
		t.Fatalf("Prometheus pods did not start in time: %v\n", err)
	}

	// check the retina pods logs for errors
	checkPodLogs(t, clientSet, podSelector)

	// TODO: add more tests here
}
