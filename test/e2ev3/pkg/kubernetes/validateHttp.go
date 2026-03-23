package kubernetes

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

const (
	RequestTimeout = 30 * time.Second
)

type ValidateHTTPResponse struct {
	URL            string
	ExpectedStatus int
}

func (v *ValidateHTTPResponse) String() string { return "validate-http-response" }

func (v *ValidateHTTPResponse) Do(ctx context.Context) error {
	log := slog.With("step", v.String())
	ctx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.URL, nil)
	if err != nil {
		return fmt.Errorf("error creating HTTP request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error making HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != v.ExpectedStatus {
		return fmt.Errorf("unexpected status code: got %d, want %d", resp.StatusCode, v.ExpectedStatus)
	}
	log.Info("HTTP validation succeeded", "url", v.URL, "statusCode", resp.StatusCode)

	return nil
}
