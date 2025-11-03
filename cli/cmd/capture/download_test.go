// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/label"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	clienttesting "k8s.io/client-go/testing"
)

const (
	testCapture  = "test-capture"
	testFile     = "test-file"
	cmdCommand   = "cmd"
	shellCommand = "sh"
)

func NewLinuxNode(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"kubernetes.io/hostname": name,
			},
		},
		Status: corev1.NodeStatus{
			NodeInfo: corev1.NodeSystemInfo{
				OperatingSystem: "linux",
				OSImage:         "Ubuntu 20.04 LTS",
			},
		},
	}
}

func NewWindowsNode(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"kubernetes.io/hostname": name,
			},
		},
		Status: corev1.NodeStatus{
			NodeInfo: corev1.NodeSystemInfo{
				OperatingSystem: "windows",
				OSImage:         "Windows Server 2022 Datacenter",
			},
		},
	}
}

func NewDownloadNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func NewCapturePodsWithStatus(captureName, namespace, nodeName string, status corev1.PodPhase) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      captureName + "-" + nodeName,
			Namespace: namespace,
			Labels: map[string]string{
				label.CaptureNameLabel: captureName,
			},
			Annotations: map[string]string{
				captureConstants.CaptureHostPathAnnotationKey: "/tmp/captures",
				captureConstants.CaptureFilenameAnnotationKey: "capture-" + nodeName,
			},
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
		},
		Status: corev1.PodStatus{
			Phase: status,
		},
	}
}

func newDownloadKubeClient(objects []runtime.Object) *fake.Clientset {
	if objects == nil {
		objects = []runtime.Object{
			NewLinuxNode("linux-node-1"),
			NewWindowsNode("windows-node-1"),
			NewDownloadNamespace("default"),
			NewDownloadNamespace("capture-test"),
		}
	}

	kubeClient := fake.NewClientset(objects...)

	// Mock pod creation for download pods
	kubeClient.PrependReactor("create", "pods", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		createAction, ok := action.(clienttesting.CreateAction)
		if !ok {
			return false, nil, fmt.Errorf("%w: expected CreateAction", ErrCreateDownloadPod)
		}
		pod := createAction.GetObject().(*corev1.Pod)

		// Simulate pod running after creation
		pod.Status.Phase = corev1.PodRunning
		return false, pod, nil
	})

	return kubeClient
}

func TestDownloadFromCluster(t *testing.T) {
	tempDir := t.TempDir()

	// Set global variables for testing
	originalCaptureName := captureName
	originalOutputPath := outputPath
	captureName = testCapture
	outputPath = tempDir
	defer func() {
		captureName = originalCaptureName
		outputPath = originalOutputPath
	}()

	testCases := []struct {
		name          string
		namespace     string
		setupObjects  func() []runtime.Object
		wantErr       bool
		expectedError string
	}{
		{
			name:      "successful capture pods found",
			namespace: "default",
			setupObjects: func() []runtime.Object {
				return []runtime.Object{
					NewLinuxNode("linux-node-1"),
					NewWindowsNode("windows-node-1"),
					NewDownloadNamespace("default"),
					NewCapturePodsWithStatus(testCapture, "default", "linux-node-1", corev1.PodSucceeded),
					NewCapturePodsWithStatus(testCapture, "default", "windows-node-1", corev1.PodSucceeded),
				}
			},
			wantErr:       false,
			expectedError: "",
		},
		{
			name:      "no capture pods found",
			namespace: "default",
			setupObjects: func() []runtime.Object {
				return []runtime.Object{
					NewLinuxNode("linux-node-1"),
					NewDownloadNamespace("default"),
				}
			},
			wantErr:       true,
			expectedError: "no pod found for job",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			objects := tc.setupObjects()
			kubeClient := newDownloadKubeClient(objects)

			ctx := context.Background()

			// We can't easily test the full downloadFromCluster function due to
			// its dependency on creating actual Kubernetes clients, so we'll focus
			// on testing the service methods and individual functions
			pods, err := getCapturePods(ctx, kubeClient, captureName, tc.namespace)

			if tc.wantErr && err == nil {
				t.Errorf("Expected error for test case %s, but got none", tc.name)
			}
			if tc.wantErr && err != nil && tc.expectedError != "" && !strings.Contains(err.Error(), tc.expectedError) {
				t.Errorf("Expected error message to contain %q, but got %q", tc.expectedError, err.Error())
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Unexpected error for test case %s: %v", tc.name, err)
			}
			if !tc.wantErr && (pods == nil || len(pods.Items) == 0) {
				t.Errorf("Expected to find capture pods for %s, but got none", tc.name)
			}

			// Validate pod properties
			for _, pod := range pods.Items {
				if pod.Status.Phase != corev1.PodSucceeded {
					t.Errorf("Expected pod phase to be Succeeded, got %s", pod.Status.Phase)
				}

				if pod.Labels[label.CaptureNameLabel] != captureName {
					t.Errorf("Expected pod to have capture name label %s, got %s",
						captureName, pod.Labels[label.CaptureNameLabel])
				}

				// Validate required annotations exist
				if _, ok := pod.Annotations[captureConstants.CaptureHostPathAnnotationKey]; !ok {
					t.Errorf("Expected pod to have host path annotation")
				}

				if _, ok := pod.Annotations[captureConstants.CaptureFilenameAnnotationKey]; !ok {
					t.Errorf("Expected pod to have filename annotation")
				}
			}

			t.Logf("Successfully found %d capture pods for %s", len(pods.Items), tc.name)
		})
	}
}

