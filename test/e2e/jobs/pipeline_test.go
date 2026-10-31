package retina

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestE2EUsesSelectedImageTagInsteadOfBuildTag(t *testing.T) {
	data, err := os.ReadFile("../../../.azure-pipelines/ci.yml")
	require.NoError(t, err)

	var document yaml.Node
	require.NoError(t, yaml.Unmarshal(data, &document))

	var step struct {
		Env    map[string]yaml.Node `yaml:"env"`
		Inputs struct {
			InlineScript string `yaml:"inlineScript"`
		} `yaml:"inputs"`
	}
	found := false
	nodes := []*yaml.Node{&document}
	for len(nodes) > 0 {
		node := nodes[len(nodes)-1]
		nodes = nodes[:len(nodes)-1]
		if node.Kind == yaml.MappingNode {
			for index := 0; index < len(node.Content); index += 2 {
				if node.Content[index].Value == "displayName" && node.Content[index+1].Value == "Run E2E Tests" {
					require.NoError(t, node.Decode(&step))
					found = true
				}
			}
		}
		nodes = append(nodes, node.Content...)
	}
	require.True(t, found, "Run E2E Tests pipeline step must exist")

	for _, tc := range []struct {
		name string
		tag  string
	}{
		{name: "prebuilt release candidate", tag: "v1.2.9-1"},
		{name: "source tag after manifest creation", tag: "0fbc6190"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := append(os.Environ(),
				"IMAGE_REGISTRY=registry.example.com",
				"IMAGE_NAMESPACE=candidate/retina",
			)
			for name, value := range step.Env {
				if value.Kind == yaml.ScalarNode && value.Value == "$(e2eImageTag)" {
					env = append(env, name+"="+tc.tag)
				}
			}
			// Reproduce the build TAG observed inside the failed OneBranch task.
			env = append(env, "TAG=96a9d869-129485")
			const mocks = `
az() { printf '%s\n' test-subscription; }
go() {
  printf 'TAG=%s\n' "$TAG"
  printf '%s\n' "$@"
}
`
			command := exec.CommandContext(t.Context(), "bash")
			command.Env = env
			command.Stdin = strings.NewReader(mocks + step.Inputs.InlineScript)
			output, err := command.CombinedOutput()
			require.NoError(t, err, "%s", output)
			require.Contains(t, string(output), "TAG="+tc.tag+"\n")
			require.Contains(t, string(output), "-image-tag="+tc.tag+"\n")
			require.NotContains(t, string(output), "96a9d869-129485")
		})
	}
}
