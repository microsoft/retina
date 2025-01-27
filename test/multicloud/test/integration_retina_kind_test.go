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
			"prefix":         "test",
			"retina_version": "v0.0.24",
		},
	}

	// clean up at the end of the test
	defer terraform.Destroy(t, opts)

	terraform.Init(t, opts)
	terraform.Apply(t, opts)

	// TODO: add actual tests here
}