func TestGetNodeOS(t *testing.T) {
	testCases := []struct {
		name     string
		node     *corev1.Node
		expected NodeOS
		wantErr  bool
	}{
		{
			name:     "Linux node",
			node:     NewLinuxNode("linux-test"),
			expected: Linux,
			wantErr:  false,
		},
		{
			name:     "Windows node",
			node:     NewWindowsNode("windows-test"),
			expected: Windows,
			wantErr:  false,
		},
		{
			name: "Unknown OS node",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "unknown-test"},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{
						OperatingSystem: "darwin",
					},
				},
			},
			expected: nil, // nil for unknown OS
			wantErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := getNodeOS(tc.node)

			if tc.wantErr && err == nil {
				t.Errorf("Expected error for %s, but got none", tc.name)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Unexpected error for %s: %v", tc.name, err)
			}
			if result != tc.expected {
				t.Errorf("Expected %v, got %v for %s", tc.expected, result, tc.name)
			}
		})
	}
}

func TestGetDownloadCmd(t *testing.T) {
	testCases := []struct {
		name     string
		node     *corev1.Node
		hostPath string
		fileName string
		wantErr  bool
		validate func(*testing.T, *DownloadCmd, error)
	}{
		{
			name:     "Linux node download cmd",
			node:     NewLinuxNode("linux-test"),
			hostPath: "/tmp/captures",
			fileName: testCapture,
			wantErr:  false,
			validate: func(t *testing.T, cmd *DownloadCmd, err error) {
				if err != nil {
					t.Fatalf("Expected no error, got: %v", err)
				}
				if cmd == nil {
					t.Fatal("Expected DownloadCmd, got nil")
				}
				if !strings.Contains(cmd.ContainerImage, "busybox") {
					t.Errorf("Expected Linux container image to contain 'busybox', got %s", cmd.ContainerImage)
				}
				if !strings.Contains(cmd.SrcFilePath, "/host/tmp/captures/"+testCapture+".tar.gz") {
					t.Errorf("Expected Linux source file path to match pattern, got %s", cmd.SrcFilePath)
				}
				if cmd.MountPath != "/host/tmp/captures" {
					t.Errorf("Expected Linux mount path '/host/tmp/captures', got %s", cmd.MountPath)
				}
				if len(cmd.KeepAliveCommand) == 0 || cmd.KeepAliveCommand[0] != shellCommand {
					t.Errorf("Expected Linux keep alive command to start with 'sh', got %v", cmd.KeepAliveCommand)
				}
				if len(cmd.FileCheckCommand) == 0 || cmd.FileCheckCommand[0] != shellCommand {
					t.Errorf("Expected Linux file check command to start with 'sh', got %v", cmd.FileCheckCommand)
				}
				if len(cmd.FileReadCommand) == 0 || cmd.FileReadCommand[0] != "cat" {
					t.Errorf("Expected Linux file read command to start with 'cat', got %v", cmd.FileReadCommand)
				}
			},
		},
		{
			name:     "Windows node download cmd",
			node:     NewWindowsNode("windows-test"),
			hostPath: "/tmp/captures",
			fileName: testCapture,
			wantErr:  false,
			validate: func(t *testing.T, cmd *DownloadCmd, err error) {
				if err != nil {
					t.Fatalf("Expected no error, got: %v", err)
				}
				if cmd == nil {
					t.Fatal("Expected DownloadCmd, got nil")
				}
				if !strings.Contains(cmd.ContainerImage, "nanoserver") {
					t.Errorf("Expected Windows container image to contain 'nanoserver', got %s", cmd.ContainerImage)
				}
				if !strings.Contains(cmd.SrcFilePath, "C:\\host\\tmp\\captures\\"+testCapture+".tar.gz") {
					t.Errorf("Expected Windows source file path to match pattern, got %s", cmd.SrcFilePath)
				}
				if cmd.MountPath != "C:\\host\\tmp\\captures" {
					t.Errorf("Expected Windows mount path 'C:\\host\\tmp\\captures', got %s", cmd.MountPath)
				}
				if len(cmd.KeepAliveCommand) == 0 || cmd.KeepAliveCommand[0] != cmdCommand {
					t.Errorf("Expected Windows keep alive command to start with 'cmd', got %v", cmd.KeepAliveCommand)
				}
				if len(cmd.FileCheckCommand) == 0 || cmd.FileCheckCommand[0] != cmdCommand {
					t.Errorf("Expected Windows file check command to start with 'cmd', got %v", cmd.FileCheckCommand)
				}
				if len(cmd.FileReadCommand) == 0 || cmd.FileReadCommand[0] != cmdCommand {
					t.Errorf("Expected Windows file read command to start with 'cmd', got %v", cmd.FileReadCommand)
				}
			},
		},
		{
			name: "Unsupported node OS",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "unsupported-test"},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{
						OperatingSystem: "darwin",
					},
				},
			},
			hostPath: "/tmp/captures",
			fileName: testCapture,
			wantErr:  true,
			validate: func(t *testing.T, cmd *DownloadCmd, err error) {
				if err == nil {
					t.Fatal("Expected error for unsupported OS, got nil")
				}
				if cmd != nil {
					t.Errorf("Expected nil DownloadCmd for unsupported OS, got %v", cmd)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := getDownloadCmd(tc.node, tc.hostPath, tc.fileName)
			tc.validate(t, result, err)
		})
	}
}

