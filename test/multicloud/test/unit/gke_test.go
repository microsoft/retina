package test

import (
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/microsoft/retina/test/multicloud/test/utils"
)

func TestGKEExample(t *testing.T) {
	t.Parallel()

	opts := &terraform.Options{
		TerraformDir: utils.ExamplesPath + "gke",

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
	caCert := utils.FetchSensitiveOutput(t, opts, "cluster_ca_certificate")
	host := utils.FetchSensitiveOutput(t, opts, "host")
	token := utils.FetchSensitiveOutput(t, opts, "access_token")

	// decode the base64 encoded cert
	caCertString := utils.DecodeBase64(t, caCert)

	// build the REST config
	restConfig := utils.CreateRESTConfigWithBearer(caCertString, token, host)

	// create a Kubernetes clientset
	clientSet, err := utils.BuildClientSet(restConfig)
	if err != nil {
		t.Fatalf("Failed to create Kubernetes clientset: %v", err)
	}

	// test the cluster is accessible
	utils.TestClusterAccess(t, clientSet)
}
