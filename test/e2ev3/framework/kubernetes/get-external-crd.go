package kubernetes

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
)

func downloadExternalCRDs(chartPath string) error {
	crdUrls := []string{
		"https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/main/example/prometheus-operator-crd/monitoring.coreos.com_servicemonitors.yaml",
	}

	for _, crdUrl := range crdUrls {
		crd, err := fetchYAML(crdUrl)
		if err != nil {
			return err
		}

		crdName, err := extractFileName(crdUrl)
		if err != nil {
			return err
		}

		log.Printf("CRD exists %s", crdName)
		log.Printf("File path to be written to %s", filepath.Join(chartPath, "/crds/"+crdName))
		err = saveToFile(filepath.Join(chartPath, "/crds/"+crdName), crd)
		if err != nil {
			return err
		}
	}
	return nil
}

func fetchYAML(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get crd source code from %s: %w", url, err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func extractFileName(rawURL string) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse url: %w", err)
	}
	return path.Base(parsedURL.Path), nil
}

func saveToFile(filename string, data []byte) error {
	err := os.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write crd.yaml to /crds dir : %w", err)
	}
	return nil
}
