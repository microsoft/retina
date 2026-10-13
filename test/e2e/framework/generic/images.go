package generic

import (
	"fmt"
	"os"

	"github.com/distribution/reference"
)

const (
	AgentImageEnv        = "RETINA_AGENT_IMAGE"
	WindowsAgentImageEnv = "RETINA_WINDOWS_AGENT_IMAGE"
	InitImageEnv         = "RETINA_INIT_IMAGE"
	OperatorImageEnv     = "RETINA_OPERATOR_IMAGE"
	KubectlImageEnv      = "KUBECTL_RETINA_IMAGE"
	KubectlShellImageEnv = "KUBECTL_RETINA_SHELL_IMAGE"
)

func Image(env, component string) (string, error) {
	if image := os.Getenv(env); image != "" {
		named, err := reference.ParseNormalizedNamed(image)
		if err != nil {
			return "", fmt.Errorf("invalid %s: %w", env, err)
		}
		if reference.IsNameOnly(named) {
			return "", fmt.Errorf("%s must include a tag or digest", env)
		}
		return image, nil
	}
	registry := os.Getenv(DefaultImageRegistry)
	namespace := os.Getenv(DefaultImageNamespace)
	tag := os.Getenv(DefaultTagEnv)
	if registry == "" || namespace == "" || tag == "" {
		return "", fmt.Errorf("image %s requires %s or registry, namespace and tag", component, env)
	}
	return registry + "/" + namespace + "/" + component + ":" + tag, nil
}
