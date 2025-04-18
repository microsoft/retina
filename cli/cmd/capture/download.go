// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	retinacmd "github.com/microsoft/retina/cli/cmd"
	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/capture/file"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
)

const BlobURL = "BLOB_URL"
const DownloadPath = "/tmp/retina/capture/"
const FileExtension = ".tar.gz"
const MountPath = "/mnt/retina/"

var ErrEmptyBlobURL = errors.Errorf("%s environment variable is empty. It must be set/exported", BlobURL)
var jobName string

var downloadCapture = &cobra.Command{
	Use:   "download",
	Short: "Download Retina Captures",
	RunE: func(*cobra.Command, []string) error {
		viper.AutomaticEnv()

		kubeConfig, err := opts.ToRESTConfig()
		if err != nil {
			return errors.Wrap(err, "failed to compose k8s rest config")
		}

		// Create a context that is canceled when a termination signal is received
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM)
		defer cancel()

		captureNamespace := *opts.Namespace
		if allNamespaces {
			captureNamespace = ""
		}

		retinacmd.Logger.Info(fmt.Sprintf("Capture Namespace: %s", captureNamespace))

		return downloadFromCluster(ctx, kubeConfig, "default")
	},
}

func downloadFromCluster(ctx context.Context, config *rest.Config, namespace string) error {
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return errors.Wrap(err, "failed to initialize k8s client")
	}

	// todo: default behaviour when no job is passed - print jobs and make user select one

	// Get Pod where job ran
	pods, err := kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil {
		return err
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no pod found for job %s", jobName)
	}
	pod := pods.Items[0]

	// Get capture container -> we need env variables from here to re-create the file name
	// Maybe in future the file name could be a Pod level Label / Annotation so we don't need to dig into Container Env Vars

	containerName := captureConstants.CaptureContainername
	var targetContainer *corev1.Container
	for i, c := range pod.Spec.Containers {
		if c.Name == containerName {
			targetContainer = &pod.Spec.Containers[i]
			break
		}
	}
	if targetContainer == nil {
		return fmt.Errorf("container %s not found in pod %s", containerName, pod.Name)
	}
	envVars := map[string]string{}
	for _, env := range targetContainer.Env {
		envVars[env.Name] = env.Value
	}

	// Get env vars from container where capture ran - to re-create the file name
	hostPath := envVars[string(captureConstants.CaptureOutputLocationEnvKeyHostPath)]
	captureName := envVars[string(captureConstants.CaptureNameEnvKey)]
	nodeHostName := envVars[string(captureConstants.NodeHostNameEnvKey)]
	captureStart := envVars[string(captureConstants.CaptureStartTimestampEnvKey)]

	retinacmd.Logger.Info(fmt.Sprintf("Host path %s", hostPath))

	timestamp, err := file.StringToTimestamp(captureStart)
	if err != nil {
		return err
	}
	captureFile := file.CaptureFilename{
		CaptureName:    captureName,
		NodeHostname:   nodeHostName,
		StartTimestamp: timestamp,
	}
	fileName := captureFile.String() + FileExtension

	srcFilePath := MountPath + fileName
	retinacmd.Logger.Info(fmt.Sprintf("File src path:  %s", srcFilePath))

	downloadPod, err := createDownloadPod(ctx, kubeClient, namespace, nodeHostName, hostPath, jobName)
	if err != nil {
		return err
	}

	retinacmd.Logger.Info(fmt.Sprintf("Download debugging: Namespace: default / Pod: %s / Container: %s / SrcFile: %s / Download Path: %s / Download Pod: %s", pod.Name, containerName, srcFilePath, DownloadPath, downloadPod.Name))

	req := kubeClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(downloadPod.Name).
		Namespace("default").
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: "download",
			Command:   []string{"tar", "cf", "-", "-C", filepath.Dir(srcFilePath), filepath.Base(srcFilePath)},
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

	retinacmd.Logger.Info(fmt.Sprintf("Request: %s", req.URL().String()))

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}

	var buf bytes.Buffer
	streamOpts := remotecommand.StreamOptions{
		Stdout: &buf,
		Stderr: &buf,
	}

	if err := exec.StreamWithContext(ctx, streamOpts); err != nil {
		retinacmd.Logger.Error(fmt.Sprintf("exec stream failed: stderr: %s / error: %w", buf.String(), err))
		return fmt.Errorf("failed to exec tar in container: %w", err)
	}

	outputFile := filepath.Join(DownloadPath, fileName)

	err = os.MkdirAll(DownloadPath, 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	err = os.WriteFile(outputFile, buf.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file to host: %w", err)
	}

	retinacmd.Logger.Info(fmt.Sprintf("File written to: %s", outputFile))

	// err = kubeClient.CoreV1().Pods(namespace).Delete(ctx, downloadPod.Name, metav1.DeleteOptions{})
	// if err != nil {
	// 	retinacmd.Logger.Warn("Failed to clean up debug pod", zap.String("name", downloadPod.Name), zap.Error(err))
	// }

	return nil
}

