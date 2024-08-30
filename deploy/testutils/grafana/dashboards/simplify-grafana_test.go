//go:build dashboard && !simplifydashboard

package dashboard

import (
	"path/filepath"
	"reflect"
	"testing"
)

// TestDashboardsAreSimplified ensures that all dashboards are simplified
func TestDashboardsAreSimplified(t *testing.T) {
	// get all json's in this folder
	files, err := filepath.Glob("../../../*/grafana/dashboards/*.json")

	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		t.Logf("verifying that dashboard is simplified: %s", file)

		sourcePath := file
		simplified := SimplifyGrafana(sourcePath, false)
		original := ParseDashboard(sourcePath)

		if !reflect.DeepEqual(simplified, original) {
			t.Errorf("ERROR: dashboard has not been simplified. Please run: make simplify-dashboards")
		}
	}
}
