//go:build unit

package hubble_test

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/cilium/cilium/pkg/hive"
	"github.com/cilium/hive/hivetest"
	"github.com/microsoft/retina/cmd/hubble"
	"github.com/microsoft/retina/pkg/config"
)

// Populate builds a rest.Config but never dials this address.
const unreachableKubeconfig = `apiVersion: v1
kind: Config
clusters:
- name: test
  cluster: {server: https://127.0.0.1:1}
contexts:
- name: test
  context: {cluster: test, user: test}
current-context: test
users:
- name: test
  user: {token: fake}
`

// controller-runtime registers controller names process-wide, so skip repeated
// Populate calls from go test -count.
var populateRan atomic.Bool

// TestAgentHiveResolves catches missing providers after Cilium upgrades; dig
// cannot detect obsolete providers because it permits unconsumed values.
func TestAgentHiveResolves(t *testing.T) {
	if !populateRan.CompareAndSwap(false, true) {
		t.Skip("hive Populate is once-per-process (controller-runtime keeps a global controller-name registry)")
	}

	path := filepath.Join(t.TempDir(), "kubeconfig")
	if err := os.WriteFile(path, []byte(unreachableKubeconfig), 0o600); err != nil {
		t.Fatalf("writing kubeconfig: %v", err)
	}
	t.Setenv("KUBECONFIG", path)

	h := hive.New(hubble.Agent)
	// Cilium's client reads this flag instead of KUBECONFIG.
	h.Viper().Set("k8s-kubeconfig-path", path)
	// Populate binds controller-runtime listeners but never closes the manager;
	// disable them while keeping typed coverage of the config fields.
	hive.AddConfigOverride(h, func(c *config.RetinaHubbleConfig) {
		c.MetricsBindAddress = "0"
		c.HealthProbeBindAddress = "0"
	})

	if err := h.Populate(hivetest.Logger(t)); err != nil {
		t.Fatalf("agent hive failed to resolve: %v", err)
	}
}
