// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"bytes"
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
	captureLabels "github.com/microsoft/retina/pkg/label"
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

const (
	DefaultOutputPath = "./"
)

var (
	blobURL     string
	captureName string
	outputPath  string
)

var (
	ErrNoPodFound                = errors.New("no pod found for job")
	ErrManyPodsFound             = errors.New("more than one pod found for job; expected exactly one")
	ErrCaptureContainerNotFound  = errors.New("capture container not found in pod")
	ErrFileNotAccessible         = errors.New("file does not exist or is not readable")
	ErrEmptyDownloadOutput       = errors.New("download command produced no output")
	ErrFailedToCreateDownloadPod = errors.New("failed to create download pod")
)

var downloadExample = templates.Examples(i18n.T(`
		# List Retina capture jobs
		kubectl retina capture list

		# Download the capture file(s) created using the capture name
		kubectl retina capture download --name <capture-name>

		# Download the capture file(s) created using the capture name and define output location
		kubectl retina capture download --name <capture-name> -o <output-location>

		# Download capture file(s) from Blob Storage via Blob URL (Blob URL requires Read/List permissions)
		kubectl retina capture download --blob-url "<blob-url>"
`))

func downloadFromCluster(ctx context.Context, config *rest.Config, namespace string) error {
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return errors.Wrap(err, "failed to initialize k8s client")
	}

	pods, err := getCapturePods(ctx, kubeClient, captureName, namespace)
	if err != nil {
		return errors.Wrap(err, "failed to obtain capture pod")
	}

	err = os.MkdirAll(filepath.Join(outputPath, captureName), 0o775)
	if err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	for i := range pods.Items {
		pod := pods.Items[i]
		if pod.Status.Phase != corev1.PodSucceeded {
			return errors.Wrap(ErrNoPodFound, captureName)
		}

		nodeName := pod.Spec.NodeName
		hostPath, ok := pod.Annotations[captureConstants.CaptureHostPathAnnotationKey]
		if !ok {
			return errors.New("cannot obtain host path from pod annotations")
		}
		fileName, ok := pod.Annotations[captureConstants.CaptureFilenameAnnotationKey]
		if !ok {
			return errors.New("cannot obtain capture file name from pod annotations")
		}

		srcFilePath := "/" + filepath.Join("host", hostPath, fileName) + ".tar.gz"
		fmt.Println("\nFile to be downloaded: ", srcFilePath)
		downloadPod, err := createDownloadPod(ctx, kubeClient, namespace, nodeName, hostPath, captureName)
		if err != nil {
			return err
		}

		fmt.Println("Obtaining file...")
		exec, err := createDownloadExec(ctx, kubeClient, config, downloadPod, srcFilePath)
		if err != nil {
			return err
		}

		var outBuf, errBuf bytes.Buffer
		streamOpts := remotecommand.StreamOptions{
			Stdout: &outBuf,
			Stderr: &errBuf,
		}
		if err = exec.StreamWithContext(ctx, streamOpts); err != nil {
			return fmt.Errorf("failed to exec in download container: %w", err)
		}

		if outBuf.Len() == 0 {
			return errors.Wrap(ErrEmptyDownloadOutput, errBuf.String())
		}

		outputFile := filepath.Join(outputPath, captureName, fileName+".tar.gz")
		fmt.Printf("Bytes retrieved: %d\n", outBuf.Len())

		err = os.WriteFile(outputFile, outBuf.Bytes(), 0o600)
		if err != nil {
			return fmt.Errorf("failed to write file to host: %w", err)
		}

		fmt.Println("File written to: ", outputFile)

		err = kubeClient.CoreV1().Pods(namespace).Delete(ctx, downloadPod.Name, metav1.DeleteOptions{})
		if err != nil {
			retinacmd.Logger.Warn("Failed to clean up debug pod", zap.String("name", downloadPod.Name), zap.Error(err))
		}
	}

	return nil
}

func getCapturePods(ctx context.Context, kubeClient *kubernetes.Clientset, captureName, namespace string) (*corev1.PodList, error) {
	pods, err := kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: captureLabels.CaptureNameLabel + "=" + captureName,
	})
	if err != nil {
		return &corev1.PodList{}, fmt.Errorf("failed to obtain list of pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return &corev1.PodList{}, errors.Wrap(ErrNoPodFound, captureName)
	}

	return pods, nil
}

func createDownloadPod(ctx context.Context, kubeClient *kubernetes.Clientset, namespace, nodeName, hostPath, captureName string) (*corev1.Pod, error) {
	podName := captureName + "-download-" + rand.String(5)

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
					Command: []string{"sh", "-c", "echo 'Download pod ready'; sleep 3600"},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "host-mount",
							MountPath: "/" + filepath.Join("host", hostPath),
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
							Path: hostPath,
						},
					},
				},
			},
		},
	}

	fmt.Printf("Creating download pod: %s\n", podName)
	_, err := kubeClient.CoreV1().Pods(namespace).Create(ctx, podSpec, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create download pod: %w", err)
	}

	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	var pod *corev1.Pod
	for {
		select {
		case <-timeout:
			return nil, errors.Wrap(ErrFailedToCreateDownloadPod, "timeout waiting for download pod to become ready")
		case <-ticker.C:
			pod, err = kubeClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to get download pod: %w", err)
			}
			if pod.Status.Phase == corev1.PodRunning {
				return pod, nil
			}
			if pod.Status.Phase == corev1.PodFailed {
				return nil, errors.Wrap(ErrFailedToCreateDownloadPod, "download pod failed to spin up successfully")
			}
		}
	}
}

