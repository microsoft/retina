package retina

import (
	"testing"

	"github.com/microsoft/retina/test/e2e/framework/generic"
	"github.com/stretchr/testify/require"
)

func TestEventWriterImage(t *testing.T) {
	t.Setenv(generic.DefaultImageRegistry, "registry.example.com")
	t.Setenv(generic.DefaultImageNamespace, "candidate/retina")
	t.Setenv(generic.DefaultTagEnv, "v1.2.9")

	for _, tc := range []struct {
		name     string
		override string
		want     string
	}{
		{
			name: "default",
			want: "registry.example.com/candidate/retina/test/e2e-test-event-writer:v1.2.9",
		},
		{
			name:     "independent tag",
			override: "registry.example.com/helpers/event-writer:stable",
			want:     "registry.example.com/helpers/event-writer:stable",
		},
		{
			name:     "digest",
			override: "registry.example.com/helpers/event-writer@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			want:     "registry.example.com/helpers/event-writer@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("EVENT_WRITER_IMAGE", tc.override)
			require.Equal(t, tc.want, eventWriterImage())
		})
	}
}
