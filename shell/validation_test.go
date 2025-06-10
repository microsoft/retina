package shell

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

// Extended interface for testing that combines both interfaces
type clientsetInterface interface {
	kubernetes.Interface
}

func TestValidateOperatingSystemSupportedForNode(t *testing.T) {
	tests := []struct {
		name      string
		osLabel   string
		wantError bool
	}{
		{
			name:      "linux node",
			osLabel:   "linux",
			wantError: false,
		},
		{
			name:      "windows node",
			osLabel:   "windows",
			wantError: false,
		},
		{
			name:      "unsupported OS",
			osLabel:   "darwin",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fake client with a node that has the specified OS label
			clientset := fake.NewSimpleClientset(&v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
					Labels: map[string]string{
						"kubernetes.io/os": tt.osLabel,
					},
				},
			})

			// Use the clientset as kubernetes.Interface
			var cs clientsetInterface = clientset

			err := validateOperatingSystemSupportedForNode(context.Background(), cs, "test-node")
			if tt.wantError {
				assert.Error(t, err, "Expected error for OS %s", tt.osLabel)
				assert.Equal(t, errUnsupportedOperatingSystem, err)
			} else {
				assert.NoError(t, err, "Expected no error for OS %s", tt.osLabel)
			}
		})
	}
}

func TestGetNodeOS(t *testing.T) {
	tests := []struct {
		name    string
		osLabel string
		wantOS  string
	}{
		{
			name:    "linux node",
			osLabel: "linux",
			wantOS:  "linux",
		},
		{
			name:    "windows node",
			osLabel: "windows",
			wantOS:  "windows",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fake client with a node that has the specified OS label
			clientset := fake.NewSimpleClientset(&v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
					Labels: map[string]string{
						"kubernetes.io/os": tt.osLabel,
					},
				},
			})

			// Use the clientset as kubernetes.Interface
			var cs clientsetInterface = clientset

			os, err := GetNodeOS(context.Background(), cs, "test-node")
			assert.NoError(t, err)
			assert.Equal(t, tt.wantOS, os)
		})
	}
}
