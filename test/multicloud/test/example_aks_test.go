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
			"prefix":              "test",
			"location":            "uksouth",
			"subscription_id":     "d6050d84-e4dd-463d-afc7-a6ab3dc33ab7", // TODO: replace with actual project once we get azure "public" access
			"tenant_id":           "ac8a4ccd-35f1-4f95-a688-f68e3d89adfc",
			"resource_group_name": "test",
			"labels": map[string]string{
				"environment": "test",
				"owner":       "test",
				"project":     "test",
			},
		},
	}

	// clean up at the end of the test
	defer terraform.Destroy(t, opts)

	terraform.Init(t, opts)

	// TODO: uncomment once we get creds for azure "public"
	// terraform.Apply(t, opts)

	// TODO: add actual tests here
}
