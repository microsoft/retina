// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package outputlocation

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/microsoft/retina/pkg/log"
)

func TestTrimBlobSASURL(t *testing.T) {
	tests := []struct {
		name               string
		inputURL           string
		expectedTrimmedURL string
	}{
		{
			name:               "valid input URL with sas token that have a newline and is surrounded by double quotes",
			inputURL:           "\"https://retina.blob.core.windows.net/container/blob?sas-token\"\n",
			expectedTrimmedURL: "https://retina.blob.core.windows.net/container/blob?sas-token",
		},
		{
			name:               "valid input URL with sas token that have a newline and is surrounded by double quotes and extra spaces",
			inputURL:           "\"https://retina.blob.core.windows.net/container/blob?sas-token  \"\n",
			expectedTrimmedURL: "https://retina.blob.core.windows.net/container/blob?sas-token",
		},
		{
			name:               "valid input URL with sas token that has extra spaces",
			inputURL:           "https://retina.blob.core.windows.net/container/blob?sas-token  ",
			expectedTrimmedURL: "https://retina.blob.core.windows.net/container/blob?sas-token",
		},
		{
			name:               "valid input URL with sas token",
			inputURL:           "\"https://retina.blob.core.windows.net/container/blob?sas-token\"\n",
			expectedTrimmedURL: "https://retina.blob.core.windows.net/container/blob?sas-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualTrimmedBlobSASURL := trimBlobSASURL(tt.inputURL)
			if actualTrimmedBlobSASURL != tt.expectedTrimmedURL {
				t.Errorf("Expected trimmed Blob SAS URL %s, but got %s", tt.expectedTrimmedURL, actualTrimmedBlobSASURL)
			}
		})
	}
}

func TestValidateBlobSASURL(t *testing.T) {
	tests := []struct {
		name          string
		inputURL      string
		expectedError error
	}{
		{
			name:          "valid input URL with sas token",
			inputURL:      "https://retina.blob.core.windows.net/container/blob?sas-token",
			expectedError: nil,
		},
		{
			name:          "valid input URL without sas token",
			inputURL:      "https://retina.blob.core.windows.net/container/blob",
			expectedError: nil,
		},
		{
			name:          "missing container name",
			inputURL:      "https://retina.blob.core.windows.net",
			expectedError: fmt.Errorf("invalid blob url"),
		},
		{
			name:          "missing blob name",
			inputURL:      "https://retina.blob.core.windows.net/container",
			expectedError: nil,
		},
		{
			name:          "empty input URL",
			inputURL:      "",
			expectedError: &url.Error{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBlobSASURL(tt.inputURL)

			if err != nil && tt.expectedError == nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if err == nil && tt.expectedError != nil {
				t.Errorf("Expected error: %v, but got none", tt.expectedError)
			}
		})
	}
}

var errNoSuchHost = errors.New("dial tcp: no such host")

// failingTransport fails every request the way http.Client does when no response is received (DNS, connection, TLS,
// timeout): with a *url.Error that carries the full request URL, SAS token included.
type failingTransport struct{}

func (failingTransport) Do(req *http.Request) (*http.Response, error) {
	return nil, &url.Error{Op: req.Method, URL: req.URL.String(), Err: errNoSuchHost}
}

// serviceErrorTransport answers every request with a storage service error response.
type serviceErrorTransport struct {
	status int
	code   string
	body   string
}

func (s serviceErrorTransport) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: s.status,
		Status:     fmt.Sprintf("%d %s", s.status, http.StatusText(s.status)),
		Header: http.Header{
			"Content-Type":    []string{"application/xml"},
			"X-Ms-Error-Code": []string{s.code},
		},
		Body:    io.NopCloser(strings.NewReader(s.body)),
		Request: req,
	}, nil
}

// The SAS token in the blob URL is a credential: failed uploads must not expose it in the returned error or in the
// capture pod's logs.
func TestBlobUploadErrorsDoNotContainSASToken(t *testing.T) {
	srcFile := filepath.Join(t.TempDir(), "capture.tar.gz")
	require.NoError(t, os.WriteFile(srcFile, []byte("capture"), 0o600))

	const sasURL = "https://retinatest.invalid/captures?sv=2022-11-02&sig=SECRETSIGNATURE"

	cases := []struct {
		name      string
		blobURL   string
		transport policy.Transporter
		wantMsgs  []string
	}{
		{
			// No response from a server (e.g. the host doesn't resolve): the error contains the request URL.
			name:      "upload fails before a response",
			blobURL:   sasURL,
			transport: failingTransport{},
			wantMsgs:  []string{`"https://retinatest.invalid/captures/capture.tar.gz?REDACTED"`},
		},
		{
			// The message depends on the URL validation in use; only the absence of the token is checked.
			name:      "URL fails to parse",
			blobURL:   "https://retinatest.invalid/captures/%zz?sv=2022-11-02&sig=SECRETSIGNATURE",
			transport: failingTransport{},
		},
		{
			name:    "upload is rejected",
			blobURL: sasURL,
			transport: serviceErrorTransport{
				status: http.StatusForbidden,
				code:   "AuthenticationFailed",
				body:   `<Error><Code>AuthenticationFailed</Code><Message>denied</Message></Error>`,
			},
			wantMsgs: []string{"403", "AuthenticationFailed"},
		},
		{
			// The service echoes the rejected query value, and azblob prints the raw response body.
			name:    "upload is rejected for a query value",
			blobURL: sasURL,
			transport: serviceErrorTransport{
				status: http.StatusBadRequest,
				code:   "InvalidQueryParameterValue",
				body: `<Error><Code>InvalidQueryParameterValue</Code><Message>invalid</Message>` +
					`<QueryParameterName>sig</QueryParameterName><QueryParameterValue>SECRETSIGNATURE</QueryParameterValue></Error>`,
			},
			wantMsgs: []string{"400", "InvalidQueryParameterValue", "<QueryParameterValue>REDACTED</QueryParameterValue>"},
		},
	}

	for i := range cases {
		tt := &cases[i]
		t.Run(tt.name, func(t *testing.T) {
			core, logs := observer.New(zap.DebugLevel)
			bu := &BlobUpload{
				l: &log.ZapLogger{Logger: zap.New(core)},
				clientOptions: &azblob.ClientOptions{ClientOptions: azcore.ClientOptions{
					Transport: tt.transport,
					// One attempt: the SDK's default retries would make the test take ~20s.
					Retry: policy.RetryOptions{MaxRetries: -1},
				}},
			}

			err := bu.upload(context.Background(), tt.blobURL, srcFile)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "SECRETSIGNATURE")
			for _, msg := range tt.wantMsgs {
				require.Contains(t, err.Error(), msg)
			}

			require.NotZero(t, logs.Len(), "the failure must still be logged")
			for _, entry := range logs.All() {
				require.NotContains(t, fmt.Sprint(entry.Message, entry.ContextMap()), "SECRETSIGNATURE")
			}
		})
	}
}
