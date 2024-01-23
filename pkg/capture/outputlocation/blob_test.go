// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package outputlocation

import (
	"fmt"
	"net/url"
	"testing"
)

func TestValidateBlobURL(t *testing.T) {
	tests := []struct {
		name          string
		inputURL      string
		expectedError error
	}{
		{
			name:          "valid input URL with sas token",
			inputURL:      "https://retina.blob.core.windows.net/container/blob?sp=r&st=2023-02-17T19:13:30Z&se=2023-02-18T03:13:30Z&spr=https&sv=2021-06-08&sr=c&sig=NtSxlRK5Vs4kVs1dIOfr%2FMdLKBVTA4t3uJ0gqLZ9exk%3D",
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
			err := validateBlobURL(tt.inputURL)

			if err != nil && tt.expectedError == nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if err == nil && tt.expectedError != nil {
				t.Errorf("Expected error: %v, but got none", tt.expectedError)
			}
		})
	}
}
