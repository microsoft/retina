// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"

	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/capture/file"
	"github.com/microsoft/retina/pkg/capture/outputlocation"
)

// Insecure or malformed upload destinations are rejected when the Capture is translated, so no capture Job runs
// (the operator records the error in the Capture status; the CLI fails the command).
func TestTranslateCaptureToJobsValidatesUploadDestinations(t *testing.T) {
	ctx, cancel := TestContext(t)
	defer cancel()

	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name:   "node1",
		Labels: map[string]string{corev1.LabelHostname: "node1", corev1.LabelOSStable: "linux"},
	}}
	blobSecret := func(url string) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "blob-sas", Namespace: "tenant-a"},
			Data:       map[string][]byte{captureConstants.CaptureOutputLocationBlobUploadSecretKey: []byte(url)},
		}
	}
	s3Secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "s3-creds", Namespace: "tenant-a"}}
	blobOutput := retinav1alpha1.OutputConfiguration{BlobUpload: ptr.To("blob-sas")}
	s3Output := func(endpoint string) retinav1alpha1.OutputConfiguration {
		return retinav1alpha1.OutputConfiguration{S3Upload: &retinav1alpha1.S3Upload{
			Endpoint: endpoint, Bucket: "captures", SecretName: "s3-creds", Region: "us-east-1",
		}}
	}

	cases := []struct {
		name    string
		secret  *corev1.Secret
		output  retinav1alpha1.OutputConfiguration
		wantErr error
		// wantErrMsg, when set, must appear in the error.
		wantErrMsg string
	}{
		{
			name:   "blob secret with https URL",
			secret: blobSecret("https://acct.blob.core.windows.net/container?sig=SECRETSIGNATURE"),
			output: blobOutput,
		},
		{
			name:   "blob secret as written by kubectl --from-file (quoted, trailing newline)",
			secret: blobSecret("\"https://acct.blob.core.windows.net/container?sig=SECRETSIGNATURE\"\n"),
			output: blobOutput,
		},
		{
			name:    "blob secret with plain-http URL to an arbitrary host",
			secret:  blobSecret("http://10.0.0.5:8080/upload?sig=SECRETSIGNATURE"),
			output:  blobOutput,
			wantErr: outputlocation.ErrInsecureUploadDestination,
		},
		{
			name: "blob secret without the URL key",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "blob-sas", Namespace: "tenant-a"},
				Data:       map[string][]byte{"other-key": []byte("https://acct.blob.core.windows.net/container")},
			},
			output:     blobOutput,
			wantErr:    outputlocation.ErrInvalidUploadDestination,
			wantErrMsg: `blob upload secret tenant-a/blob-sas: invalid upload destination: missing key "blob-upload-url"`,
		},
		{
			name: "blob secret with the URL key present but empty",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "blob-sas", Namespace: "tenant-a"},
				Data:       map[string][]byte{captureConstants.CaptureOutputLocationBlobUploadSecretKey: {}},
			},
			output:     blobOutput,
			wantErr:    outputlocation.ErrInvalidUploadDestination,
			wantErrMsg: "blob upload URL is empty",
		},
		{name: "S3 with region only", secret: s3Secret, output: s3Output("")},
		{name: "S3 with https endpoint", secret: s3Secret, output: s3Output("https://s3.example.com")},
		{
			name:    "S3 with plain-http endpoint",
			secret:  s3Secret,
			output:  s3Output("http://minio.minio.svc:9000"),
			wantErr: outputlocation.ErrInsecureUploadDestination,
		},
		{
			name:       "S3 with quoted endpoint",
			secret:     s3Secret,
			output:     s3Output(`"https://s3.example.com"`),
			wantErr:    outputlocation.ErrInvalidUploadDestination,
			wantErrMsg: "invalid OutputConfiguration.S3Upload.Endpoint: invalid upload destination: S3 endpoint must be a plain URL, without quotes",
		},
	}

	for i := range cases {
		tt := &cases[i]
		t.Run(tt.name, func(t *testing.T) {
			objects := []runtime.Object{node, tt.secret}
			translator := NewCaptureToPodTranslatorForTest(fakeclientset.NewSimpleClientset(objects...))
			jobs, err := translator.TranslateCaptureToJobs(ctx, &retinav1alpha1.Capture{
				ObjectMeta: metav1.ObjectMeta{Name: "c", Namespace: "tenant-a"},
				Status:     retinav1alpha1.CaptureStatus{StartTime: file.Now()},
				Spec: retinav1alpha1.CaptureSpec{
					CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
						CaptureTarget: retinav1alpha1.CaptureTarget{
							NodeSelector: &metav1.LabelSelector{MatchLabels: map[string]string{corev1.LabelHostname: "node1"}},
						},
						CaptureOption: retinav1alpha1.CaptureOption{Duration: &metav1.Duration{Duration: time.Minute}},
					},
					OutputConfiguration: tt.output,
				},
			})
			if tt.wantErr == nil {
				require.NoError(t, err)
				require.Len(t, jobs, 1)
				return
			}
			require.ErrorIs(t, err, tt.wantErr)
			if tt.wantErrMsg != "" {
				require.ErrorContains(t, err, tt.wantErrMsg)
			}
			require.NotContains(t, err.Error(), "SECRETSIGNATURE")
			require.Empty(t, jobs)
		})
	}
}
