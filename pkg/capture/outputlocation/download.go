// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package outputlocation

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/homedir"
	"k8s.io/kubectl/pkg/scheme"

	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/log"
)

type Download struct {
	l *log.ZapLogger
}

// Assert that Download implements Location interface
var _ Location = &Download{}

func NewDownload(logger *log.ZapLogger) Location {
	return &Download{l: logger}
}

func (d *Download) Name() string {
	return "Download"
}

func (d *Download) Enabled() bool {
	// downloadPath := os.Getenv(string(captureConstants.CaptureOutputLocationEnvKeyDownload))
	// if len(downloadPath) == 0 {
	// 	d.l.Debug("Output location is not enabled", zap.String("location", d.Name()))
	// 	return false
	// }
	return true
}

func (d *Download) Output(ctx context.Context, srcFilePath string) error {
	namespace := os.Getenv(string(captureConstants.NamespaceEnvKey))
	pod := os.Getenv(string(captureConstants.PodNameEnvKey))
	container := captureConstants.CaptureContainername
	downloadPath := "/tmp/"

	d.l.Info("Download debugging",
		zap.String("Namespace:", "default"),
		zap.String("Pod:", pod),
		zap.String("Container:", container),
		zap.String("Source file path:", srcFilePath),
		zap.String("Download path:", downloadPath),
	)

	// os.Getenv(string(captureConstants.CaptureOutputLocationEnvKeyDownload)
	// if len(downloadPath) == 0 {
	// 	d.l.Warn("Download path not set, skipping download", zap.String("location", d.Name()))
	// 	return nil
	// }

	kubeConfigFilePath := filepath.Join(homedir.HomeDir(), ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigFilePath)
	// if err != nil {
	// 	return fmt.Errorf("failed to get in-cluster config: %w", err)
	// }

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   []string{"tar", "cf", "-", srcFilePath},
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

	d.l.Info("Request:", zap.String("URL:", req.URL().String()))

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}

	var buf bytes.Buffer
	streamOpts := remotecommand.StreamOptions{
		Stdin:  os.Stdin,
		Stdout: &buf,
		Stderr: &buf,
	}

	if err := exec.StreamWithContext(ctx, streamOpts); err != nil {
		d.l.Error("Exec stream failed", zap.String("stderr", buf.String()), zap.Error(err))
		return fmt.Errorf("failed to exec tar in container: %w", err)
	}

	res := buf.Bytes()
	d.l.Info("Restult", zap.ByteString("res", res))

	// tr := tar.NewReader(&stdout)
	// for {
	// 	hdr, err := tr.Next()
	// 	if err == io.EOF {
	// 		break
	// 	}
	// 	if err != nil {
	// 		return fmt.Errorf("failed to read tar: %w", err)
	// 	}

	// 	localFilePath := filepath.Join(downloadPath, filepath.Base(hdr.Name))
	// 	outFile, err := os.Create(localFilePath)
	// 	if err != nil {
	// 		return fmt.Errorf("failed to create file %s: %w", localFilePath, err)
	// 	}
	// 	if _, err := io.Copy(outFile, tr); err != nil {
	// 		outFile.Close()
	// 		return fmt.Errorf("failed to write file %s: %w", localFilePath, err)
	// 	}
	// 	outFile.Close()

	// 	d.l.Info("Downloaded file",
	// 		zap.String("remote", hdr.Name),
	// 		zap.String("local", localFilePath),
	// 	)
	// }

	return nil
}
