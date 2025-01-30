package test

import (
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
)

func TestGKEExample(t *testing.T) {
	t.Parallel()

	opts := &terraform.Options{
		TerraformDir: "../examples/gke",

		Vars: map[string]interface{}{
			"prefix":       "test",
			"location":     "europe-west2", // London
			"project":      "mc-retina",    // TODO: replace with actual project once we get gcloud access
			"machine_type": "e2-standard-4",
		},
	}

	// clean up at the end of the test
	defer terraform.Destroy(t, opts)
	terraform.InitAndApply(t, opts)

	// get outputs
	caCert := fetchSensitiveOutput(t, opts, "cluster_ca_certificate")
	host := fetchSensitiveOutput(t, opts, "host")
	token := fetchSensitiveOutput(t, opts, "access_token")

	// decode the base64 encoded cert
	caCertString := decodeBase64(t, caCert)

	// build the REST config
	restConfig := createRESTConfigWithBearer(caCertString, token, host)

	// create a Kubernetes clientset
	clientSet, err := buildClientSet(restConfig)
	if err != nil {
		t.Fatalf("Failed to create Kubernetes clientset: %v", err)
	}

	// test the cluster is accessible
	testClusterAccess(t, clientSet)
}
