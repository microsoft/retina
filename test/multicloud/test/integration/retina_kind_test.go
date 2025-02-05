package test

import (
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/microsoft/retina/test/multicloud/test/utils"
)

func TestRetinaKindIntegration(t *testing.T) {
	t.Parallel()

	opts := &terraform.Options{
		TerraformDir: utils.ExamplesPath + "integration/retina-kind",

		Vars: map[string]interface{}{
			"prefix":               "test-integration",
			"retina_chart_version": utils.RetinaVersion,
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

	retinaPodSelector := utils.PodSelector{
		Namespace:     "kube-system",
		LabelSelector: "k8s-app=retina",
		ContainerName: "retina",
	}

	timeOut := time.Duration(90) * time.Second
	// check the retina pods are running
	result, err := utils.ArePodsRunning(clientSet, retinaPodSelector, timeOut)
	if !result {
		t.Fatalf("Retina pods did not start in time: %v\n", err)
	}

	// check the retina pods logs for errors
	utils.CheckPodLogs(t, clientSet, retinaPodSelector)

	// TODO: add more tests here
}
