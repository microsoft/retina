package test

import (
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
)

func TestKindExample(t *testing.T) {
	t.Parallel()

	opts := &terraform.Options{
		TerraformDir: "../examples/kind",

		Vars: map[string]interface{}{
			"prefix": "test",
		},
	}

	// clean up at the end of the test
	defer terraform.Destroy(t, opts)

	terraform.Init(t, opts)
	terraform.Apply(t, opts)

	// TODO: add actual tests here
}
