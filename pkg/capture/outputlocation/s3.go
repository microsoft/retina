// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package outputlocation

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.uber.org/zap"

	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/log"
)

type S3Upload struct {
	l *log.ZapLogger

	region          string
	endpoint        string
	bucket          string
	path            string
	accessKeyID     string
	secretAccessKey string
}

var _ Location = &S3Upload{}

func NewS3Upload(logger *log.ZapLogger) Location {
	return &S3Upload{l: logger}
}

func (su *S3Upload) Name() string {
	return "S3Upload"
}

func (su *S3Upload) Enabled() bool {
	su.bucket = os.Getenv(string(captureConstants.CaptureOutputLocationEnvKeyS3Bucket))
	if su.bucket == "" {
		su.l.Debug("Output location is not enabled because bucket is not set", zap.String("location", su.Name()))
		return false
	}

	su.endpoint = os.Getenv(string(captureConstants.CaptureOutputLocationEnvKeyS3Endpoint))
	su.region = os.Getenv(string(captureConstants.CaptureOutputLocationEnvKeyS3Region))
	if su.endpoint == "" && su.region == "" {
		su.l.Debug("Output location is not enabled because both endpoint and region are not set", zap.String("location", su.Name()))
		return false
	}

	su.path = os.Getenv(string(captureConstants.CaptureOutputLocationEnvKeyS3Path))

	var err error
	su.accessKeyID, err = readAccessKeyId()
	if err != nil {
		su.l.Error("Failed to obtain access key id from secret", zap.Error(err))
		return false
	}

	su.secretAccessKey, err = readSecretAccessKey()
	if err != nil {
		su.l.Error("Failed to obtain secret access key id from secret", zap.Error(err))
		return false
	}

	return true
}

func (su *S3Upload) Output(srcFilePath string) error {
	objectKey := path.Join(su.path, srcFilePath)

	su.l.Info("Upload capture file to s3",
		zap.String("location", su.Name()),
		zap.String("source file path", srcFilePath),
		zap.String("bucketName", su.bucket),
		zap.String("objectKey", objectKey),
	)

	s3Client, err := su.getClient()
	if err != nil {
		su.l.Error("Failed to get AWS client", zap.Error(err))
		return err
	}

	s3File, err := os.Open(srcFilePath)
	if err != nil {
		su.l.Error("Failed to open capture file", zap.Error(err))
		return err
	}
	defer s3File.Close()

	_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(su.bucket),
		Key:    aws.String(objectKey),
		Body:   s3File,
	})
	if err != nil {
		su.l.Error("Couldn't upload file",
			zap.String("srcFilePath", srcFilePath),
			zap.String("bucketName", su.bucket),
			zap.String("objectKey", objectKey),
			zap.Error(err))
	}
	return err
}

func (su *S3Upload) getClient() (*s3.Client, error) {
	var opts []func(options *config.LoadOptions) error

	if su.endpoint != "" {
		opts = append(opts,
			config.WithEndpointResolverWithOptions(
				aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
					return aws.Endpoint{
						URL:               su.endpoint,
						HostnameImmutable: true,
					}, nil
				}),
			),
		)
	}

	if su.region != "" {
		opts = append(opts, config.WithRegion(su.region))
	} else {
		opts = append(opts, config.WithRegion("auto"))
	}

	if su.accessKeyID != "" {
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			su.accessKeyID,
			su.secretAccessKey,
			"",
		)))
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		opts...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %v", err)
	}

	return s3.NewFromConfig(cfg), nil
}

func readAccessKeyId() (string, error) {
	secretPath := filepath.Join(captureConstants.CaptureOutputLocationS3UploadSecretPath, captureConstants.CaptureOutputLocationS3UploadAccessKeyID)
	if runtime.GOOS == "windows" {
		containerSandboxMountPoint := os.Getenv(captureConstants.ContainerSandboxMountPointEnvKey)
		if len(containerSandboxMountPoint) == 0 {
			return "", fmt.Errorf("failed to find sandbox mount path through env %s", captureConstants.ContainerSandboxMountPointEnvKey)
		}
		secretPath = filepath.Join(containerSandboxMountPoint, captureConstants.CaptureOutputLocationS3UploadSecretPath, captureConstants.CaptureOutputLocationS3UploadAccessKeyID)
	}
	secretBytes, err := os.ReadFile(secretPath)
	return string(secretBytes), err
}

func readSecretAccessKey() (string, error) {
	secretPath := filepath.Join(captureConstants.CaptureOutputLocationS3UploadSecretPath, captureConstants.CaptureOutputLocationS3UploadSecretAccessKey)
	if runtime.GOOS == "windows" {
		containerSandboxMountPoint := os.Getenv(captureConstants.ContainerSandboxMountPointEnvKey)
		if len(containerSandboxMountPoint) == 0 {
			return "", fmt.Errorf("failed to find sandbox mount path through env %s", captureConstants.ContainerSandboxMountPointEnvKey)
		}
		secretPath = filepath.Join(containerSandboxMountPoint, captureConstants.CaptureOutputLocationS3UploadSecretPath, captureConstants.CaptureOutputLocationS3UploadSecretAccessKey)
	}
	secretBytes, err := os.ReadFile(secretPath)
	return string(secretBytes), err
}
