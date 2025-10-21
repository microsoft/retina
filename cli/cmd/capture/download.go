// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"bytes"
	"context"
	"errors"
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
	captureUtils "github.com/microsoft/retina/pkg/capture/utils"
	captureLabels "github.com/microsoft/retina/pkg/label"
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

// NodeOS represents the operating system of a Kubernetes node
type NodeOS *int

const (
	LinuxOS   = 0
	WindowsOS = 1
)

var (
	Linux   NodeOS = &[]int{LinuxOS}[0]
	Windows NodeOS = &[]int{WindowsOS}[0]
)

const (
	DefaultOutputPath = "./"
)

var (
	blobURL               string
	ErrCreateDirectory    = errors.New("failed to create directory")
	ErrGetNodeInfo        = errors.New("failed to get node information")
	ErrWriteFileToHost    = errors.New("failed to write file to host")
	ErrObtainPodList      = errors.New("failed to obtain list of pods")
	ErrExecFileDownload   = errors.New("failed to exec file download in container")
	ErrCreateDownloadPod  = errors.New("failed to create download pod")
	ErrGetDownloadPod     = errors.New("failed to get download pod")
	ErrCheckFileExistence = errors.New("failed to check file existence")
	ErrCreateExecutor     = errors.New("failed to create executor")
	ErrExecCommand        = errors.New("failed to exec command")
	ErrCreateOutputDir    = errors.New("failed to create output directory")
	ErrNoBlobsFound       = errors.New("no blobs found with prefix")
	captureName           string
	outputPath            string
)

var (
	ErrNoPodFound                = errors.New("no pod found for job")
	ErrManyPodsFound             = errors.New("more than one pod found for job; expected exactly one")
	ErrCaptureContainerNotFound  = errors.New("capture container not found in pod")
	ErrFileNotAccessible         = errors.New("file does not exist or is not readable")
	ErrEmptyDownloadOutput       = errors.New("download command produced no output")
	ErrFailedToCreateDownloadPod = errors.New("failed to create download pod")
	ErrUnsupportedNodeOS         = errors.New("unsupported node operating system")
)

// DownloadCmd holds all OS-specific commands and configurations
type DownloadCmd struct {
	ContainerImage   string
	SrcFilePath      string
	MountPath        string
	KeepAliveCommand []string
	FileCheckCommand []string
	FileReadCommand  []string
}

// DownloadService encapsulates the download functionality and shared dependencies
type DownloadService struct {
	kubeClient kubernetes.Interface
	config     *rest.Config
	namespace  string
}

// NewDownloadService creates a new download service with shared dependencies
func NewDownloadService(kubeClient kubernetes.Interface, config *rest.Config, namespace string) *DownloadService {
	return &DownloadService{
		kubeClient: kubeClient,
		config:     config,
		namespace:  namespace,
	}
}

func getDownloadCmd(node *corev1.Node, hostPath, fileName string) (*DownloadCmd, error) {
	nodeOS, err := getNodeOS(node)
	if err != nil {
		return nil, err
	}

	if nodeOS == nil {
		return nil, ErrUnsupportedNodeOS
	}

	switch *nodeOS {
	case WindowsOS:
		srcFilePath := "C:\\host" + strings.ReplaceAll(hostPath, "/", "\\") + "\\" + fileName + ".tar.gz"
		mountPath := "C:\\host" + strings.ReplaceAll(hostPath, "/", "\\")
		return &DownloadCmd{
			ContainerImage:   getWindowsContainerImage(node),
			SrcFilePath:      srcFilePath,
			MountPath:        mountPath,
			KeepAliveCommand: []string{"cmd", "/c", "echo Download pod ready & ping -n 3601 127.0.0.1 > nul"},
			FileCheckCommand: []string{"cmd", "/c", fmt.Sprintf("if exist %s echo FILE_EXISTS", srcFilePath)},
			FileReadCommand:  []string{"cmd", "/c", "type", srcFilePath},
		}, nil
	case LinuxOS:
		srcFilePath := "/" + filepath.Join("host", hostPath, fileName) + ".tar.gz"
		mountPath := "/" + filepath.Join("host", hostPath)
		return &DownloadCmd{
			ContainerImage:   "mcr.microsoft.com/azurelinux/busybox:1.36",
			SrcFilePath:      srcFilePath,
			MountPath:        mountPath,
			KeepAliveCommand: []string{"sh", "-c", "echo 'Download pod ready'; sleep 3600"},
			FileCheckCommand: []string{"sh", "-c", fmt.Sprintf("if [ -r %q ]; then echo 'FILE_EXISTS'; fi", srcFilePath)},
			FileReadCommand:  []string{"cat", srcFilePath},
		}, nil
	default:
		return nil, ErrUnsupportedNodeOS
	}
}

