// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package outputlocation

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/log"
)

const fakeSASQuery = "sv=2022-11-02&sig=SECRETSIGNATURE"

func TestValidateBlobUploadURL(t *testing.T) {
	cases := []struct {
		name    string
		url     string
		wantErr error
	}{
		{name: "https with SAS token", url: "https://acct.blob.core.windows.net/container?" + fakeSASQuery},
		{name: "https with blob path", url: "https://acct.blob.core.windows.net/container/blob?" + fakeSASQuery},
		{name: "quoted with trailing newline, as read from a secret file", url: "\"https://acct.blob.core.windows.net/container?" + fakeSASQuery + "\"\n"},
		{name: "uppercase scheme", url: "HTTPS://acct.blob.core.windows.net/container?" + fakeSASQuery},
		{name: "plain http", url: "http://acct.blob.core.windows.net/container?" + fakeSASQuery, wantErr: ErrInsecureUploadDestination},
		{name: "http to an arbitrary host", url: "http://10.0.0.5:8080/upload?" + fakeSASQuery, wantErr: ErrInsecureUploadDestination},
		{name: "other scheme", url: "ftp://acct.blob.core.windows.net/container", wantErr: ErrInsecureUploadDestination},
		{name: "no scheme", url: "acct.blob.core.windows.net/container?" + fakeSASQuery, wantErr: ErrInsecureUploadDestination},
		{name: "empty", url: "", wantErr: ErrInvalidUploadDestination},
		{name: "whitespace only", url: " \n", wantErr: ErrInvalidUploadDestination},
		{name: "https without host", url: "https:///container?" + fakeSASQuery, wantErr: ErrInvalidUploadDestination},
		{name: "https without container", url: "https://acct.blob.core.windows.net?" + fakeSASQuery, wantErr: ErrInvalidUploadDestination},
		{name: "https with root path only", url: "https://acct.blob.core.windows.net/?" + fakeSASQuery, wantErr: ErrInvalidUploadDestination},
		{name: "unparsable", url: "https://acct.blob.core.windows.net/%zz?" + fakeSASQuery, wantErr: ErrInvalidUploadDestination},
	}

	for i := range cases {
		tt := &cases[i]
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBlobUploadURL(tt.url)
			if tt.wantErr == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tt.wantErr)
			// The SAS token is a credential; it must never end up in an error (and so in a Capture status or log).
			require.NotContains(t, err.Error(), "SECRETSIGNATURE")
		})
	}
}

func TestValidateS3Endpoint(t *testing.T) {
	cases := []struct {
		name     string
		endpoint string
		wantErr  error
		wantMsg  string
	}{
		{name: "empty uses the AWS https endpoint for the region", endpoint: ""},
		{name: "https", endpoint: "https://s3.example.com"},
		{name: "https with port", endpoint: "https://play.min.io:9000"},
		{name: "surrounding whitespace is ignored", endpoint: "  https://s3.example.com\n"},
		{name: "plain http", endpoint: "http://minio.minio.svc:9000", wantErr: ErrInsecureUploadDestination},
		{name: "no scheme", endpoint: "s3.example.com", wantErr: ErrInsecureUploadDestination},
		{name: "https without host", endpoint: "https://", wantErr: ErrInvalidUploadDestination},
		{name: "double-quoted", endpoint: `"https://s3.example.com"`, wantErr: ErrInvalidUploadDestination, wantMsg: "without quotes"},
		{name: "single-quoted", endpoint: `'https://s3.example.com'`, wantErr: ErrInvalidUploadDestination, wantMsg: "without quotes"},
		{name: "unparsable", endpoint: "https://s3.example.com/%zz", wantErr: ErrInvalidUploadDestination},
	}

	for i := range cases {
		tt := &cases[i]
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateS3Endpoint(tt.endpoint)
			if tt.wantErr == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tt.wantErr)
			if tt.wantMsg != "" {
				require.ErrorContains(t, err, tt.wantMsg)
			}
		})
	}
}

// The capture workload re-checks the destination right before uploading (BlobUpload.Output calls
// validateBlobSASURL on the URL read from the mounted secret), which covers captures created by any path.
func TestBlobUploadValidationRejectsInsecureURL(t *testing.T) {
	require.ErrorIs(t, validateBlobSASURL("http://10.0.0.5:8080/upload?"+fakeSASQuery), ErrInsecureUploadDestination)
	require.NoError(t, validateBlobSASURL("https://acct.blob.core.windows.net/container?"+fakeSASQuery))
}

func TestS3UploadOutputRejectsInsecureEndpoint(t *testing.T) {
	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	require.NoError(t, err)
	// A real file, so the test doesn't depend on the endpoint being checked before the file is opened.
	srcFile := filepath.Join(t.TempDir(), "capture.tar.gz")
	require.NoError(t, os.WriteFile(srcFile, nil, 0o600))

	su := &S3Upload{l: log.Logger().Named("test"), endpoint: "http://minio.minio.svc:9000", bucket: "captures"}
	err = su.Output(context.Background(), srcFile)
	require.ErrorIs(t, err, ErrInsecureUploadDestination)
	require.ErrorContains(t, err, "refusing to upload to S3, invalid endpoint")
	require.NotContains(t, err.Error(), "minio.minio.svc", "the endpoint must not be echoed in the error")
}

// The workload uses the endpoint exactly as validated: surrounding whitespace from the env var is trimmed on read.
// Enabled stores the endpoint before reading the S3 credential files, so this holds even though Enabled returns
// false here (the secret files don't exist in tests).
func TestS3UploadEnabledTrimsEndpoint(t *testing.T) {
	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	require.NoError(t, err)
	t.Setenv(string(captureConstants.CaptureOutputLocationEnvKeyS3Bucket), "captures")
	t.Setenv(string(captureConstants.CaptureOutputLocationEnvKeyS3Endpoint), "  https://s3.example.com\n")

	su := &S3Upload{l: log.Logger().Named("test")}
	_ = su.Enabled()
	require.Equal(t, "https://s3.example.com", su.endpoint)
}