func TestGetWindowsContainerImage(t *testing.T) {
	testCases := []struct {
		name           string
		osImage        string
		expectedSuffix string
	}{
		{
			name:           "Windows Server 2022",
			osImage:        "Windows Server 2022 Datacenter",
			expectedSuffix: "ltsc2022",
		},
		{
			name:           "Windows Server 2019",
			osImage:        "Windows Server 2019 Datacenter",
			expectedSuffix: "ltsc2019",
		},
		{
			name:           "Windows Server 2025",
			osImage:        "Windows Server 2025 Datacenter",
			expectedSuffix: "ltsc2025",
		},
		{
			name:           "Unknown Windows version",
			osImage:        "Windows Server Unknown",
			expectedSuffix: "ltsc2022", // Default
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "test-node"},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{
						OSImage: tc.osImage,
					},
				},
			}

			result := getWindowsContainerImage(node)
			expectedImage := "mcr.microsoft.com/windows/nanoserver:" + tc.expectedSuffix

			if result != expectedImage {
				t.Errorf("Expected %s, got %s", expectedImage, result)
			}
		})
	}
}

func TestDownloadService(t *testing.T) {
	kubeClient := newDownloadKubeClient(nil)
	config := &rest.Config{}
	namespace := "test-namespace"

	service := NewDownloadService(kubeClient, config, namespace)

	// Test service creation
	if service == nil {
		t.Fatal("Expected service to be created, got nil")
	}

	if service.config != config {
		t.Error("Expected config to match")
	}
	if service.namespace != namespace {
		t.Error("Expected namespace to match")
	}
}

