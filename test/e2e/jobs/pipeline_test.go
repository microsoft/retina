package retina

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type pipelineStep struct {
	Bash   string               `yaml:"bash"`
	Env    map[string]yaml.Node `yaml:"env"`
	Inputs struct {
		InlineScript string `yaml:"inlineScript"`
	} `yaml:"inputs"`
}

func readPipelineStep(t *testing.T, pipeline, displayName string) pipelineStep {
	t.Helper()
	data, err := os.ReadFile("../../../.azure-pipelines/" + pipeline)
	require.NoError(t, err)

	var document yaml.Node
	require.NoError(t, yaml.Unmarshal(data, &document))

	var step pipelineStep
	found := false
	nodes := []*yaml.Node{&document}
	for len(nodes) > 0 {
		node := nodes[len(nodes)-1]
		nodes = nodes[:len(nodes)-1]
		if node.Kind == yaml.MappingNode {
			for index := 0; index < len(node.Content); index += 2 {
				if node.Content[index].Value == "displayName" && node.Content[index+1].Value == displayName {
					require.NoError(t, node.Decode(&step))
					found = true
				}
			}
		}
		nodes = append(nodes, node.Content...)
	}
	require.True(t, found, "%s pipeline step must exist in %s", displayName, pipeline)
	return step
}

func TestE2EUsesSelectedImageTagInsteadOfBuildTag(t *testing.T) {
	for _, pipeline := range []string{"ci.yml", "dalec-e2e.yml"} {
		t.Run(pipeline, func(t *testing.T) {
			checkE2ESelectedImageTag(t, readPipelineStep(t, pipeline, "Run E2E Tests"))
		})
	}
}

func checkE2ESelectedImageTag(t *testing.T, step pipelineStep) {
	t.Helper()
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

func TestDalecPipelineValidatesPrebuiltInputs(t *testing.T) {
	step := readPipelineStep(t, "dalec-e2e.yml", "Validate prebuilt image inputs")
	for _, tc := range []struct {
		name      string
		tag       string
		writer    string
		wantError string
	}{
		{name: "valid", tag: "candidate", writer: "registry.example.com/helper:v1"},
		{name: "missing tag", writer: "registry.example.com/helper:tag", wantError: "imageTag is required"},
		{name: "missing helper", tag: "candidate", wantError: "eventWriterImage is required"},
		{name: "whitespace", tag: "candidate tag", writer: "registry.example.com/helper:tag", wantError: "must not contain whitespace"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command := exec.CommandContext(t.Context(), "bash")
			command.Env = append(os.Environ(),
				"IMAGE_REGISTRY=registry.example.com",
				"IMAGE_NAMESPACE=dalec/candidate",
				"E2E_IMAGE_TAG="+tc.tag,
				"EVENT_WRITER_IMAGE="+tc.writer,
			)
			command.Stdin = strings.NewReader(step.Bash)
			output, err := command.CombinedOutput()
			if tc.wantError != "" {
				require.Error(t, err)
				require.Contains(t, string(output), tc.wantError)
			} else {
				require.NoError(t, err, "%s", output)
			}
		})
	}
}
