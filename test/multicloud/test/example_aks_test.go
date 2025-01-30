package test

import (
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
)

func TestAKSExample(t *testing.T) {
	t.Parallel()

	opts := &terraform.Options{
		TerraformDir: "../examples/aks",

		Vars: map[string]interface{}{
			"prefix":              "test-mc",
			"location":            "uksouth",
			"resource_group_name": "test-mc",
			"labels": map[string]string{
				"environment": "test",
				"owner":       "test",
				"project":     "test",
			},
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

	// decode the base64 encoded strings
	caCertDecoded := decodeBase64(t, caCert)
	clientCertDecoded := decodeBase64(t, clientCert)
	clientKeyDecoded := decodeBase64(t, clientKey)

	// build the REST config
	restConfig := createRESTConfigWithClientCert(caCertDecoded, clientCertDecoded, clientKeyDecoded, host)

	// create a Kubernetes clientset
	clientSet, err := buildClientSet(restConfig)
	if err != nil {
		t.Fatalf("Failed to create Kubernetes clientset: %v", err)
	}

	// test the cluster is accessible
	testClusterAccess(t, clientSet)
}