func getNodeOS(node *corev1.Node) (NodeOS, error) {
	nodeOS := strings.ToLower(node.Status.NodeInfo.OperatingSystem)

	if strings.Contains(nodeOS, "windows") {
		retinacmd.Logger.Info("Detected node OS: Windows", zap.String("node", node.Name), zap.String("os", node.Status.NodeInfo.OperatingSystem))
		return Windows, nil
	}

	if strings.Contains(nodeOS, "linux") {
		retinacmd.Logger.Info("Detected node OS: Linux", zap.String("node", node.Name), zap.String("os", node.Status.NodeInfo.OperatingSystem))
		return Linux, nil
	}

	return nil, fmt.Errorf("unsupported operating system: %s: %w", node.Status.NodeInfo.OperatingSystem, ErrUnsupportedNodeOS)
}

// Detects the Windows LTSC version and returns the appropriate nanoserver image
func getWindowsContainerImage(node *corev1.Node) string {
	osImage := strings.ToLower(node.Status.NodeInfo.OSImage)

	var suffix string
	switch {
	case strings.Contains(osImage, "2025"):
		suffix = "ltsc2025"
	case strings.Contains(osImage, "2022"):
		suffix = "ltsc2022"
	case strings.Contains(osImage, "2019"):
		suffix = "ltsc2019"
	case strings.Contains(osImage, "2016"):
		suffix = "ltsc2016"
	default:
		retinacmd.Logger.Warn("Could not determine Windows LTSC version, defaulting to ltsc2022",
			zap.String("node", node.Name),
			zap.String("osImage", osImage))
		suffix = "ltsc2022"
	}

	containerImage := "mcr.microsoft.com/windows/nanoserver:" + suffix
	retinacmd.Logger.Info("Selected Windows container image", zap.String("image", containerImage))

	return containerImage
}

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
	fmt.Println("Downloading capture: ", captureName)
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to initialize k8s client: %w", err)
	}

	downloadService := NewDownloadService(kubeClient, config, namespace)

	pods, err := getCapturePods(ctx, kubeClient, captureName, namespace)
	if err != nil {
		return fmt.Errorf("failed to obtain capture pod: %w", err)
	}

	err = os.MkdirAll(filepath.Join(outputPath, captureName), 0o775)
	if err != nil {
		return errors.Join(ErrCreateDirectory, err)
	}

	for i := range pods.Items {
		pod := pods.Items[i]
		if pod.Status.Phase != corev1.PodSucceeded {
			return fmt.Errorf("%s: %w", captureName, ErrNoPodFound)
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

		err = downloadService.DownloadFile(ctx, nodeName, hostPath, fileName, captureName)
		if err != nil {
			return err
		}
	}

	return nil
}

