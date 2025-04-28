// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Azure/azure-sdk-for-go/storage"
	retinacmd "github.com/microsoft/retina/cli/cmd"
	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/capture/file"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
)

// Type ContainerEnvVars contains the required values to rebuild the name and location of a capture file on the host.
type ContainerEnvVars struct {
	hostPath         string
	nodeHostName     string
	captureName      string
	captureStartTime string
}

const MountPath = "/host/mnt/retina/"
const DefaultOutputPath = "/tmp/retina/capture/"

var blobURL string
var extract bool
var jobName string
var outputPath string

var ErrNoPodFound = errors.New("no pod found for job")
var ErrManyPodsFound = errors.New("more than one pod found for job; expected exactly one")
var ErrCaptureContainerNotFound = errors.New("capture container not found in pod")
var ErrDownloadPodFailed = errors.New("download pod failed to spin up successfully")

var downloadExample = templates.Examples(i18n.T(`
		# List Retina capture jobs
		kubectl retina capture list

		# Download the capture file created by a job
		kubectl retina capture download --job <job-name>

		# Download the capture file created by a job and automatically extract the tarball
		kubectl retina capture download --job <job-name> -e

		# Download the capture file created by a job and define output location
		kubectl retina capture download --job <job-name> -o <output-location>

		# Download capture files from Blob Storage via Blob URL (Blob URl requires Read/List permissions)
		kubectl retina capture download --blob-url "<blob-url>"
`))

var downloadCapture = &cobra.Command{
	Use:     "download",
	Short:   "Download Retina Captures",
	Example: downloadExample,
	RunE: func(*cobra.Command, []string) error {
		viper.AutomaticEnv()

		kubeConfig, err := opts.ToRESTConfig()
		if err != nil {
			return errors.Wrap(err, "failed to compose k8s rest config")
		}

		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM)
		defer cancel()

		captureNamespace := *opts.Namespace
		if captureNamespace == "" {
			captureNamespace = "default"
		}

		downloadedFiles := []string{}

		if jobName != "" {
			var files []string
			files, err = downloadFromCluster(ctx, kubeConfig, captureNamespace)
			if err != nil {
				return err
			}
			downloadedFiles = append(downloadedFiles, files...)
		}

		if blobURL != "" {
			var files []string
			files, err = downloadFromBlob()
			if err != nil {
				return err
			}
			downloadedFiles = append(downloadedFiles, files...)
		}

		if extract {
			for _, outputFile := range downloadedFiles {
				err = extractFiles(outputFile, outputPath)
				if err != nil {
					return err
				}
			}
			fmt.Println("Extracted within: ", outputPath)
		}

		return err
	},
}

func downloadFromCluster(ctx context.Context, config *rest.Config, namespace string) ([]string, error) {
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize k8s client")
	}

	// Currently there is no direct way to obtain the name of the capture from the job / pod / container
	// To re-create it ourselves we do the following
	// 1. Get pod where the capture job ran
	// 2. Get the capture container
	// 3. Use env variables from the container to re-create the file name

	// In future, the file name should be a Pod level Label / Annotation so we don't need to dig into Container Env Vars

	pod, err := getCapturePod(ctx, kubeClient, jobName, namespace)
	if err != nil {
		return nil, errors.Wrap(err, "failed to obtain capture pod")
	}

	container, err := getCaptureContainer(pod)
	if err != nil {
		return nil, errors.Wrap(err, "failed to obtain capture container")
	}

	env := getCaptureEnvironment(container)
	fileName, err := getCaptureFileName(env)
	if err != nil {
		return nil, errors.Wrap(err, "failed to recreate capture file name")
	}

	srcFilePath := MountPath + fileName
	fmt.Println("File to be downloaded: ", srcFilePath)
	downloadPod, err := createDownloadPod(ctx, kubeClient, namespace, env.nodeHostName, env.hostPath, jobName)
	if err != nil {
		return nil, err
	}

	exec, err := createDownloadExec(kubeClient, config, downloadPod, srcFilePath)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	streamOpts := remotecommand.StreamOptions{
		Stdout: &buf,
		Stderr: &buf,
	}
	if err := exec.StreamWithContext(ctx, streamOpts); err != nil {
		return nil, fmt.Errorf("failed to exec tar in container: %w", err)
	}

	outputFile := filepath.Join(outputPath, fileName)
	err = os.MkdirAll(outputPath, 0o775)
	if err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}
	err = os.WriteFile(outputFile, buf.Bytes(), 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to write file to host: %w", err)
	}
	fmt.Println("File written to: ", outputFile)

	err = kubeClient.CoreV1().Pods(namespace).Delete(ctx, downloadPod.Name, metav1.DeleteOptions{})
	if err != nil {
		retinacmd.Logger.Warn("Failed to clean up debug pod", zap.String("name", downloadPod.Name), zap.Error(err))
	}

	return []string{outputFile}, nil
}

