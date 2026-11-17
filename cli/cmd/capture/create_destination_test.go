// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/microsoft/retina/pkg/capture/outputlocation"
)

// Insecure upload destinations fail the command before any secret (holding the SAS URL or S3 keys) or Job is
// created, so nothing is left behind in the cluster.
func TestCreateRejectsInsecureUploadDestination(t *testing.T) {
	base := []string{"create", "--name", "dest", "--namespace", "tenant-a", "--node-selectors", "kubernetes.io/hostname=node1", "--no-wait"}

	cases := []struct {
		name    string
		args    []string
		wantErr error
		// wantErrMsg, when set, must appear in the error.
		wantErrMsg string
	}{
		{
			name:    "plain-http blob upload URL",
			args:    []string{"--blob-upload", "http://10.0.0.5:8080/upload?sig=SECRETSIGNATURE"},
			wantErr: outputlocation.ErrInsecureUploadDestination,
		},
		{
			name:    "blob upload URL without container",
			args:    []string{"--blob-upload", "https://acct.blob.core.windows.net?sig=SECRETSIGNATURE"},
			wantErr: outputlocation.ErrInvalidUploadDestination,
		},
		{
			name:    "plain-http S3 endpoint",
			args:    []string{"--s3-bucket", "captures", "--s3-endpoint", "http://minio.minio.svc:9000", "--s3-access-key-id", "id", "--s3-secret-access-key", "key"},
			wantErr: outputlocation.ErrInsecureUploadDestination,
		},
		{
			name:       "quoted S3 endpoint",
			args:       []string{"--s3-bucket", "captures", "--s3-endpoint", `"https://s3.example.com"`, "--s3-access-key-id", "id", "--s3-secret-access-key", "key"},
			wantErr:    outputlocation.ErrInvalidUploadDestination,
			wantErrMsg: "--s3-endpoint: invalid upload destination: S3 endpoint must be a plain URL, without quotes",
		},
	}

	for i := range cases {
		tt := &cases[i]
		t.Run(tt.name, func(t *testing.T) {
			kubeClient := fake.NewClientset(NewNode("node1"), NewNamespace("tenant-a"))
			cmd := NewCommand(kubeClient)
			cmd.SetArgs(append(append([]string{}, base...), tt.args...))
			cmd.SetOut(new(bytes.Buffer))
			cmd.SetErr(new(bytes.Buffer))

			err := cmd.Execute()
			require.ErrorIs(t, err, tt.wantErr)
			if tt.wantErrMsg != "" {
				require.ErrorContains(t, err, tt.wantErrMsg)
			}
			require.NotContains(t, err.Error(), "SECRETSIGNATURE")

			secrets, listErr := kubeClient.CoreV1().Secrets("tenant-a").List(context.Background(), metav1.ListOptions{})
			require.NoError(t, listErr)
			require.Empty(t, secrets.Items, "no secret may be created for a rejected destination")
			jobs, listErr := kubeClient.BatchV1().Jobs("tenant-a").List(context.Background(), metav1.ListOptions{})
			require.NoError(t, listErr)
			require.Empty(t, jobs.Items, "no capture Job may be created for a rejected destination")
		})
	}
}

func TestCreateAcceptsHTTPSBlobUploadURL(t *testing.T) {
	kubeClient := fake.NewClientset(NewNode("node1"), NewNamespace("tenant-a"))
	cmd := NewCommand(kubeClient)
	cmd.SetArgs([]string{
		"create", "--name", "dest", "--namespace", "tenant-a", "--node-selectors", "kubernetes.io/hostname=node1", "--no-wait",
		"--blob-upload", "https://acct.blob.core.windows.net/container?sig=x",
	})
	cmd.SetOut(new(bytes.Buffer))

	require.NoError(t, cmd.Execute())
	jobs, err := kubeClient.BatchV1().Jobs("tenant-a").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, jobs.Items, 1)
}
