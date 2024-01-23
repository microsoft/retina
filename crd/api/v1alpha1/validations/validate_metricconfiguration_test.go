/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package validations

import (
	"testing"

	"github.com/microsoft/retina/crd/api/v1alpha1"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestMetricsConfiguration tests the metrics configuration validation
func TestMetricsConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		obj     *v1alpha1.MetricsConfiguration
		wantErr bool
	}{
		{
			name: "invalid test 1",
			obj: &v1alpha1.MetricsConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "metricsconfig",
				},
				Spec: v1alpha1.MetricsSpec{},
			},
			wantErr: true,
		},
		{
			name: "Test 1",
			obj: &v1alpha1.MetricsConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "metricsconfig",
				},
				Spec: v1alpha1.MetricsSpec{},
			},
			wantErr: true,
		},
		{
			name: "valid metrics crd",
			obj: &v1alpha1.MetricsConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "metricsconfig",
				},
				Spec: v1alpha1.MetricsSpec{
					ContextOptions: []v1alpha1.MetricsContextOptions{
						{
							MetricName: "drop_count",
						},
					},
					Namespaces: v1alpha1.MetricsNamespaces{
						Include: []string{"default"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid metrics crd",
			obj: &v1alpha1.MetricsConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "metricsconfig",
				},
				Spec: v1alpha1.MetricsSpec{
					ContextOptions: []v1alpha1.MetricsContextOptions{
						{
							MetricName: "drop_count",
						},
					},
					Namespaces: v1alpha1.MetricsNamespaces{
						Include: []string{"default"},
						Exclude: []string{"kube-system"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid metrics crd with exclude",
			obj: &v1alpha1.MetricsConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "metricsconfig",
				},
				Spec: v1alpha1.MetricsSpec{
					ContextOptions: []v1alpha1.MetricsContextOptions{
						{
							MetricName: "drop_count",
						},
					},
					Namespaces: v1alpha1.MetricsNamespaces{
						Exclude: []string{"kube-system"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid metrics crd with random metric name",
			obj: &v1alpha1.MetricsConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "metricsconfig",
				},
				Spec: v1alpha1.MetricsSpec{
					ContextOptions: []v1alpha1.MetricsContextOptions{
						{
							MetricName: "test-wrong-metric",
						},
					},
					Namespaces: v1alpha1.MetricsNamespaces{
						Exclude: []string{"kube-system"},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MetricsCRD(tt.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("MetricsConfiguration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

// TestMetricsSpec tests the metrics spec validation
func TestCompare(t *testing.T) {
	tests := []struct {
		name  string
		old   *v1alpha1.MetricsConfiguration
		new   *v1alpha1.MetricsConfiguration
		equal bool
	}{
		{
			name: "invalid test 1",
			old: &v1alpha1.MetricsConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "metricsconfig",
				},
				Spec: v1alpha1.MetricsSpec{},
			},
			new:   &v1alpha1.MetricsConfiguration{},
			equal: false,
		},
		{
			name: "valid test 1",
			old: &v1alpha1.MetricsConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "metricsconfig",
				},
				Spec: v1alpha1.MetricsSpec{},
			},
			new: &v1alpha1.MetricsConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "metricsconfig",
				},
				Spec: v1alpha1.MetricsSpec{},
			},
			equal: true,
		},
		{
			name: "valid test 2",
			old: &v1alpha1.MetricsConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "metricsconfig",
				},
				Spec: v1alpha1.MetricsSpec{
					ContextOptions: []v1alpha1.MetricsContextOptions{
						{
							MetricName: "drop_count",
						},
					},
					Namespaces: v1alpha1.MetricsNamespaces{
						Include: []string{"default"},
					},
				},
			},
			new: &v1alpha1.MetricsConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "metricsconfig",
				},
				Spec: v1alpha1.MetricsSpec{
					ContextOptions: []v1alpha1.MetricsContextOptions{
						{
							MetricName: "drop_count",
						},
					},
					Namespaces: v1alpha1.MetricsNamespaces{
						Include: []string{"default"},
					},
				},
			},
			equal: true,
		},
		{
			name: "invalied test 3",
			old: &v1alpha1.MetricsConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "metricsconfig",
				},
				Spec: v1alpha1.MetricsSpec{
					ContextOptions: []v1alpha1.MetricsContextOptions{
						{
							MetricName:   "drop_count",
							SourceLabels: []string{"ip", "port"},
						},
					},
					Namespaces: v1alpha1.MetricsNamespaces{
						Include: []string{"default"},
					},
				},
			},
			new: &v1alpha1.MetricsConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "metricsconfig",
				},
				Spec: v1alpha1.MetricsSpec{
					ContextOptions: []v1alpha1.MetricsContextOptions{
						{
							MetricName: "drop_count",
						},
					},
					Namespaces: v1alpha1.MetricsNamespaces{
						Include: []string{"default"},
					},
				},
			},
			equal: false,
		},
		{
			name: "valid test 3",
			old: &v1alpha1.MetricsConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "metricsconfig",
				},
				Spec: v1alpha1.MetricsSpec{
					ContextOptions: []v1alpha1.MetricsContextOptions{
						{
							MetricName:   "drop_count",
							SourceLabels: []string{"ip", "port"},
						},
					},
					Namespaces: v1alpha1.MetricsNamespaces{
						Include: []string{"default", "test"},
						Exclude: []string{"kube-system"},
					},
				},
			},
			new: &v1alpha1.MetricsConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "metricsconfig",
				},
				Spec: v1alpha1.MetricsSpec{
					ContextOptions: []v1alpha1.MetricsContextOptions{
						{
							MetricName:   "drop_count",
							SourceLabels: []string{"ip", "port"},
						},
					},
					Namespaces: v1alpha1.MetricsNamespaces{
						Include: []string{"default"},
						Exclude: []string{"kube-system"},
					},
				},
			},
			equal: false,
		},
		{
			name: "valid test 3",
			old: &v1alpha1.MetricsConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "metricsconfig",
				},
				Spec: v1alpha1.MetricsSpec{
					ContextOptions: []v1alpha1.MetricsContextOptions{
						{
							MetricName:   "drop_count",
							SourceLabels: []string{"ip", "port"},
						},
					},
					Namespaces: v1alpha1.MetricsNamespaces{
						Include: []string{"default", "test"},
						Exclude: []string{"kube-system"},
					},
				},
			},
			new: &v1alpha1.MetricsConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "metricsconfig",
				},
				Spec: v1alpha1.MetricsSpec{
					ContextOptions: []v1alpha1.MetricsContextOptions{
						{
							MetricName:   "drop_count",
							SourceLabels: []string{"ip", "port"},
						},
					},
					Namespaces: v1alpha1.MetricsNamespaces{
						Include: []string{"default", "test"},
						Exclude: []string{"kube-system"},
					},
				},
			},
			equal: true,
		},
		{
			name: "valid test 5",
			old: &v1alpha1.MetricsConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "metricsconfig",
				},
				Spec: v1alpha1.MetricsSpec{
					ContextOptions: []v1alpha1.MetricsContextOptions{
						{
							MetricName:   "drop_count",
							SourceLabels: []string{"ns", "ip", "port"},
						},
					},
					Namespaces: v1alpha1.MetricsNamespaces{
						Include: []string{"default", "test"},
						Exclude: []string{"kube-system"},
					},
				},
			},
			new: &v1alpha1.MetricsConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "metricsconfig",
				},
				Spec: v1alpha1.MetricsSpec{
					ContextOptions: []v1alpha1.MetricsContextOptions{
						{
							MetricName:   "drop_count",
							SourceLabels: []string{"ip", "port", "ns"},
						},
					},
					Namespaces: v1alpha1.MetricsNamespaces{
						Include: []string{"default", "test"},
						Exclude: []string{"kube-system"},
					},
				},
			},
			equal: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eq := CompareMetricsConfig(tt.old, tt.new)
			assert.Equal(t, tt.equal, eq, "Compare() error = %v, wantErr %v", eq, tt.equal)
		})
	}
}
