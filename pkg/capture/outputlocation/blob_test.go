// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package outputlocation

import (
	"fmt"
	"net/url"
	"testing"
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
