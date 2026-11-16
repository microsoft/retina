// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package outputlocation

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

var (
	// ErrInsecureUploadDestination is returned when a remote upload destination doesn't use https. Capture files
	// contain full packet payloads, so they must not be sent in plaintext.
	ErrInsecureUploadDestination = errors.New("upload destination must use https")
	// ErrInvalidUploadDestination is returned when a remote upload destination isn't a usable URL.
	ErrInvalidUploadDestination = errors.New("invalid upload destination")
)

// ValidateBlobUploadURL checks a blob SAS URL (as stored in the blob upload secret, so surrounding quotes and
// whitespace are trimmed) before capture files are uploaded to it. Errors never include the URL itself, which
// carries the SAS token.
func ValidateBlobUploadURL(blobSASURL string) error {
	trimmed := trimBlobSASURL(blobSASURL)
	if trimmed == "" {
		return fmt.Errorf("%w: blob upload URL is empty", ErrInvalidUploadDestination)
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("%w: blob upload URL is not a valid URL", ErrInvalidUploadDestination)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("%w: blob upload URL uses scheme %q", ErrInsecureUploadDestination, u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("%w: blob upload URL has no host", ErrInvalidUploadDestination)
	}
	if strings.TrimPrefix(u.Path, "/") == "" {
		return fmt.Errorf("%w: blob upload URL has no container", ErrInvalidUploadDestination)
	}
	return nil
}

// ValidateS3Endpoint checks a custom S3 endpoint. An empty endpoint is valid: the AWS SDK then derives an https
// endpoint from the region. Surrounding whitespace is ignored (the workload trims it too), but quotes are not
// stripped: unlike blob URLs, which are read from secret files, the endpoint comes from the Capture spec or a CLI
// flag, so quotes are a typo.
func ValidateS3Endpoint(endpoint string) error {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil
	}
	if strings.ContainsAny(endpoint, `"'`) {
		return fmt.Errorf("%w: S3 endpoint must be a plain URL, without quotes", ErrInvalidUploadDestination)
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("%w: S3 endpoint is not a valid URL", ErrInvalidUploadDestination)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("%w: S3 endpoint uses scheme %q", ErrInsecureUploadDestination, u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("%w: S3 endpoint has no host", ErrInvalidUploadDestination)
	}
	return nil
}