func getCapturePod(ctx context.Context, kubeClient *kubernetes.Clientset, jobName, namespace string) (corev1.Pod, error) {
	pods, err := kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil {
		return corev1.Pod{}, err
	}
	if len(pods.Items) == 0 {
		return corev1.Pod{}, errors.Wrap(ErrNoPodFound, jobName)
	}
	// The assumption is that the capture job only runs on one pod.
	if len(pods.Items) > 1 {
		return corev1.Pod{}, errors.Wrap(ErrManyPodsFound, jobName)
	}

	return pods.Items[0], nil
}

func getCaptureContainer(pod corev1.Pod) (*corev1.Container, error) {
	containerName := captureConstants.CaptureContainername
	var targetContainer *corev1.Container
	for i, c := range pod.Spec.Containers {
		if c.Name == containerName {
			targetContainer = &pod.Spec.Containers[i]
			break
		}
	}
	if targetContainer == nil {
		return nil, errors.Wrap(ErrCaptureContainerNotFound, pod.Name)
	}
	return targetContainer, nil
}

func getCaptureEnvironment(container *corev1.Container) ContainerEnvVars {
	var captureEnv = ContainerEnvVars{}

	for _, env := range container.Env {
		switch env.Name {
		case string(captureConstants.CaptureOutputLocationEnvKeyHostPath):
			captureEnv.hostPath = env.Value
		case string(captureConstants.NodeHostNameEnvKey):
			captureEnv.nodeHostName = env.Value
		case string(captureConstants.CaptureNameEnvKey):
			captureEnv.captureName = env.Value
		case string(captureConstants.CaptureStartTimestampEnvKey):
			captureEnv.captureStartTime = env.Value
		}
	}

	return captureEnv
}

func getCaptureFileName(env ContainerEnvVars) (string, error) {
	timestamp, err := file.StringToTimestamp(env.captureStartTime)
	if err != nil {
		return "", err
	}

	captureFile := file.CaptureFilename{
		CaptureName:    env.captureName,
		NodeHostname:   env.nodeHostName,
		StartTimestamp: timestamp,
	}

	fileName := captureFile.String() + ".tar.gz"
	return fileName, nil
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

	_, err := kubeClient.CoreV1().Pods(namespace).Create(ctx, podSpec, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create debug pod: %w", err)
	}

	fmt.Println("Creating download pod to retrieve the files...")
	for {
		time.Sleep(1 * time.Second)
		pod, err := kubeClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if pod.Status.Phase == corev1.PodRunning {
			return pod, nil
		}
		if pod.Status.Phase == corev1.PodFailed {
			return nil, ErrDownloadPodFailed
		}
	}
}

func createDownloadExec(kubeClient *kubernetes.Clientset, config *rest.Config, pod *corev1.Pod, srcFilePath string) (remotecommand.Executor, error) {
	req := kubeClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace("default").
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: "download",
			Command:   []string{"tar", "czf", "-", "-C", filepath.Dir(srcFilePath), filepath.Base(srcFilePath)},
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}
	return exec, nil
}

