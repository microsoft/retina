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
	captureUtils "github.com/microsoft/retina/pkg/capture/utils"
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

// NodeOS represents the operating system of a Kubernetes node
type NodeOS int

const (
	Linux NodeOS = iota
	Windows
)

const (
	DefaultOutputPath = "./"
)

var (
	blobURL string
	// Error variables for lint compliance (err113)
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
	ctx        context.Context
}

// NewDownloadService creates a new download service with shared dependencies
func NewDownloadService(ctx context.Context, kubeClient kubernetes.Interface, config *rest.Config, namespace string) *DownloadService {
	return &DownloadService{
		kubeClient: kubeClient,
		config:     config,
		namespace:  namespace,
		ctx:        ctx,
	}
}

func getDownloadCmd(node *corev1.Node, hostPath, fileName string) *DownloadCmd {
	nodeOS, err := getNodeOS(node)
	if err != nil {
		retinacmd.Logger.Error("Failed to detect node OS", zap.String("node", node.Name), zap.Error(err))
		return nil
	}

	switch nodeOS {
	case Windows:
		srcFilePath := "C:\\host" + strings.ReplaceAll(hostPath, "/", "\\") + "\\" + fileName + ".tar.gz"
		mountPath := "C:\\host" + strings.ReplaceAll(hostPath, "/", "\\")
		return &DownloadCmd{
			ContainerImage:   getWindowsContainerImage(node),
			SrcFilePath:      srcFilePath,
			MountPath:        mountPath,
			KeepAliveCommand: []string{"cmd", "/c", "echo Download pod ready & ping -n 3601 127.0.0.1 > nul"},
			FileCheckCommand: []string{"cmd", "/c", fmt.Sprintf("if exist %s echo FILE_EXISTS", srcFilePath)},
			FileReadCommand:  []string{"cmd", "/c", "type", srcFilePath},
		}
	case Linux:
		srcFilePath := "/" + filepath.Join("host", hostPath, fileName) + ".tar.gz"
		mountPath := "/" + filepath.Join("host", hostPath)
		return &DownloadCmd{
			ContainerImage:   "mcr.microsoft.com/azurelinux/busybox:1.36",
			SrcFilePath:      srcFilePath,
			MountPath:        mountPath,
			KeepAliveCommand: []string{"sh", "-c", "echo 'Download pod ready'; sleep 3600"},
			FileCheckCommand: []string{"sh", "-c", fmt.Sprintf("if [ -r %q ]; then echo 'FILE_EXISTS'; fi", srcFilePath)},
			FileReadCommand:  []string{"cat", srcFilePath},
		}
	default:
		return nil
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

	return Linux, errors.Wrap(ErrEmptyDownloadOutput, "unsupported operating system: "+node.Status.NodeInfo.OperatingSystem)
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
		return errors.Wrap(err, "failed to initialize k8s client")
	}

	downloadService := NewDownloadService(ctx, kubeClient, config, namespace)

	pods, err := getCapturePods(ctx, kubeClient, captureName, namespace)
	if err != nil {
		return errors.Wrap(err, "failed to obtain capture pod")
	}

	err = os.MkdirAll(filepath.Join(outputPath, captureName), 0o775)
	if err != nil {
		return errors.Wrap(err, ErrCreateDirectory.Error())
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

		err = downloadService.DownloadFile(nodeName, hostPath, fileName, captureName)
		if err != nil {
			return err
		}
	}

	return nil
}

