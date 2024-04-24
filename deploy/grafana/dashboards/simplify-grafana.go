//go:build dashboard

package dashboard

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
)

// SimplifyGrafana parses a json file representing a Grafana dashboard and overwrites it with a simplified version.
// It removes some unnecessary fields and changes some values so that the dashboard can be used by anyone.
func SimplifyGrafana(filename string, overwrite bool) map[string]interface{} {
	dashboard := ParseDashboard(filename)

	// remove unnecessary top-level fields
	delete(dashboard, "weekStart")
	delete(dashboard, "fiscalYearStartMonth")
	delete(dashboard, "graphTooltip")
	delete(dashboard, "annotations")

	// set empty __inputs
	if _, ok := dashboard["__inputs"]; ok {
		dashboard["__inputs"] = []interface{}{}
	}

	// set current variables to be empty
	if _, ok := dashboard["templating"]; ok {
		templating := dashboard["templating"].(map[string]interface{})
		if _, ok := templating["list"]; ok {
			list := templating["list"].([]interface{})
			for _, item := range list {
				if item, ok := item.(map[string]interface{}); ok {
					item["current"] = map[string]interface{}{}
				}
			}
		}
	}

	// remove any pluginVersion
	removeFieldAnywhere(dashboard, "pluginVersion")

	// change all datasource.uid to "${datasource}"
	replaceDatasource(dashboard)

	if !overwrite {
		return dashboard
	}

	// overwrite the file with the simplified version
	simplifiedData, err := JSONMarhsalIndent(dashboard, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	err = os.WriteFile(filename, simplifiedData, 0644)
	if err != nil {
		log.Fatal(err)
	}

	return dashboard
}

func ParseDashboard(filename string) map[string]interface{} {
	data, err := os.ReadFile(filename)
	if err != nil {
		log.Fatal(err)
	}

	var dashboard map[string]interface{}
	err = json.Unmarshal(data, &dashboard)
	if err != nil {
		log.Fatal(err)
	}

	return dashboard
}

func removeFieldAnywhere(data map[string]interface{}, field string) {
	if _, ok := data[field]; ok {
		delete(data, field)
	}

	for _, value := range data {
		if m, ok := value.(map[string]interface{}); ok {
			removeFieldAnywhere(m, field)
		} else if l, ok := value.([]interface{}); ok {
			for _, item := range l {
				if m, ok := item.(map[string]interface{}); ok {
					removeFieldAnywhere(m, field)
				}
			}
		}
	}
}

func replaceDatasource(data map[string]interface{}) {
	if datasource, ok := data["datasource"].(map[string]interface{}); ok {
		if _, ok := datasource["uid"].(string); ok {
			datasource["uid"] = "${datasource}"
		}
	}

	for _, value := range data {
		if m, ok := value.(map[string]interface{}); ok {
			replaceDatasource(m)
		} else if l, ok := value.([]interface{}); ok {
			for _, item := range l {
				if m, ok := item.(map[string]interface{}); ok {
					replaceDatasource(m)
				}
			}
		}
	}
}

// JSONMarhsalIndent is json.MarshalIndent without HTML escaping (e.g. converting > to \u003e)
func JSONMarhsalIndent(t interface{}, prefix, indent string) ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent(prefix, indent)
	err := encoder.Encode(t)
	return buffer.Bytes(), err
}
