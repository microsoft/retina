//go:build pipeline

package pipelines

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func readPipeline(t *testing.T) *yaml.Node {
	t.Helper()
	data, err := os.ReadFile("../../.azure-pipelines/ci.yml")
	require.NoError(t, err)

	var document yaml.Node
	require.NoError(t, yaml.Unmarshal(data, &document))
	return &document
}

func findPipelineStep(t *testing.T, document *yaml.Node, displayName string) *yaml.Node {
	t.Helper()
	nodes := []*yaml.Node{document}
	for len(nodes) > 0 {
		node := nodes[len(nodes)-1]
		nodes = nodes[:len(nodes)-1]
		if node.Kind == yaml.MappingNode {
			for index := 0; index < len(node.Content); index += 2 {
				if node.Content[index].Value == "displayName" && node.Content[index+1].Value == displayName {
					return node
				}
			}
		}
		nodes = append(nodes, node.Content...)
	}
	t.Fatalf("%s pipeline step must exist", displayName)
	return nil
}

func TestPipelineUsesSelectedImageTagInsteadOfBuildTag(t *testing.T) {
	var step struct {
		Env    map[string]yaml.Node `yaml:"env"`
		Inputs struct {
			InlineScript string `yaml:"inlineScript"`
		} `yaml:"inputs"`
	}
	require.NoError(t, findPipelineStep(t, readPipeline(t), "Run E2E Tests").Decode(&step))

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

func TestPrebuiltImageInputsAreOptionalAtQueueTimeButRequiredInPrebuiltMode(t *testing.T) {
	document := readPipeline(t)
	var pipeline struct {
		Parameters []struct {
			Name    string `yaml:"name"`
			Default string `yaml:"default"`
		} `yaml:"parameters"`
	}
	require.NoError(t, document.Decode(&pipeline))

	inputs := []struct {
		parameter string
		env       string
		value     string
	}{
		{"imageRegistry", "IMAGE_REGISTRY", "registry.example.com"},
		{"imageNamespace", "IMAGE_NAMESPACE", "candidate/retina"},
		{"eventWriterImage", "EVENT_WRITER_IMAGE", "registry.example.com/helpers/event-writer:stable"},
	}
	t.Setenv("E2E_IMAGE_TAG", "v1.2.9")
	defaults := make(map[string]string)
	for _, parameter := range pipeline.Parameters {
		defaults[parameter.Name] = parameter.Default
	}

	var step struct {
		Bash string `yaml:"bash"`
	}
	require.NoError(t, findPipelineStep(t, document, "Validate prebuilt image inputs").Decode(&step))
	require.NotEmpty(t, step.Bash)

	for _, input := range inputs {
		t.Run(input.parameter+" left at default", func(t *testing.T) {
			value, exists := defaults[input.parameter]
			require.True(t, exists)
			require.NotEmpty(t, value, "Azure DevOps requires a nonempty queue-time default even when prebuilt mode is disabled")
			for _, configured := range inputs {
				t.Setenv(configured.env, configured.value)
			}
			t.Setenv(input.env, value)
			command := exec.CommandContext(t.Context(), "bash")
			command.Stdin = strings.NewReader(step.Bash)
			output, err := command.CombinedOutput()
			require.Error(t, err, "prebuilt mode must reject an unspecified %s", input.parameter)
			require.Contains(t, string(output), "##vso[task.logissue type=error]")
		})
	}

	t.Run("prebuilt resource tag not selected", func(t *testing.T) {
		for _, input := range inputs {
			t.Setenv(input.env, input.value)
		}
		t.Setenv("E2E_IMAGE_TAG", "not-set")
		command := exec.CommandContext(t.Context(), "bash")
		command.Stdin = strings.NewReader(step.Bash)
		output, err := command.CombinedOutput()
		require.Error(t, err, "prebuilt mode must reject the default resource tag")
		require.Contains(t, string(output), "##vso[task.logissue type=error]")
	})

	t.Run("explicit prebuilt images", func(t *testing.T) {
		for _, input := range inputs {
			t.Setenv(input.env, input.value)
		}
		command := exec.CommandContext(t.Context(), "bash")
		command.Stdin = strings.NewReader(step.Bash)
		output, err := command.CombinedOutput()
		require.NoError(t, err, "%s", output)
	})
}

func TestPipelineUsesSingleModeSelector(t *testing.T) {
	var pipeline struct {
		Parameters []struct {
			Name    string   `yaml:"name"`
			Type    string   `yaml:"type"`
			Default string   `yaml:"default"`
			Values  []string `yaml:"values"`
		} `yaml:"parameters"`
	}
	require.NoError(t, readPipeline(t).Decode(&pipeline))

	found := false
	for _, parameter := range pipeline.Parameters {
		require.NotContains(t, []string{"runE2EValidation", "usePrebuiltImages"}, parameter.Name,
			"independent E2E switches must not remain on the Parameters page")
		if parameter.Name == "e2eMode" {
			found = true
			require.Equal(t, "string", parameter.Type)
			require.Equal(t, "Build only", parameter.Default)
			require.ElementsMatch(t, []string{
				"Build only",
				"Publish branch images and run E2E",
				"Run E2E with prebuilt images",
			}, parameter.Values)
		}
	}
	require.True(t, found, "one E2E mode selector must replace the independent checkboxes")
}

func TestWindowsPRChecksCoverBuildFlowAndNativeImages(t *testing.T) {
	data, err := os.ReadFile("../../.github/workflows/test.yaml")
	require.NoError(t, err)
	type step struct {
		Name string            `yaml:"name"`
		Uses string            `yaml:"uses"`
		Run  string            `yaml:"run"`
		With map[string]string `yaml:"with"`
	}
	var workflow struct {
		Jobs map[string]struct {
			RunsOn   string `yaml:"runs-on"`
			Needs    string `yaml:"needs"`
			Strategy struct {
				Matrix struct {
					Year []string `yaml:"year"`
				} `yaml:"matrix"`
			} `yaml:"strategy"`
			Steps []step `yaml:"steps"`
		} `yaml:"jobs"`
	}
	require.NoError(t, yaml.Unmarshal(data, &workflow))

	unit, exists := workflow.Jobs["test-windows"]
	require.True(t, exists)
	require.Equal(t, "windows-${{ matrix.year }}", unit.RunsOn)
	require.ElementsMatch(t, []string{"2022", "2025"}, unit.Strategy.Matrix.Year)
	var runsUnitTests bool
	for _, s := range unit.Steps {
		if strings.TrimSpace(s.Run) == "make test-windows" {
			runsUnitTests = true
		}
	}
	require.True(t, runsUnitTests, "both Windows runners must execute the unit-test suite")

	build, exists := workflow.Jobs["test-windows-build"]
	require.True(t, exists)
	require.Equal(t, "ubuntu-latest", build.RunsOn)
	var compile, upload *step
	for index := range build.Steps {
		s := &build.Steps[index]
		if strings.Contains(s.Run, "make build-windows-binaries") {
			compile = s
		}
		if strings.HasPrefix(s.Uses, "actions/upload-artifact@") {
			upload = s
		}
	}
	require.NotNil(t, compile, "both Windows images must consume shared prebuilt binaries")
	require.Contains(t, compile.Run, "APP_INSIGHTS_ID=")
	require.NotNil(t, upload)
	require.Equal(t, "windows-test-binaries", upload.With["name"])
	require.Equal(t, "output/windows_amd64/", upload.With["path"])
	require.Equal(t, "error", upload.With["if-no-files-found"])

	images, exists := workflow.Jobs["test-windows-images"]
	require.True(t, exists)
	require.Equal(t, "test-windows-build", images.Needs)
	require.Equal(t, "windows-${{ matrix.year }}", images.RunsOn)
	require.ElementsMatch(t, []string{"2022", "2025"}, images.Strategy.Matrix.Year)
	var download, packaging, hardening *step
	for index := range images.Steps {
		s := &images.Steps[index]
		require.NotContains(t, s.Run, "docker push", "PR validation must not publish images")
		require.NotContains(t, s.Run, "--push", "PR validation must not publish images")
		if strings.HasPrefix(s.Uses, "actions/download-artifact@") {
			download = s
		}
		if strings.Contains(s.Run, "make retina-image-win") {
			packaging = s
		}
		if strings.Contains(s.Run, "EnableCertPaddingCheck") {
			hardening = s
		}
	}
	require.NotNil(t, download)
	require.Equal(t, upload.With["name"], download.With["name"])
	require.Equal(t, upload.With["path"], download.With["path"])
	require.NotNil(t, packaging, "native Windows runners must build the Windows images")
	require.Contains(t, packaging.Run, "WINDOWS_YEARS=${{ matrix.year }}")
	require.Contains(t, packaging.Run, "BINARIES_PATH=output/windows_amd64")
	require.NotNil(t, hardening, "validate registry hardening in the resulting image")
	require.Contains(t, hardening.Run, `HKLM:\Software\Microsoft\Cryptography\Wintrust\Config`)
	require.Contains(t, hardening.Run, `HKLM:\Software\Wow6432Node\Microsoft\Cryptography\Wintrust\Config`)
}