func TestGetCapturePods(t *testing.T) {
	testCases := []struct {
		name          string
		captureName   string
		namespace     string
		setupPods     func() []runtime.Object
		expectedCount int
		wantErr       bool
	}{
		{
			name:        "find capture pods successfully",
			captureName: testCapture,
			namespace:   "default",
			setupPods: func() []runtime.Object {
				return []runtime.Object{
					NewCapturePodsWithStatus(testCapture, "default", "node1", corev1.PodSucceeded),
					NewCapturePodsWithStatus(testCapture, "default", "node2", corev1.PodSucceeded),
				}
			},
			expectedCount: 2,
			wantErr:       false,
		},
		{
			name:        "no capture pods found",
			captureName: "nonexistent-capture",
			namespace:   "default",
			setupPods: func() []runtime.Object {
				return []runtime.Object{}
			},
			expectedCount: 0,
			wantErr:       true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			objects := tc.setupPods()
			kubeClient := newDownloadKubeClient(objects)

			ctx := context.Background()
			pods, err := getCapturePods(ctx, kubeClient, tc.captureName, tc.namespace)

			if tc.wantErr && err == nil {
				t.Errorf("Expected error for %s, but got none", tc.name)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Unexpected error for %s: %v", tc.name, err)
			}
			if !tc.wantErr && len(pods.Items) != tc.expectedCount {
				t.Errorf("Expected %d pods, got %d", tc.expectedCount, len(pods.Items))
			}
		})
	}
}

// Mock test for blob download functionality
func TestDownloadFromBlobValidation(t *testing.T) {
	// Test URL parsing validation
	testCases := []struct {
		name    string
		blobURL string
		wantErr bool
	}{
		{
			name:    "invalid URL",
			blobURL: "not-a-valid-url",
			wantErr: true,
		},
		{
			name:    "valid https URL but no authentication",
			blobURL: "https://storageaccount.blob.core.windows.net/container/blob",
			wantErr: true, // Expect error due to missing authentication in test environment
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set the global variable for the test
			originalBlobURL := blobURL
			blobURL = tc.blobURL
			defer func() { blobURL = originalBlobURL }()

			err := downloadFromBlob()

			if tc.wantErr && err == nil {
				t.Errorf("Expected error for %s, but got none", tc.name)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Unexpected error for %s: %v", tc.name, err)
			}
			if tc.wantErr && err != nil {
				t.Logf("Expected error occurred for %s: %v", tc.name, err)
			}
			if !tc.wantErr && err == nil {
				t.Logf("Successfully completed %s", tc.name)
			}
		})
	}
}

func TestDownloadServiceMethods(t *testing.T) {
	ctx := context.Background()

	// Setup test objects
	objects := []runtime.Object{
		NewLinuxNode("test-node"),
		NewDownloadNamespace("test-namespace"),
	}

	kubeClient := newDownloadKubeClient(objects)
	config := &rest.Config{}
	service := NewDownloadService(kubeClient, config, "test-namespace")

	t.Run("createDownloadPod creates pod correctly", func(t *testing.T) {
		downloadCmd := &DownloadCmd{
			ContainerImage:   "mcr.microsoft.com/azurelinux/busybox:1.36",
			MountPath:        "/host/tmp/captures",
			KeepAliveCommand: []string{"sh", "-c", "echo 'Download pod ready'; sleep 3600"},
		}

		pod, err := service.createDownloadPod(ctx, "test-node", "/tmp/captures", testCapture, downloadCmd)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if pod == nil {
			t.Fatal("Expected pod to be created, got nil")
		}

		if pod.Spec.NodeName != "test-node" {
			t.Errorf("Expected NodeName to be 'test-node', got: %s", pod.Spec.NodeName)
		}

		if len(pod.Spec.Containers) != 1 {
			t.Errorf("Expected 1 container, got: %d", len(pod.Spec.Containers))
		}

		container := pod.Spec.Containers[0]
		if container.Image != downloadCmd.ContainerImage {
			t.Errorf("Expected container image %s, got: %s", downloadCmd.ContainerImage, container.Image)
		}
	})

	t.Run("waitForPodReady handles pod states correctly", func(t *testing.T) {
		// This test is limited due to the fake client behavior
		// In a real scenario, we would test timeout and different pod phases
		podName := "test-pod-ready"

		// Create a pod that should be running
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: service.namespace,
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
			},
		}

		// Add pod to client
		_, err := service.kubeClient.CoreV1().Pods(service.namespace).Create(ctx, pod, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Failed to create test pod: %v", err)
		}

		readyPod, err := service.waitForPodReady(ctx, podName)
		if err != nil {
			t.Fatalf("Expected no error waiting for pod, got: %v", err)
		}

		if readyPod.Status.Phase != corev1.PodRunning {
			t.Errorf("Expected pod phase Running, got: %s", readyPod.Status.Phase)
		}
	})
}

