package test

import (
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/microsoft/retina/test/multicloud/test/utils"
)

func TestAKSExample(t *testing.T) {
	t.Parallel()

	opts := &terraform.Options{
		TerraformDir: utils.ExamplesPath + "aks",

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
	caCert := utils.FetchSensitiveOutput(t, opts, "cluster_ca_certificate")
	clientCert := utils.FetchSensitiveOutput(t, opts, "client_certificate")
	clientKey := utils.FetchSensitiveOutput(t, opts, "client_key")
	host := utils.FetchSensitiveOutput(t, opts, "host")

	// decode the base64 encoded strings
	caCertDecoded := utils.DecodeBase64(t, caCert)
	clientCertDecoded := utils.DecodeBase64(t, clientCert)
	clientKeyDecoded := utils.DecodeBase64(t, clientKey)

	// build the REST config
	restConfig := utils.CreateRESTConfigWithClientCert(caCertDecoded, clientCertDecoded, clientKeyDecoded, host)

	// create a Kubernetes clientset
	clientSet, err := utils.BuildClientSet(restConfig)
	if err != nil {
		t.Fatalf("Failed to create Kubernetes clientset: %v", err)
	}

	// test the cluster is accessible
	utils.TestClusterAccess(t, clientSet)
}
