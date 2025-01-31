package test

import (
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/terraform"
)

func TestRetinaKindIntegration(t *testing.T) {
	t.Parallel()

	opts := &terraform.Options{
		TerraformDir: examplesPath + "integration/retina-kind",

		Vars: map[string]interface{}{
			"prefix":         "test-integration",
			"retina_version": "v0.0.24",
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

	retinaPodSelector := PodSelector{
		Namespace:     "kube-system",
		LabelSelector: "k8s-app=retina",
		ContainerName: "retina",
	}

	timeOut := time.Duration(90) * time.Second
	// check the retina pods are running
	result, err := arePodsRunning(clientSet, retinaPodSelector, timeOut)
	if !result {
		t.Fatalf("Retina pods did not start in time: %v\n", err)
	}

	// check the retina pods logs for errors
	checkPodLogs(t, clientSet, retinaPodSelector)

	// TODO: add more tests here
}
