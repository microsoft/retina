package scaletest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestObservedTrafficTargets(t *testing.T) {
	logs := `
2026/08/26 response from http://10.0.0.10:8080/: ok
2026/08/26 response from http://10.0.0.20:8080/: ok
2026/08/26 error making request: timeout
`
	targets := map[string]string{
		"10.0.0.10": "linux",
		"10.0.0.20": "windows",
		"10.0.0.30": "windows",
	}

	require.Equal(t, map[string]bool{
		"linux":   true,
		"windows": true,
	}, observedTrafficTargets(logs, targets))
}
