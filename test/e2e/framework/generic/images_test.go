package generic

import (
	"strings"
	"testing"
)

func TestImage(t *testing.T) {
	t.Setenv(DefaultImageRegistry, "ghcr.io")
	t.Setenv(DefaultImageNamespace, "microsoft/retina")
	t.Setenv(DefaultTagEnv, "v1.2.9")
	for _, image := range []string{
		"",
		"example.azurecr.io/retina-agent:v1",
		"example.azurecr.io/retina-agent@sha256:" + strings.Repeat("a", 64),
		"example.azurecr.io/retina-agent:v1@sha256:" + strings.Repeat("a", 64),
	} {
		t.Run(image, func(t *testing.T) {
			t.Setenv(AgentImageEnv, image)
			got, err := Image(AgentImageEnv, "retina-agent")
			if err != nil {
				t.Fatal(err)
			}
			want := image
			if want == "" {
				want = "ghcr.io/microsoft/retina/retina-agent:v1.2.9"
			}
			if got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
	for _, image := range []string{"example.azurecr.io/retina-agent", "bad image:v1", "example.azurecr.io/retina-agent@sha256:bad"} {
		t.Setenv(AgentImageEnv, image)
		if _, err := Image(AgentImageEnv, "retina-agent"); err == nil {
			t.Fatalf("accepted invalid reference %q", image)
		}
	}
}
