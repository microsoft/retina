//go:build dashboard && simplifydashboard

package dashboard

import (
	"testing"

	"os"
	"path/filepath"
)

// TestOverwriteDashboards simplifies and overwrites Grafana dashboards in this folder.
func TestOverwriteDashboards(t *testing.T) {
	// get all json's in this folder
	files, err := os.ReadDir("./")
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".json" {
			t.Logf("simplifying/overwriting dashboard: %s", file.Name())

			sourcePath := file.Name()
			_ = SimplifyGrafana(sourcePath, true)
		}
	}
}
