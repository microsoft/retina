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
			"location":     "eu-central1",
			"project":      "mc-retina", // TODO: replace with actual project once we get gcloud access
			"machine_type": "e2-standard-4",
		},
	}

	// clean up at the end of the test
	defer terraform.Destroy(t, opts)

	terraform.Init(t, opts)

	// TODO: uncomment once we get creds for gcloud
	// terraform.Apply(t, opts)

	// TODO: add actual tests here
}
