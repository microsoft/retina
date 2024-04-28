//go:build dashboard && !simplifydashboard

package dashboard

import (
	"testing"

	"io/ioutil"
	"path/filepath"

	"reflect"
)

// TestDashboardsAreSimplified ensures that all dashboards are simplified
func TestDashboardsAreSimplified(t *testing.T) {
	// get all json's in this folder
	files, err := ioutil.ReadDir("./")
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".json" {
			t.Logf("verifying that dashboard is simplified: %s", file.Name())

			sourcePath := file.Name()
			simplified := SimplifyGrafana(sourcePath, false)
			original := ParseDashboard(sourcePath)

			if !reflect.DeepEqual(simplified, original) {
				t.Errorf("ERROR: dashboard has not been simplified. Please run: make simplify-dashboards")
			}
		}
	}
}
