package kubernetes

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/microsoft/retina/test/e2e/framework/generic"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
)

func TestCandidateImageRendering(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	for _, profile := range []string{"standard", "hubble"} {
		t.Run(profile, func(t *testing.T) {
			chart, err := loader.Load(filepath.Join(root, "deploy", profile, "manifests", "controller", "helm", "retina"))
			if err != nil {
				t.Fatal(err)
			}
			for _, form := range []string{":v1", "@sha256:" + strings.Repeat("a", 64), ":v1@sha256:" + strings.Repeat("b", 64)} {
				t.Run(form, func(t *testing.T) {
					want := []string{}
					for _, image := range []struct {
						env  string
						name string
					}{
						{generic.AgentImageEnv, "retina-agent"},
						{generic.WindowsAgentImageEnv, "retina-windows-agent"},
						{generic.InitImageEnv, "retina-init"},
						{generic.OperatorImageEnv, "retina-operator"},
					} {
						ref := "example.azurecr.io/" + image.name + form
						t.Setenv(image.env, ref)
						want = append(want, ref)
					}
					values := map[string]interface{}{
						"operator": map[string]interface{}{"enabled": true},
						"os":       map[string]interface{}{"linux": true, "windows": true},
					}
					if err := applyImageOverrides(values, profile == "hubble"); err != nil {
						t.Fatal(err)
					}
					// An advanced upgrade must preserve the selected image values.
					values = chartutil.CoalesceTables(map[string]interface{}{"enablePodLevel": true}, values)
					renderValues, err := chartutil.ToRenderValues(chart, values, chartutil.ReleaseOptions{Name: "retina", Namespace: "kube-system"}, nil)
					if err != nil {
						t.Fatal(err)
					}
					rendered, err := engine.Render(chart, renderValues)
					if err != nil {
						t.Fatal(err)
					}
					var output strings.Builder
					for _, text := range rendered {
						output.WriteString(text)
					}
					for _, ref := range want {
						if !strings.Contains(output.String(), `image: "`+ref+`"`) {
							t.Fatalf("candidate %q missing from rendered chart", ref)
						}
					}
				})
			}
		})
	}
}

func TestEmptyImageOverrides(t *testing.T) {
	for _, env := range []string{generic.AgentImageEnv, generic.WindowsAgentImageEnv, generic.InitImageEnv, generic.OperatorImageEnv} {
		t.Setenv(env, "")
	}
	values := map[string]interface{}{}
	if err := applyImageOverrides(values, false); err != nil {
		t.Fatal(err)
	}
	if len(values) != 0 {
		t.Fatalf("empty overrides changed default values: %v", values)
	}
	root := filepath.Join("..", "..", "..", "..")
	for _, profile := range []string{"standard", "hubble"} {
		chart, err := loader.Load(filepath.Join(root, "deploy", profile, "manifests", "controller", "helm", "retina"))
		if err != nil {
			t.Fatal(err)
		}
		values := map[string]interface{}{
			"operator": map[string]interface{}{"enabled": true},
			"os":       map[string]interface{}{"linux": true, "windows": true},
		}
		renderValues, err := chartutil.ToRenderValues(chart, values, chartutil.ReleaseOptions{Name: "retina", Namespace: "kube-system"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		rendered, err := engine.Render(chart, renderValues)
		if err != nil {
			t.Fatal(err)
		}
		agentKey := "image"
		if profile == "hubble" {
			agentKey = "agent"
		}
		agent := chart.Values[agentKey].(map[string]interface{})
		operator := chart.Values["operator"].(map[string]interface{})
		initRepository, initTag := agent["initRepository"], agent["tag"]
		if profile == "hubble" {
			init := agent["init"].(map[string]interface{})
			initRepository, initTag = init["repository"], init["tag"]
		}
		expected := map[string]bool{
			fmt.Sprintf("%s:%s", agent["repository"], agent["tag"]):       true,
			fmt.Sprintf("%s:%s", initRepository, initTag):                 true,
			fmt.Sprintf("%s:%s", operator["repository"], operator["tag"]): true,
		}
		found := 0
		for _, text := range rendered {
			for _, line := range strings.Split(text, "\n") {
				line = strings.TrimPrefix(strings.TrimSpace(line), "- ")
				if strings.HasPrefix(line, "image: ") && strings.Contains(line, "/retina-") {
					found++
					ref := strings.Trim(strings.TrimPrefix(line, "image: "), `"`)
					if !expected[ref] {
						t.Fatalf("invalid default image: %s", line)
					}
				}
			}
		}
		if found != 4 {
			t.Fatalf("%s chart rendered %d Retina images, want 4", profile, found)
		}
	}
}

func TestInvalidImageOverride(t *testing.T) {
	t.Setenv(generic.AgentImageEnv, "invalid image")
	if err := applyImageOverrides(map[string]interface{}{}, false); err == nil {
		t.Fatal("invalid override was accepted")
	}
}