func downloadFromBlob() ([]string, error) {
	u, err := url.Parse(blobURL)
	if err != nil {
		retinacmd.Logger.Error("err: ", zap.Error(err))
		return nil, errors.Wrapf(err, "failed to parse SAS URL %s", blobURL)
	}

	b, err := storage.NewAccountSASClientFromEndpointToken(u.String(), u.Query().Encode())
	if err != nil {
		retinacmd.Logger.Error("err: ", zap.Error(err))
		return nil, errors.Wrap(err, "failed to create storage account client")
	}

	blobService := b.GetBlobService()
	containerPath := strings.TrimLeft(u.Path, "/")
	splitPath := strings.SplitN(containerPath, "/", 2)
	containerName := splitPath[0]

	params := storage.ListBlobsParameters{Prefix: *opts.Name}
	blobList, err := blobService.GetContainerReference(containerName).ListBlobs(params)
	if err != nil {
		retinacmd.Logger.Error("err: ", zap.Error(err))
		return nil, errors.Wrap(err, "failed to list blobstore ")
	}

	if len(blobList.Blobs) == 0 {
		retinacmd.Logger.Error("err: ", zap.Error(err))
		return nil, errors.Errorf("no blobs found with prefix: %s", *opts.Name)
	}

	err = os.MkdirAll(outputPath, 0o775)
	if err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	files := []string{}
	for _, v := range blobList.Blobs {
		blob := blobService.GetContainerReference(containerName).GetBlobReference(v.Name)
		readCloser, err := blob.Get(&storage.GetBlobOptions{})
		if err != nil {
			retinacmd.Logger.Error("err: ", zap.Error(err))
			return nil, errors.Wrap(err, "failed to read from blobstore")
		}
		defer readCloser.Close()

		blobData, err := io.ReadAll(readCloser)
		if err != nil {
			retinacmd.Logger.Error("err: ", zap.Error(err))
			return nil, errors.Wrap(err, "failed to obtain blob from blobstore")
		}

		outputFile := filepath.Join(outputPath, v.Name)
		err = os.WriteFile(outputFile, blobData, 0o644)
		if err != nil {
			retinacmd.Logger.Error("err: ", zap.Error(err))
			return nil, errors.Wrap(err, "failed to write file")
		}

		files = append(files, outputFile)
		fmt.Println("Downloaded: ", outputFile)
	}
	return files, nil
}

func extractFiles(srcFile, outputDir string) error {
	output := strings.TrimSuffix(filepath.Base(srcFile), ".tar.gz")
	dest := filepath.Join(outputDir, output)
	if err := os.MkdirAll(dest, 0o775); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	file, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer file.Close()

	return processTarGz(file, dest)
}

func processTarGz(r io.Reader, destDir string) error {
	gzReader, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzReader.Close()
	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		targetPath := filepath.Join(destDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o775); err != nil {
				return err
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o775); err != nil {
				return err
			}

			data, err := io.ReadAll(tarReader)
			if err != nil {
				return err
			}

			// Check if this is a nested tar.gz file by checking the header for the "gzip magic number"
			isGzip := len(data) > 2 && data[0] == 0x1f && data[1] == 0x8b

			if isGzip {
				reader := bytes.NewReader(data)
				err = processTarGz(reader, destDir)
				if err != nil {
					return err
				}
			} else {
				err = saveFile(targetPath, data)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func saveFile(path string, data []byte) error {
	outFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = outFile.Write(data)
	return err
}

func init() {
	capture.AddCommand(downloadCapture)
	downloadCapture.Flags().StringVar(&blobURL, "blob-url", "", "Blob URL from which to download")
	downloadCapture.Flags().BoolVarP(&extract, "extract", "e", false, "Extract the tarball upon download")
	downloadCapture.Flags().StringVar(&jobName, "job", "", "The name of a capture job")
	downloadCapture.Flags().StringVarP(&outputPath, "output", "o", DefaultOutputPath, "Path to save the downloaded capture")
}
