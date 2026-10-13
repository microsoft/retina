package kubernetes

import (
	"fmt"
	"os"

	"github.com/microsoft/retina/test/e2e/framework/generic"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func applyImageOverrides(values map[string]interface{}, hubble bool) error {
	agentPath := []string{"image", "override"}
	initPath := []string{"image", "initOverride"}
	windowsPath := []string{"image", "windowsOverride"}
	if hubble {
		agentPath = []string{"agent", "override"}
		initPath = []string{"agent", "init", "override"}
		windowsPath = []string{"agent", "windowsOverride"}
	}
	for _, override := range []struct {
		env       string
		component string
		path      []string
	}{
		{generic.AgentImageEnv, "retina-agent", agentPath},
		{generic.InitImageEnv, "retina-init", initPath},
		{generic.OperatorImageEnv, "retina-operator", []string{"operator", "override"}},
		{generic.WindowsAgentImageEnv, "retina-agent", windowsPath},
	} {
		if os.Getenv(override.env) == "" {
			continue
		}
		image, err := generic.Image(override.env, override.component)
		if err != nil {
			return err
		}
		if err := unstructured.SetNestedField(values, image, override.path...); err != nil {
			return fmt.Errorf("set %s: %w", override.env, err)
		}
	}
	return nil
}