// DownloadFile downloads a capture file from a specific node
func (ds *DownloadService) DownloadFile(ctx context.Context, nodeName, hostPath, fileName, captureName string) error {
	node, err := ds.kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return errors.Join(ErrGetNodeInfo, err)
	}

	downloadCmd, err := getDownloadCmd(node, hostPath, fileName)
	if err != nil {
		return err
	}

	fmt.Println("File to be downloaded: ", downloadCmd.SrcFilePath)
	downloadPod, err := ds.createDownloadPod(ctx, nodeName, hostPath, captureName, downloadCmd)
	if err != nil {
		return err
	}

	fileExists, err := ds.verifyFileExists(ctx, downloadPod, downloadCmd)
	if err != nil || !fileExists {
		return err
	}

	fmt.Println("Obtaining file...")
	fileContent, err := ds.executeFileDownload(ctx, downloadPod, downloadCmd)
	if err != nil {
		return err
	}

	outputFile := filepath.Join(outputPath, captureName, fileName+".tar.gz")
	fmt.Printf("Bytes retrieved: %d\n", len(fileContent))

	err = os.WriteFile(outputFile, fileContent, 0o600)
	if err != nil {
		return errors.Join(ErrWriteFileToHost, err)
	}

	fmt.Printf("File written to: %s\n", outputFile)

	// Ensure cleanup
	err = ds.kubeClient.CoreV1().Pods(ds.namespace).Delete(ctx, downloadPod.Name, metav1.DeleteOptions{})
	if err != nil {
		retinacmd.Logger.Warn("Failed to clean up debug pod", zap.String("name", downloadPod.Name), zap.Error(err))
	}
	return nil
}

func getCapturePods(ctx context.Context, kubeClient kubernetes.Interface, captureName, namespace string) (*corev1.PodList, error) {
	pods, err := kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: captureLabels.CaptureNameLabel + "=" + captureName,
	})
	if err != nil {
		return &corev1.PodList{}, errors.Join(ErrObtainPodList, err)
	}
	if len(pods.Items) == 0 {
		return &corev1.PodList{}, fmt.Errorf("%s: %w", captureName, ErrNoPodFound)
	}

	return pods, nil
}

// executeFileDownload downloads the file content from the pod
func (ds *DownloadService) executeFileDownload(ctx context.Context, pod *corev1.Pod, downloadCmd *DownloadCmd) ([]byte, error) {
	content, err := ds.createDownloadExec(ctx, pod, downloadCmd.FileReadCommand)
	if err != nil {
		return nil, errors.Join(ErrExecFileDownload, err)
	}

	if content == "" {
		return nil, ErrEmptyDownloadOutput
	}

	return []byte(content), nil
}

// createDownloadPod creates a pod for downloading files from the host
func (ds *DownloadService) createDownloadPod(ctx context.Context, nodeName, hostPath, captureName string, downloadCmd *DownloadCmd) (*corev1.Pod, error) {
	podName := captureName + "-download-" + rand.String(5)

	podSpec := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: ds.namespace,
			Labels:    captureUtils.GetDownloadLabelsFromCaptureName(captureName),
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{
				{
					Name:    captureConstants.DownloadContainerName,
					Image:   downloadCmd.ContainerImage,
					Command: downloadCmd.KeepAliveCommand,
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "host-mount",
							MountPath: downloadCmd.MountPath,
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
	_, err := ds.kubeClient.CoreV1().Pods(ds.namespace).Create(ctx, podSpec, metav1.CreateOptions{})
	if err != nil {
		return nil, errors.Join(ErrCreateDownloadPod, err)
	}

	return ds.waitForPodReady(ctx, podName)
}

// waitForPodReady waits for the pod to be in running state
func (ds *DownloadService) waitForPodReady(ctx context.Context, podName string) (*corev1.Pod, error) {
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for download pod to become ready: %w", ErrFailedToCreateDownloadPod)
		case <-ticker.C:
			pod, err := ds.kubeClient.CoreV1().Pods(ds.namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				return nil, errors.Join(ErrGetDownloadPod, err)
			}
			if pod.Status.Phase == corev1.PodRunning {
				return pod, nil
			}
			if pod.Status.Phase == corev1.PodFailed {
				return nil, fmt.Errorf("download pod failed to spin up successfully: %w", ErrFailedToCreateDownloadPod)
			}
		}
	}
}