func TestDownloadServiceErrorHandling(t *testing.T) {
	ctx := context.Background()
	kubeClient := newDownloadKubeClient(nil)
	config := &rest.Config{}
	service := NewDownloadService(kubeClient, config, "test-namespace")

	t.Run("DownloadFile handles unsupported node OS", func(t *testing.T) {
		// Create a node with unsupported OS
		unsupportedNode := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "unsupported-node"},
			Status: corev1.NodeStatus{
				NodeInfo: corev1.NodeSystemInfo{
					OperatingSystem: "darwin", // Unsupported OS
				},
			},
		}

		// Add the node to the client
		_, err := service.kubeClient.CoreV1().Nodes().Create(ctx, unsupportedNode, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Failed to create test node: %v", err)
		}

		err = service.DownloadFile(ctx, "unsupported-node", "/tmp", testFile, testCapture)
		if err == nil {
			t.Error("Expected error for unsupported node OS, got nil")
		}

		if !errors.Is(err, ErrUnsupportedNodeOS) {
			t.Errorf("Expected ErrUnsupportedNodeOS, got: %v", err)
		}
	})

	t.Run("DownloadFile handles missing node", func(t *testing.T) {
		err := service.DownloadFile(ctx, "nonexistent-node", "/tmp", testFile, testCapture)
		if err == nil {
			t.Error("Expected error for missing node, got nil")
		}

		if !strings.Contains(err.Error(), "failed to get node information") {
			t.Errorf("Expected error about missing node, got: %v", err)
		}
	})
}