// DownloadFile downloads a capture file from a specific node
func (ds *DownloadService) DownloadFile(nodeName, hostPath, fileName, captureName string) error {
	node, err := ds.kubeClient.CoreV1().Nodes().Get(ds.ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, ErrGetNodeInfo.Error())
	}

	downloadCmd := getDownloadCmd(node, hostPath, fileName)
	if downloadCmd == nil {
		return ErrUnsupportedNodeOS
	}

	fmt.Println("File to be downloaded: ", downloadCmd.SrcFilePath)
	downloadPod, err := ds.createDownloadPod(nodeName, hostPath, captureName, downloadCmd)
	if err != nil {
		return err
	}

	fileExists, err := ds.verifyFileExists(downloadPod, downloadCmd)
	if err != nil || !fileExists {
		return err
	}

	fmt.Println("Obtaining file...")
	fileContent, err := ds.executeFileDownload(downloadPod, downloadCmd)
	if err != nil {
		return err
	}

	outputFile := filepath.Join(outputPath, captureName, fileName+".tar.gz")
	fmt.Printf("Bytes retrieved: %d\n", len(fileContent))

	err = os.WriteFile(outputFile, fileContent, 0o600)
	if err != nil {
		return errors.Wrap(err, ErrWriteFileToHost.Error())
	}

	fmt.Printf("File written to: %s\n", outputFile)

	// Ensure cleanup
	err = ds.kubeClient.CoreV1().Pods(ds.namespace).Delete(ds.ctx, downloadPod.Name, metav1.DeleteOptions{})
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
		return &corev1.PodList{}, errors.Wrap(err, ErrObtainPodList.Error())
	}
	if len(pods.Items) == 0 {
		return &corev1.PodList{}, errors.Wrap(ErrNoPodFound, captureName)
	}

	return pods, nil
}

// executeFileDownload downloads the file content from the pod
func (ds *DownloadService) executeFileDownload(pod *corev1.Pod, downloadCmd *DownloadCmd) ([]byte, error) {
	content, err := ds.createDownloadExec(pod, downloadCmd.FileReadCommand)
	if err != nil {
		return nil, errors.Wrap(err, ErrExecFileDownload.Error())
	}

	if content == "" {
		return nil, ErrEmptyDownloadOutput
	}

	return []byte(content), nil
}

// createDownloadPod creates a pod for downloading files from the host
func (ds *DownloadService) createDownloadPod(nodeName, hostPath, captureName string, downloadCmd *DownloadCmd) (*corev1.Pod, error) {
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
					Name:    captureConstants.DownloadContainername,
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
	_, err := ds.kubeClient.CoreV1().Pods(ds.namespace).Create(ds.ctx, podSpec, metav1.CreateOptions{})
	if err != nil {
		return nil, errors.Wrap(err, ErrCreateDownloadPod.Error())
	}

	return ds.waitForPodReady(podName)
}

// waitForPodReady waits for the pod to be in running state
func (ds *DownloadService) waitForPodReady(podName string) (*corev1.Pod, error) {
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return nil, errors.Wrap(ErrFailedToCreateDownloadPod, "timeout waiting for download pod to become ready")
		case <-ticker.C:
			pod, err := ds.kubeClient.CoreV1().Pods(ds.namespace).Get(ds.ctx, podName, metav1.GetOptions{})
			if err != nil {
				return nil, errors.Wrap(err, ErrGetDownloadPod.Error())
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

// verifyFileExists checks if the target file exists and is accessible
func (ds *DownloadService) verifyFileExists(pod *corev1.Pod, downloadCmd *DownloadCmd) (bool, error) {
	maxAttempts := 3

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		checkOutput, err := ds.createDownloadExec(pod, downloadCmd.FileCheckCommand)
		if err != nil {
			if attempt == maxAttempts {
				return false, errors.Wrapf(err, "failed to check file existence after %d attempts", attempt)
			}
			time.Sleep(time.Duration(attempt*2) * time.Second)
			continue
		}

		if strings.Contains(checkOutput, "FILE_EXISTS") {
			return true, nil
		}

		time.Sleep(time.Duration(attempt*2) * time.Second)
	}

	return false, errors.Wrap(ErrFileNotAccessible, downloadCmd.SrcFilePath)
}

// createDownloadExec executes a command in the pod and returns the output
func (ds *DownloadService) createDownloadExec(pod *corev1.Pod, command []string) (string, error) {
	req := ds.kubeClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: captureConstants.DownloadContainername,
			Command:   command,
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(ds.config, "POST", req.URL())
	if err != nil {
		return "", errors.Wrap(err, ErrCreateExecutor.Error())
	}

	var outBuf, errBuf bytes.Buffer
	streamOpts := remotecommand.StreamOptions{
		Stdout: &outBuf,
		Stderr: &errBuf,
	}

	if err = exec.StreamWithContext(ds.ctx, streamOpts); err != nil {
		return "", errors.Wrapf(err, "failed to exec command (stderr: %s)", errBuf.String())
	}

	return outBuf.String(), nil
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
		return errors.Wrap(err, ErrCreateOutputDir.Error())
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
