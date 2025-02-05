package test

import (
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/microsoft/retina/test/multicloud/test/utils"
)

func TestRetinaGKEIntegration(t *testing.T) {
	t.Parallel()

	opts := &terraform.Options{
		TerraformDir: utils.ExamplesPath + "integration/retina-gke",

		Vars: map[string]interface{}{
			"prefix":               "test",
			"location":             "europe-west2", // London
			"project":              "mc-retina",    // TODO: replace with actual project once we get gcloud access
			"machine_type":         "e2-standard-4",
			"retina_chart_version": utils.RetinaVersion,
			"retina_values": []map[string]interface{}{
				{
					"name":  "logLevel",
					"value": "info",
				},
				{
					"name":  "operator.tag",
					"value": utils.RetinaVersion,
				},
				// Example using a public image built during testing
				{
					"name":  "image.repository",
					"value": "acnpublic.azurecr.io/xiaozhiche320/retina/retina-agent",
				},
				{
					"name":  "image.tag",
					"value": "c17d5ea-linux-amd64",
				},
			},
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

	// TODO: uncomment once the log level for "iface not supported" is changed to WARN
	// check the retina pods logs for errors
	// utils.CheckPodLogs(t, clientSet, retinaPodSelector)

	// TODO: add more tests here
}