func verifyFileExists(ctx context.Context, kubeClient *kubernetes.Clientset, config *rest.Config, pod *corev1.Pod, filePath string) (bool, error) {
	maxAttempts := 3

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		checkReq := kubeClient.CoreV1().RESTClient().Post().
			Resource("pods").
			Name(pod.Name).
			Namespace(pod.Namespace).
			SubResource("exec").
			VersionedParams(&corev1.PodExecOptions{
				Container: "download",
				Command:   []string{"sh", "-c", fmt.Sprintf("if [ -r %q ]; then echo 'FILE_EXISTS'; fi", filePath)},
				Stdout:    true,
				Stderr:    true,
			}, scheme.ParameterCodec)

		checkExec, err := remotecommand.NewSPDYExecutor(config, "POST", checkReq.URL())
		if err != nil {
			if attempt == maxAttempts {
				return false, fmt.Errorf("failed to create check executor after %d attempts: %w", attempt, err)
			}
			time.Sleep(time.Duration(attempt*2) * time.Second)
			continue
		}

		var checkBuf bytes.Buffer
		if err = checkExec.StreamWithContext(ctx, remotecommand.StreamOptions{
			Stdout: &checkBuf,
			Stderr: &checkBuf,
		}); err != nil {
			if attempt == maxAttempts {
				return false, fmt.Errorf("failed to check file existence after %d attempts: %w", attempt, err)
			}
			time.Sleep(time.Duration(attempt*2) * time.Second)
			continue
		}

		checkOutput := checkBuf.String()

		if strings.Contains(checkOutput, "FILE_EXISTS") {
			return true, nil
		}

		time.Sleep(time.Duration(attempt*2) * time.Second)
	}

	return false, errors.Wrap(ErrFileNotAccessible, filePath)
}

func createDownloadExec(ctx context.Context, kubeClient *kubernetes.Clientset, config *rest.Config, pod *corev1.Pod, srcFilePath string) (remotecommand.Executor, error) {
	fileExists, err := verifyFileExists(ctx, kubeClient, config, pod, srcFilePath)
	if err != nil || !fileExists {
		return nil, err
	}

	req := kubeClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: "download",
			Command:   []string{"cat", srcFilePath},
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}
	return exec, nil
}

func downloadFromBlob() error {
	u, err := url.Parse(blobURL)
	if err != nil {
		retinacmd.Logger.Error("err: ", zap.Error(err))
		return errors.Wrapf(err, "failed to parse SAS URL %s", blobURL)
	}

	b, err := storage.NewAccountSASClientFromEndpointToken(u.String(), u.Query().Encode())
	if err != nil {
		retinacmd.Logger.Error("err: ", zap.Error(err))
		return errors.Wrap(err, "failed to create storage account client")
	}

	blobService := b.GetBlobService()
	containerPath := strings.TrimLeft(u.Path, "/")
	splitPath := strings.SplitN(containerPath, "/", 2)
	containerName := splitPath[0]

	params := storage.ListBlobsParameters{Prefix: *opts.Name}
	blobList, err := blobService.GetContainerReference(containerName).ListBlobs(params)
	if err != nil {
		retinacmd.Logger.Error("err: ", zap.Error(err))
		return errors.Wrap(err, "failed to list blobstore ")
	}

	if len(blobList.Blobs) == 0 {
		retinacmd.Logger.Error("err: ", zap.Error(err))
		return errors.Errorf("no blobs found with prefix: %s", *opts.Name)
	}

	err = os.MkdirAll(outputPath, 0o775)
	if err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	for i := range blobList.Blobs {
		blob := blobList.Blobs[i]
		blobRef := blobService.GetContainerReference(containerName).GetBlobReference(blob.Name)
		readCloser, err := blobRef.Get(&storage.GetBlobOptions{})
		if err != nil {
			retinacmd.Logger.Error("err: ", zap.Error(err))
			return errors.Wrap(err, "failed to read from blobstore")
		}

		blobData, err := io.ReadAll(readCloser)
		readCloser.Close()
		if err != nil {
			retinacmd.Logger.Error("err: ", zap.Error(err))
			return errors.Wrap(err, "failed to obtain blob from blobstore")
		}

		outputFile := filepath.Join(outputPath, blob.Name)
		err = os.WriteFile(outputFile, blobData, 0o600)
		if err != nil {
			retinacmd.Logger.Error("err: ", zap.Error(err))
			return errors.Wrap(err, "failed to write file")
		}

		fmt.Println("Downloaded: ", outputFile)
	}
	return nil
}

func NewDownloadSubCommand() *cobra.Command {
	downloadCapture := &cobra.Command{
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

			if captureName == "" && blobURL == "" {
				return errors.New("either --name or --blob-url must be specified")
			}

			if captureName != "" {
				err = downloadFromCluster(ctx, kubeConfig, captureNamespace)
				if err != nil {
					return err
				}
			}

			if blobURL != "" {
				err = downloadFromBlob()
				if err != nil {
					return err
				}
			}

			return nil
		},
	}

	downloadCapture.Flags().StringVar(&blobURL, "blob-url", "", "Blob URL from which to download")
	downloadCapture.Flags().StringVar(&captureName, "name", "", "The name of a the capture")
	downloadCapture.Flags().StringVarP(&outputPath, "output", "o", DefaultOutputPath, "Path to save the downloaded capture")

	return downloadCapture
}