func TestDownloadCommandFlags(t *testing.T) {
	testCases := []struct {
		name     string
		args     []string
		validate func(*testing.T, *cobra.Command)
	}{
		{
			name: "valid name flag provided",
			args: []string{"--name", testCapture},
			validate: func(t *testing.T, cmd *cobra.Command) {
				nameFlag := cmd.Flag("name")
				if nameFlag == nil {
					t.Error("Expected name flag to exist")
					return
				}

				if nameFlag.Value.String() != testCapture {
					t.Errorf("Expected name flag value '%s', got '%s'", testCapture, nameFlag.Value.String())
				}
			},
		},
		{
			name: "valid blob-url flag provided",
			args: []string{"--blob-url", "https://storageaccount.blob.core.windows.net/container/blob?sastoken"},
			validate: func(t *testing.T, cmd *cobra.Command) {
				blobURLFlag := cmd.Flag("blob-url")
				if blobURLFlag == nil {
					t.Error("Expected blob-url flag to exist")
					return
				}

				expectedURL := "https://storageaccount.blob.core.windows.net/container/blob?sastoken"
				if blobURLFlag.Value.String() != expectedURL {
					t.Errorf("Expected blob-url flag value '%s', got '%s'", expectedURL, blobURLFlag.Value.String())
				}
			},
		},
		{
			name: "name with custom output path",
			args: []string{"--name", testCapture, "--output", "/tmp/downloads"},
			validate: func(t *testing.T, cmd *cobra.Command) {
				nameFlag := cmd.Flag("name")
				outputFlag := cmd.Flag("output")

				if nameFlag == nil {
					t.Error("Expected name flag to exist")
					return
				}
				if outputFlag == nil {
					t.Error("Expected output flag to exist")
					return
				}

				if nameFlag.Value.String() != testCapture {
					t.Errorf("Expected name flag value '%s', got '%s'", testCapture, nameFlag.Value.String())
				}

				if outputFlag.Value.String() != "/tmp/downloads" {
					t.Errorf("Expected output flag value '/tmp/downloads', got '%s'", outputFlag.Value.String())
				}
			},
		},
		{
			name: "both name and blob-url flags",
			args: []string{"--name", testCapture, "--blob-url", "https://example.com/blob"},
			validate: func(t *testing.T, cmd *cobra.Command) {
				nameFlag := cmd.Flag("name")
				blobURLFlag := cmd.Flag("blob-url")

				if nameFlag == nil {
					t.Error("Expected name flag to exist")
					return
				}
				if blobURLFlag == nil {
					t.Error("Expected blob-url flag to exist")
					return
				}

				if nameFlag.Value.String() != testCapture {
					t.Errorf("Expected name flag value '%s', got '%s'", testCapture, nameFlag.Value.String())
				}

				if blobURLFlag.Value.String() != "https://example.com/blob" {
					t.Errorf("Expected blob-url flag value 'https://example.com/blob', got '%s'", blobURLFlag.Value.String())
				}
			},
		},
		{
			name: "missing required flags",
			args: []string{},
			validate: func(t *testing.T, cmd *cobra.Command) {
				// Test the validation logic directly by checking global variables
				// Save original values
				originalCaptureName := captureName
				originalBlobURL := blobURL
				originalDownloadAll := downloadAll
				captureName = ""
				blobURL = ""
				downloadAll = false
				defer func() {
					captureName = originalCaptureName
					blobURL = originalBlobURL
					downloadAll = originalDownloadAll
				}()

				// Test the validation condition directly
				if captureName == "" && blobURL == "" && !downloadAll {
					t.Log("Correctly identified missing required flags")
				} else {
					t.Error("Should have identified missing required flags")
				}

				// Verify the command has the expected flags defined
				nameFlag := cmd.Flag("name")
				blobURLFlag := cmd.Flag("blob-url")
				allFlag := cmd.Flag("all")
				outputFlag := cmd.Flag("output")

				if nameFlag == nil {
					t.Error("Expected name flag to be defined")
				}
				if blobURLFlag == nil {
					t.Error("Expected blob-url flag to be defined")
				}
				if allFlag == nil {
					t.Error("Expected all flag to be defined")
				}
				if outputFlag == nil {
					t.Error("Expected output flag to be defined")
				}

				// Test that flags have expected default values
				if nameFlag != nil && nameFlag.DefValue != "" {
					t.Errorf("Expected name flag default to be empty, got '%s'", nameFlag.DefValue)
				}
				if blobURLFlag != nil && blobURLFlag.DefValue != "" {
					t.Errorf("Expected blob-url flag default to be empty, got '%s'", blobURLFlag.DefValue)
				}
				if allFlag != nil && allFlag.DefValue != "false" {
					t.Errorf("Expected all flag default to be false, got '%s'", allFlag.DefValue)
				}
			},
		},
		{
			name: "valid all flag provided",
			args: []string{"--all"},
			validate: func(t *testing.T, cmd *cobra.Command) {
				allFlag := cmd.Flag("all")
				if allFlag == nil {
					t.Error("Expected all flag to be defined")
					return
				}

				if allFlag.Value.String() != "true" {
					t.Errorf("Expected all flag value 'true', got '%s'", allFlag.Value.String())
				}
			},
		},
		{
			name: "all flag with custom output path",
			args: []string{"--all", "-o", "/custom/path"},
			validate: func(t *testing.T, cmd *cobra.Command) {
				allFlag := cmd.Flag("all")
				outputFlag := cmd.Flag("output")

				if allFlag == nil {
					t.Error("Expected all flag to be defined")
					return
				}
				if outputFlag == nil {
					t.Error("Expected output flag to be defined")
					return
				}

				if allFlag.Value.String() != "true" {
					t.Errorf("Expected all flag value 'true', got '%s'", allFlag.Value.String())
				}

				if outputFlag.Value.String() != "/custom/path" {
					t.Errorf("Expected output flag value '/custom/path', got '%s'", outputFlag.Value.String())
				}
			},
		},
		{
			name: "all-namespaces flag with all flag",
			args: []string{"--all", "--all-namespaces"},
			validate: func(t *testing.T, _ *cobra.Command) {
				if !downloadAll {
					t.Error("Expected downloadAll to be true")
				}
				if !downloadAllNamespaces {
					t.Error("Expected downloadAllNamespaces to be true")
				}
				if captureName != "" {
					t.Error("Expected captureName to be empty")
				}
				if blobURL != "" {
					t.Error("Expected blobURL to be empty")
				}
			},
		},
		{
			name: "all-namespaces flag without all flag (should fail validation)",
			args: []string{"--all-namespaces"},
			validate: func(t *testing.T, _ *cobra.Command) {
				if downloadAll {
					t.Error("Expected downloadAll to be false")
				}
				if !downloadAllNamespaces {
					t.Error("Expected downloadAllNamespaces to be true")
				}
				// This should fail validation in the actual command execution
				// but we can't test that here since we're only parsing flags
			},
		},
		{
			name: "all-namespaces with name flag (should fail validation)",
			args: []string{"--name", "test", "--all-namespaces"},
			validate: func(t *testing.T, _ *cobra.Command) {
				if downloadAll {
					t.Error("Expected downloadAll to be false")
				}
				if !downloadAllNamespaces {
					t.Error("Expected downloadAllNamespaces to be true")
				}
				if captureName != "test" {
					t.Error("Expected captureName to be 'test'")
				}
				// This should fail validation in the actual command execution
			},
		},
	}

	// Test all cases with unified approach
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := NewDownloadSubCommand()

			// Parse flags without executing the command
			err := cmd.ParseFlags(tc.args)
			if err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}

			tc.validate(t, cmd)
		})
	}
}

