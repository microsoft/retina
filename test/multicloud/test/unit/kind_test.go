package test

import (
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/microsoft/retina/test/multicloud/test/utils"
)

func TestKindExample(t *testing.T) {
	t.Parallel()

	opts := &terraform.Options{
		TerraformDir: utils.ExamplesPath + "kind",

		Vars: map[string]interface{}{
			"prefix": "test",
		},
	}

	// clean up at the end of the test
	defer terraform.Destroy(t, opts)
	terraform.InitAndApply(t, opts)

	// get outputs
	caCert := utils.FetchSensitiveOutput(t, opts, "cluster_ca_certificate")
	clientCert := utils.FetchSensitiveOutput(t, opts, "client_certificate")
	clientKey := utils.FetchSensitiveOutput(t, opts, "client_key")
	host := utils.FetchSensitiveOutput(t, opts, "host")

	// build the REST config
	restConfig := utils.CreateRESTConfigWithClientCert(caCert, clientCert, clientKey, host)

	// create a Kubernetes clientset
	clientSet, err := utils.BuildClientSet(restConfig)
	if err != nil {
		t.Fatalf("Failed to create Kubernetes clientset: %v", err)
	}

	// test the cluster is accessible
	utils.TestClusterAccess(t, clientSet)
}
