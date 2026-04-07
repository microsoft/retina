package kubernetes

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"

	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
)

func downloadExternalCRDs(ctx context.Context, chartPath string) error {
	log := slog.With("prefix", utils.Prefix(ctx))
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

		log.Info("CRD exists", "name", crdName)
		log.Info("writing CRD file", "path", filepath.Join(chartPath, "/crds/"+crdName))
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