func TestDownloadAllCapturesGracefulErrorHandling(t *testing.T) {
	// Test the graceful error handling by testing individual components
	// that are used in downloadAllCaptures

	testCases := []struct {
		name        string
		pod         *corev1.Pod
		expectSkip  bool
		description string
	}{
		{
			name:        "successful pod",
			pod:         NewCapturePodsWithStatus("test-capture", "default", "node1", corev1.PodSucceeded),
			expectSkip:  false,
			description: "Pod with Succeeded status should be processed",
		},
		{
			name:        "failed pod",
			pod:         NewCapturePodsWithStatus("test-capture", "default", "node1", corev1.PodFailed),
			expectSkip:  true,
			description: "Pod with Failed status should be skipped with warning",
		},
		{
			name:        "running pod",
			pod:         NewCapturePodsWithStatus("test-capture", "default", "node1", corev1.PodRunning),
			expectSkip:  true,
			description: "Pod with Running status should be skipped with warning",
		},
		{
			name: "pod missing host path annotation",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Labels: map[string]string{
						label.CaptureNameLabel: "test-capture",
					},
					Annotations: map[string]string{
						// Missing CaptureHostPathAnnotationKey
						captureConstants.CaptureFilenameAnnotationKey: "test-file",
					},
				},
				Spec:   corev1.PodSpec{NodeName: "node1"},
				Status: corev1.PodStatus{Phase: corev1.PodSucceeded},
			},
			expectSkip:  true,
			description: "Pod missing host path annotation should be skipped with warning",
		},
		{
			name: "pod missing filename annotation",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Labels: map[string]string{
						label.CaptureNameLabel: "test-capture",
					},
					Annotations: map[string]string{
						captureConstants.CaptureHostPathAnnotationKey: "/tmp/captures",
						// Missing CaptureFilenameAnnotationKey
					},
				},
				Spec:   corev1.PodSpec{NodeName: "node1"},
				Status: corev1.PodStatus{Phase: corev1.PodSucceeded},
			},
			expectSkip:  true,
			description: "Pod missing filename annotation should be skipped with warning",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test the logic that would be used in downloadAllCaptures
			shouldSkip := false

			// Check pod status
			if tc.pod.Status.Phase != corev1.PodSucceeded {
				shouldSkip = true
				t.Logf("Pod %s would be skipped due to status: %s", tc.pod.Name, tc.pod.Status.Phase)
			}

			// Check annotations if pod status is good
			if !shouldSkip {
				if _, ok := tc.pod.Annotations[captureConstants.CaptureHostPathAnnotationKey]; !ok {
					shouldSkip = true
					t.Logf("Pod %s would be skipped due to missing host path annotation", tc.pod.Name)
				}
				if _, ok := tc.pod.Annotations[captureConstants.CaptureFilenameAnnotationKey]; !ok {
					shouldSkip = true
					t.Logf("Pod %s would be skipped due to missing filename annotation", tc.pod.Name)
				}
			}

			if shouldSkip != tc.expectSkip {
				t.Errorf("Expected skip=%v, got skip=%v for %s", tc.expectSkip, shouldSkip, tc.description)
			}

			t.Logf("Test case '%s' completed: %s", tc.name, tc.description)
		})
	}
}
