// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package outputlocation

import (
	"context"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/microsoft/retina/pkg/log"
)

// A failed upload must be returned to the caller instead of being reported as a successful upload.
func TestS3UploadOutputReturnsUploadError(t *testing.T) {
	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	require.NoError(t, err)

	var requests atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>AccessDenied</Code><Message>denied</Message></Error>`))
	}))
	t.Cleanup(srv.Close)

	// Make the AWS SDK trust the test server's certificate and ignore any local AWS configuration.
	dir := t.TempDir()
	caFile := filepath.Join(dir, "ca.pem")
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	require.NoError(t, os.WriteFile(caFile, caPEM, 0o600))
	t.Setenv("AWS_CA_BUNDLE", caFile)
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(dir, "missing-config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, "missing-credentials"))
	t.Setenv("AWS_PROFILE", "")

	srcFile := filepath.Join(dir, "capture.tar.gz")
	require.NoError(t, os.WriteFile(srcFile, []byte("capture"), 0o600))

	su := &S3Upload{
		l:               log.Logger().Named("test"),
		endpoint:        srv.URL,
		region:          "us-east-1",
		bucket:          "captures",
		accessKeyID:     "test-access-key",
		secretAccessKey: "test-secret-key",
	}
	err = su.Output(context.Background(), srcFile)
	require.ErrorContains(t, err, "failed to upload file to S3")
	require.Positive(t, requests.Load(), "the upload must reach the server, so the error comes from PutObject")
}
