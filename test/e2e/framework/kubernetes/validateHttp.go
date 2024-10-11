package kubernetes

import (
	"context"
	"fmt"
	"log"
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

func (v *ValidateHTTPResponse) Run() error {
	ctx, cancel := context.WithTimeout(context.Background(), RequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.URL, http.NoBody)
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
	log.Printf("HTTP validation succeeded for URL: %s with status code %d\n", v.URL, resp.StatusCode)

	return nil
}

func (v *ValidateHTTPResponse) Prevalidate() error {
	return nil
}

func (v *ValidateHTTPResponse) Stop() error {
	return nil
}
