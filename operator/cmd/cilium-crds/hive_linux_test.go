//go:build unit

package ciliumcrds_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cilium/cilium/pkg/hive"
	"github.com/cilium/hive/hivetest"
	"github.com/microsoft/retina/operator/cilium-crds/config"
	ciliumcrds "github.com/microsoft/retina/operator/cmd/cilium-crds"
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

// TestOperatorHiveResolves catches missing providers in both graphs: Populate
// resolves always-on cells, and cell.Decorate resolves leader-only cells.
func TestOperatorHiveResolves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kubeconfig")
	if err := os.WriteFile(path, []byte(unreachableKubeconfig), 0o600); err != nil {
		t.Fatalf("writing kubeconfig: %v", err)
	}
	t.Setenv("KUBECONFIG", path)

	h := hive.New(ciliumcrds.Operator)
	h.Viper().Set("k8s-kubeconfig-path", path)
	// registerOperatorHooks rejects an empty namespace during Populate.
	hive.AddConfigOverride(h, func(c *config.Config) { c.LeaderElectionNamespace = "kube-system" })

	if err := h.Populate(hivetest.Logger(t)); err != nil {
		t.Fatalf("operator hive failed to resolve: %v", err)
	}
}
