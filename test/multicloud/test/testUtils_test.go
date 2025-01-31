package test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestBuildClientSet(t *testing.T) {
	config := &rest.Config{}
	clientset, err := buildClientSet(config)
	if err != nil {
		t.Fatalf("Failed to build clientset: %v", err)
	}
	if clientset == nil {
		t.Fatalf("Expected clientset to be non-nil")
	}
}

func TestCreateRESTConfigWithBearer(t *testing.T) {
	config := createRESTConfigWithBearer("caCert", "bearerToken", "host")
	if config.BearerToken != "bearerToken" {
		t.Fatalf("Expected BearerToken to be 'bearerToken'")
	}
}

func TestCreateRESTConfigWithClientCert(t *testing.T) {
	config := createRESTConfigWithClientCert("caCert", "clientCert", "clientKey", "host")
	if config.TLSClientConfig.CAData == nil || string(config.TLSClientConfig.CAData) != "caCert" {
		t.Fatalf("Expected CAData to be 'caCert'")
	}
	if config.TLSClientConfig.CertData == nil || string(config.TLSClientConfig.CertData) != "clientCert" {
		t.Fatalf("Expected CertData to be 'clientCert'")
	}
	if config.TLSClientConfig.KeyData == nil || string(config.TLSClientConfig.KeyData) != "clientKey" {
		t.Fatalf("Expected KeyData to be 'clientKey'")
	}
	if config.Host != "host" {
		t.Fatalf("Expected Host to be 'host'")
	}
}

func TestTestClusterAccess(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	testClusterAccess(t, clientset)
}

func TestCheckLogsForErrors(t *testing.T) {
	testCases := []struct {
		name     string
		logData  string
		expected bool
	}{
		{
			name:     "Log contains error",
			logData:  "error: something went wrong",
			expected: false,
		},
		{
			name:     "Log contains level=ERR",
			logData:  "level=ERR: something went wrong",
			expected: true,
		},
		{
			name:     "Log contains level=error",
			logData:  "level=error: something went wrong",
			expected: true,
		},
		{
			name:     "Log does not contain error",
			logData:  "info: all systems operational",
			expected: false,
		},
		{
			name:     "Empty log",
			logData:  "",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logs := io.NopCloser(bytes.NewReader([]byte(tc.logData)))
			err := checkLogsForErrors(logs)
			if (err != nil) != tc.expected {
				t.Fatalf("Expected error: %v, got: %v", tc.expected, err != nil)
			}
		})
	}
}

func TestCheckPodLogs(t *testing.T) {
	clientset := fake.NewSimpleClientset(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
		},
	})
	podSpec := PodSpec{
		Namespace:     "default",
		LabelSelector: "app=test",
		ContainerName: "test-container",
	}
	checkPodLogs(t, clientset, podSpec)
}

func TestDecodeBase64(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("test"))
	decoded := decodeBase64(t, encoded)
	if decoded != "test" {
		t.Fatalf("Expected 'test', got '%s'", decoded)
	}
}

func TestFetchSensitiveOutput(t *testing.T) {
	// Create a temporary directory for the mock state file
	tempDir, err := os.MkdirTemp("", "terraform")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a mock state file with the sensitive output
	stateFilePath := tempDir + "/terraform.tfstate"
	mockState := map[string]interface{}{
		"version": 4,
		"outputs": map[string]interface{}{
			"test-output": map[string]interface{}{
				"value":     "sensitive-value",
				"type":      "string",
				"sensitive": true,
			},
		},
	}
	stateData, err := json.Marshal(mockState)
	if err != nil {
		t.Fatalf("Failed to marshal mock state: %v", err)
	}
	if err := os.WriteFile(stateFilePath, stateData, 0644); err != nil {
		t.Fatalf("Failed to write mock state file: %v", err)
	}

	// Set up terraform options to use the mock state file
	opts := &terraform.Options{
		TerraformDir: tempDir,
	}

	// Fetch the sensitive output
	output := fetchSensitiveOutput(t, opts, "test-output")
	if output != "sensitive-value" {
		t.Fatalf("Expected 'sensitive-value', got '%s'", output)
	}
}

func TestCheckPodsRunning(t *testing.T) {
	testCases := []struct {
		name     string
		pods     []v1.Pod
		expected bool
	}{
		{
			name: "All pods running",
			pods: []v1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod-1",
						Namespace: "default",
						Labels: map[string]string{
							"app": "test",
						},
					},
					Status: v1.PodStatus{
						Phase: v1.PodRunning,
						ContainerStatuses: []v1.ContainerStatus{
							{
								Ready: true,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod-2",
						Namespace: "default",
						Labels: map[string]string{
							"app": "test",
						},
					},
					Status: v1.PodStatus{
						Phase: v1.PodRunning,
						ContainerStatuses: []v1.ContainerStatus{
							{
								Ready: true,
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Some pods not running",
			pods: []v1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod-1",
						Namespace: "default",
						Labels: map[string]string{
							"app": "test",
						},
					},
					Status: v1.PodStatus{
						Phase: v1.PodRunning,
						ContainerStatuses: []v1.ContainerStatus{
							{
								Ready: true,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod-2",
						Namespace: "default",
						Labels: map[string]string{
							"app": "test",
						},
					},
					Status: v1.PodStatus{
						Phase: v1.PodPending,
					},
				},
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var objects []runtime.Object
			for _, pod := range tc.pods {
				objects = append(objects, pod.DeepCopyObject())
			}
			clientset := fake.NewSimpleClientset(objects...)
			podSpec := PodSpec{
				Namespace:     "default",
				LabelSelector: "app=test",
			}
			result, err := arePodsRunning(clientset, podSpec, 1)
			if result != tc.expected {
				t.Fatalf("Expected %v, got %v with error: %v", tc.expected, result, err)
			}
		})
	}
}
