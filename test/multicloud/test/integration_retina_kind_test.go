package test

import (
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
)

func TestRetinaKindIntegration(t *testing.T) {
	t.Parallel()

	opts := &terraform.Options{
		TerraformDir: "../examples/integration/retina-kind",

		Vars: map[string]interface{}{
			"prefix":         "test-integration",
			"retina_version": "v0.0.24",
		},
	}

	// clean up at the end of the test
	defer terraform.Destroy(t, opts)

	terraform.Init(t, opts)
	terraform.Apply(t, opts)

	// TODO: add actual tests here
	// test the cluster is accessible with the ca cert, client cert and client key
	caCert := terraform.Output(t, opts, "cluster_ca_certificate")
	clientCert := terraform.Output(t, opts, "client_certificate")
	clientKey := terraform.Output(t, opts, "client_key")
	host := terraform.Output(t, opts, "host")

	// test the cluster is accessible with the ca cert, client cert and client key
	checkClusterAccess(t, caCert, clientCert, clientKey, host)

	// create a Kubernetes clientset
	clientSet, err := buildClientSet(caCert, clientCert, clientKey, host)
	if err != nil {
		t.Fatalf("Failed to create Kubernetes clientset: %v", err)
	}

	// check the retina pods logs for errors
	checkRetinaLogs(t, clientSet)

	// TODO: add more tests here
}