// verifyFileExists checks if the target file exists and is accessible
func (ds *DownloadService) verifyFileExists(ctx context.Context, pod *corev1.Pod, downloadCmd *DownloadCmd) (bool, error) {
	maxAttempts := 3

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		checkOutput, err := ds.createDownloadExec(ctx, pod, downloadCmd.FileCheckCommand)
		if err != nil {
			if attempt == maxAttempts {
				return false, fmt.Errorf("failed to check file existence after %d attempts: %w", attempt, err)
			}
			time.Sleep(time.Duration(attempt*2) * time.Second)
			continue
		}

		if strings.Contains(checkOutput, "FILE_EXISTS") {
			return true, nil
		}

		time.Sleep(time.Duration(attempt*2) * time.Second)
	}

	return false, fmt.Errorf("%s: %w", downloadCmd.SrcFilePath, ErrFileNotAccessible)
}

// createDownloadExec executes a command in the pod and returns the output
func (ds *DownloadService) createDownloadExec(ctx context.Context, pod *corev1.Pod, command []string) (string, error) {
	req := ds.kubeClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: captureConstants.DownloadContainerName,
			Command:   command,
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(ds.config, "POST", req.URL())
	if err != nil {
		return "", errors.Join(ErrCreateExecutor, err)
	}

	var outBuf, errBuf bytes.Buffer
	streamOpts := remotecommand.StreamOptions{
		Stdout: &outBuf,
		Stderr: &errBuf,
	}

	if err = exec.StreamWithContext(ctx, streamOpts); err != nil {
		return "", fmt.Errorf("failed to exec command (stderr: %s): %w", errBuf.String(), err)
	}

	return outBuf.String(), nil
}

func downloadFromBlob() error {
	u, err := url.Parse(blobURL)
	if err != nil {
		retinacmd.Logger.Error("err: ", zap.Error(err))
		return fmt.Errorf("failed to parse SAS URL %s: %w", blobURL, err)
	}

	b, err := storage.NewAccountSASClientFromEndpointToken(u.String(), u.Query().Encode())
	if err != nil {
		retinacmd.Logger.Error("err: ", zap.Error(err))
		return fmt.Errorf("failed to create storage account client: %w", err)
	}

	blobService := b.GetBlobService()
	containerPath := strings.TrimLeft(u.Path, "/")
	splitPath := strings.SplitN(containerPath, "/", 2)
	containerName := splitPath[0]

	params := storage.ListBlobsParameters{Prefix: *opts.Name}
	blobList, err := blobService.GetContainerReference(containerName).ListBlobs(params)
	if err != nil {
		retinacmd.Logger.Error("err: ", zap.Error(err))
		return fmt.Errorf("failed to list blobstore: %w", err)
	}

	if len(blobList.Blobs) == 0 {
		retinacmd.Logger.Error("err: ", zap.Error(err))
		return fmt.Errorf("%w: %s", ErrNoBlobsFound, *opts.Name)
	}

	err = os.MkdirAll(outputPath, 0o775)
	if err != nil {
		return errors.Join(ErrCreateOutputDir, err)
	}

	for i := range blobList.Blobs {
		blob := blobList.Blobs[i]
		blobRef := blobService.GetContainerReference(containerName).GetBlobReference(blob.Name)
		readCloser, err := blobRef.Get(&storage.GetBlobOptions{})
		if err != nil {
			retinacmd.Logger.Error("err: ", zap.Error(err))
			return fmt.Errorf("failed to read from blobstore: %w", err)
		}

		blobData, err := io.ReadAll(readCloser)
		readCloser.Close()
		if err != nil {
			retinacmd.Logger.Error("err: ", zap.Error(err))
			return fmt.Errorf("failed to obtain blob from blobstore: %w", err)
		}

		outputFile := filepath.Join(outputPath, blob.Name)
		err = os.WriteFile(outputFile, blobData, 0o600)
		if err != nil {
			retinacmd.Logger.Error("err: ", zap.Error(err))
			return fmt.Errorf("failed to write file: %w", err)
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
				return fmt.Errorf("failed to compose k8s rest config: %w", err)
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