func createDownloadPod(ctx context.Context, kubeClient *kubernetes.Clientset, namespace, nodeName, hostPath, jobName string) (*corev1.Pod, error) {
	podName := jobName + "-download-" + rand.String(5)

	podSpec := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{
				{
					Name:    "download",
					Image:   "busybox",
					Command: []string{"sleep", "3600"},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "host-mount",
							MountPath: MountPath, // inside container
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
			Volumes: []corev1.Volume{
				{
					Name: "host-mount",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: hostPath, // on the node
						},
					},
				},
			},
		},
	}

	// Create the pod
	_, err := kubeClient.CoreV1().Pods(namespace).Create(ctx, podSpec, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create debug pod: %w", err)
	}

	// Wait until pod is running
	for {
		time.Sleep(1 * time.Second)
		retinacmd.Logger.Info("Waiting for Download Pod to spin up...")
		pod, err := kubeClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if pod.Status.Phase == corev1.PodRunning {
			return pod, nil
		}
		if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
			return nil, fmt.Errorf("debug pod ended before becoming ready")
		}
	}
}

// func downloadFromBlob() error {
// // BLOB_URL
// blobURL := viper.GetString(BlobURL)
// if blobURL == "" {
// 	return ErrEmptyBlobURL
// }

// u, err := url.Parse(blobURL)
// if err != nil {
// 	return errors.Wrapf(err, "failed to parse SAS URL %s", blobURL)
// }

// // blobService, err := storage.NewAccountSASClientFromEndpointToken(u.String(), u.Query().Encode()).GetBlobService()
// b, err := storage.NewAccountSASClientFromEndpointToken(u.String(), u.Query().Encode())
// if err != nil {
// 	return errors.Wrap(err, "failed to create storage account client")
// }

// blobService := b.GetBlobService()
// containerPath := strings.TrimLeft(u.Path, "/")
// splitPath := strings.SplitN(containerPath, "/", 2) //nolint:gomnd // TODO string splitting probably isn't the right way to parse this URL?
// containerName := splitPath[0]

// params := storage.ListBlobsParameters{Prefix: *opts.Name}
// blobList, err := blobService.GetContainerReference(containerName).ListBlobs(params)
// if err != nil {
// 	return errors.Wrap(err, "failed to list blobstore ")
// }

// if len(blobList.Blobs) == 0 {
// 	return errors.Errorf("no blobs found with prefix: %s", *opts.Name)
// }

// for _, v := range blobList.Blobs {
// 	blob := blobService.GetContainerReference(containerName).GetBlobReference(v.Name)
// 	readCloser, err := blob.Get(&storage.GetBlobOptions{})
// 	if err != nil {
// 		return errors.Wrap(err, "failed to read from blobstore")
// 	}

// 	defer readCloser.Close()

// 	blobData, err := io.ReadAll(readCloser)
// 	if err != nil {
// 		return errors.Wrap(err, "failed to obtain blob from blobstore")
// 	}

// 	err = os.WriteFile(v.Name, blobData, 0o644) //nolint:gosec,gomnd // intentionally permissive bitmask
// 	if err != nil {
// 		return errors.Wrap(err, "failed to write file")
// 	}
// 	fmt.Println("Downloaded blob: ", v.Name)
// }
// return nil
// }

func init() {
	capture.AddCommand(downloadCapture)
	downloadCapture.Flags().StringVar(&jobName, "job", "", "Name of the capture job")
}
