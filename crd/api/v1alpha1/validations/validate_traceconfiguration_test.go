/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package validations

import (
	"testing"

	"github.com/microsoft/retina/crd/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestTraceConfiguration tests the validation of trace configuration
func TestTraceConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		trace   *v1alpha1.TracesConfiguration
		wantErr bool
	}{
		{
			name:    "nil trace configuration",
			trace:   nil,
			wantErr: true,
		},
		{
			name:    "empty trace configuration",
			trace:   &v1alpha1.TracesConfiguration{},
			wantErr: true,
		},
		{
			name: "nil trace spec configuration",
			trace: &v1alpha1.TracesConfiguration{
				Spec: nil,
			},
			wantErr: true,
		},
		{
			name: "empty trace spec configuration",
			trace: &v1alpha1.TracesConfiguration{
				Spec: &v1alpha1.TracesSpec{},
			},
			wantErr: true,
		},
		{
			name: "empty trace targets",
			trace: &v1alpha1.TracesConfiguration{
				Spec: &v1alpha1.TracesSpec{
					TraceConfiguration: []*v1alpha1.TraceConfiguration{
						{
							TraceCaptureLevel: v1alpha1.AllPacketsCapture,
							TraceTargets:      []*v1alpha1.TraceTargets{},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid trace capture level",
			trace: &v1alpha1.TracesConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "traceconfig",
				},
				Spec: &v1alpha1.TracesSpec{
					TraceConfiguration: []*v1alpha1.TraceConfiguration{
						{
							TraceCaptureLevel: "invalid",
							TraceTargets: []*v1alpha1.TraceTargets{
								{
									Source: &v1alpha1.TraceTarget{
										IPBlock: v1alpha1.IPBlock{
											CIDR: "10.0.0.0/8",
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid trace configuration name",
			trace: &v1alpha1.TracesConfiguration{
				Spec: &v1alpha1.TracesSpec{
					TraceConfiguration: []*v1alpha1.TraceConfiguration{
						{
							TraceCaptureLevel: v1alpha1.AllPacketsCapture,
							TraceTargets: []*v1alpha1.TraceTargets{
								{
									Source: &v1alpha1.TraceTarget{
										IPBlock: v1alpha1.IPBlock{
											CIDR: "10.0.0.0/8",
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid trace targets",
			trace: &v1alpha1.TracesConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "traceconfig",
				},
				Spec: &v1alpha1.TracesSpec{
					TraceConfiguration: []*v1alpha1.TraceConfiguration{
						{
							TraceCaptureLevel: v1alpha1.AllPacketsCapture,
							TraceTargets: []*v1alpha1.TraceTargets{
								{},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid trace configuration",
			trace: &v1alpha1.TracesConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "traceconfig",
				},
				Spec: &v1alpha1.TracesSpec{
					TraceConfiguration: []*v1alpha1.TraceConfiguration{
						{
							TraceCaptureLevel: v1alpha1.AllPacketsCapture,
							TraceTargets: []*v1alpha1.TraceTargets{
								{
									Source: &v1alpha1.TraceTarget{
										IPBlock: v1alpha1.IPBlock{
											CIDR: "10.0.0.0/8",
										},
									},
								},
							},
						},
					},
					TraceOutputConfiguration: &v1alpha1.TraceOutputConfiguration{
						TraceOutputDestination: "Test",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid trace configuration with pod selector",
			trace: &v1alpha1.TracesConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "traceconfig",
				},
				Spec: &v1alpha1.TracesSpec{
					TraceConfiguration: []*v1alpha1.TraceConfiguration{
						{
							TraceCaptureLevel: v1alpha1.AllPacketsCapture,
							TraceTargets: []*v1alpha1.TraceTargets{
								{
									Source: &v1alpha1.TraceTarget{
										PodSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{
												"app": "nginx",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid trace configuration with ns and pod selector",
			trace: &v1alpha1.TracesConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "traceconfig",
				},
				Spec: &v1alpha1.TracesSpec{
					TraceConfiguration: []*v1alpha1.TraceConfiguration{
						{
							TraceCaptureLevel: v1alpha1.AllPacketsCapture,
							TraceTargets: []*v1alpha1.TraceTargets{
								{
									Source: &v1alpha1.TraceTarget{
										PodSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{
												"app": "nginx",
											},
										},
										NamespaceSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{
												"ns": "nginx",
											},
										},
									},
								},
							},
						},
					},
					TraceOutputConfiguration: &v1alpha1.TraceOutputConfiguration{
						TraceOutputDestination: "Test",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid trace configuration with ns and node selector",
			trace: &v1alpha1.TracesConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "traceconfig",
				},
				Spec: &v1alpha1.TracesSpec{
					TraceConfiguration: []*v1alpha1.TraceConfiguration{
						{
							TraceCaptureLevel: v1alpha1.AllPacketsCapture,
							TraceTargets: []*v1alpha1.TraceTargets{
								{
									Source: &v1alpha1.TraceTarget{
										NodeSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{
												"app": "nginx",
											},
										},
										NamespaceSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{
												"ns": "nginx",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid trace configuration with ns , pod and node selector",
			trace: &v1alpha1.TracesConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "traceconfig",
				},
				Spec: &v1alpha1.TracesSpec{
					TraceConfiguration: []*v1alpha1.TraceConfiguration{
						{
							TraceCaptureLevel: v1alpha1.AllPacketsCapture,
							TraceTargets: []*v1alpha1.TraceTargets{
								{
									Source: &v1alpha1.TraceTarget{
										NodeSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{
												"app": "nginx",
											},
										},
										PodSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{
												"ns": "nginx",
											},
										},
										NamespaceSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{
												"ns": "nginx",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid trace configuration with ns, pod and service selector",
			trace: &v1alpha1.TracesConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "traceconfig",
				},
				Spec: &v1alpha1.TracesSpec{
					TraceConfiguration: []*v1alpha1.TraceConfiguration{
						{
							TraceCaptureLevel: v1alpha1.AllPacketsCapture,
							TraceTargets: []*v1alpha1.TraceTargets{
								{
									Source: &v1alpha1.TraceTarget{
										ServiceSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{
												"app": "nginx",
											},
										},
										PodSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{
												"ns": "nginx",
											},
										},
										NamespaceSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{
												"ns": "nginx",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid trace configuration with node and service selector",
			trace: &v1alpha1.TracesConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "traceconfig",
				},
				Spec: &v1alpha1.TracesSpec{
					TraceConfiguration: []*v1alpha1.TraceConfiguration{
						{
							TraceCaptureLevel: v1alpha1.AllPacketsCapture,
							TraceTargets: []*v1alpha1.TraceTargets{
								{
									Source: &v1alpha1.TraceTarget{
										ServiceSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{
												"app": "nginx",
											},
										},
										NodeSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{
												"ns": "nginx",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid trace configuration with service selector and port",
			trace: &v1alpha1.TracesConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "traceconfig",
				},
				Spec: &v1alpha1.TracesSpec{
					TraceConfiguration: []*v1alpha1.TraceConfiguration{
						{
							TraceCaptureLevel: v1alpha1.AllPacketsCapture,
							TraceTargets: []*v1alpha1.TraceTargets{
								{
									Source: &v1alpha1.TraceTarget{
										ServiceSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{
												"app": "nginx",
											},
										},
									},
									Ports: []*v1alpha1.TracePorts{
										{
											Port: "80",
										},
									},
									TracePoints: v1alpha1.TracePoints{
										v1alpha1.NetworkToNode,
										v1alpha1.NodeToPod,
									},
								},
							},
							IncludeLayer7Data: true,
						},
					},
					TraceOutputConfiguration: &v1alpha1.TraceOutputConfiguration{
						TraceOutputDestination: "Test",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := TracesCRD(tt.trace); (err != nil) != tt.wantErr {
				t.Errorf("TraceConfiguration() name = %s error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestTraceTarget(t *testing.T) {
	tests := []struct {
		name    string
		target  *v1alpha1.TraceTarget
		wantErr bool
	}{
		{
			name: "valid trace target",
			target: &v1alpha1.TraceTarget{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"ns": "nginx",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid trace target with ns and service selector",
			target: &v1alpha1.TraceTarget{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"ns": "nginx",
					},
				},
				ServiceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"ns": "nginx",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid trace target with ns and pod selector",
			target: &v1alpha1.TraceTarget{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"ns": "nginx",
					},
				},
				PodSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"ns": "nginx",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid trace target with ns and node selector",
			target: &v1alpha1.TraceTarget{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"ns": "nginx",
					},
				},
				NodeSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"ns": "nginx",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid trace target with pod and service selector",
			target: &v1alpha1.TraceTarget{
				PodSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"ns": "nginx",
					},
				},
				ServiceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"ns": "nginx",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid trace target with pod and node selector",
			target: &v1alpha1.TraceTarget{
				PodSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"ns": "nginx",
					},
				},
				NodeSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"ns": "nginx",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid trace target with service and node selector",
			target: &v1alpha1.TraceTarget{
				ServiceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"ns": "nginx",
					},
				},
				NodeSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"ns": "nginx",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid trace target with pod and service selector",
			target: &v1alpha1.TraceTarget{
				PodSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"ns": "nginx",
					},
				},
				ServiceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"ns": "nginx",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid trace target with ipblock and namespace selector",
			target: &v1alpha1.TraceTarget{
				IPBlock: v1alpha1.IPBlock{
					CIDR: "0.0.0.0",
				},
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"ns": "nginx",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid trace target with ipblock and pod selector",
			target: &v1alpha1.TraceTarget{
				IPBlock: v1alpha1.IPBlock{
					CIDR: "0.0.0.0",
				},
				PodSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"ns": "nginx",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid trace target with ipblock and service selector",
			target: &v1alpha1.TraceTarget{
				IPBlock: v1alpha1.IPBlock{
					CIDR: "0.0.0.0",
				},
				ServiceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"ns": "nginx",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid trace target with ipblock and node selector",
			target: &v1alpha1.TraceTarget{
				IPBlock: v1alpha1.IPBlock{
					CIDR: "0.0.0.0",
				},
				NodeSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"ns": "nginx",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid trace target with ipblock",
			target: &v1alpha1.TraceTarget{
				IPBlock: v1alpha1.IPBlock{
					Except: []string{"0.0.0.0"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := TraceTarget(tt.target); (err != nil) != tt.wantErr {
				t.Errorf("TraceConfiguration() name = %s error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestTracePorts(t *testing.T) {
	tests := []struct {
		name    string
		trace   *v1alpha1.TracePorts
		wantErr bool
	}{
		{
			name: "valid trace ports",
			trace: &v1alpha1.TracePorts{
				Port:     "80",
				Protocol: "TCP",
			},
			wantErr: false,
		},
		{
			name: "udp valid trace ports",
			trace: &v1alpha1.TracePorts{
				Port:     "80",
				Protocol: "UDP",
			},
			wantErr: false,
		},
		{
			name: "invalid trace ports",
			trace: &v1alpha1.TracePorts{
				Port:     "80",
				Protocol: "UDP",
				EndPort:  "79",
			},
			wantErr: true,
		},
		{
			name: "invalid trace ports 2",
			trace: &v1alpha1.TracePorts{
				Port:     "-1",
				Protocol: "UDP",
				EndPort:  "79",
			},
			wantErr: true,
		},
		{
			name: "valid trace ports 2",
			trace: &v1alpha1.TracePorts{
				Port:     "80",
				Protocol: "UDP",
				EndPort:  "89",
			},
			wantErr: false,
		},
		{
			name: "invalid trace ports 3",
			trace: &v1alpha1.TracePorts{
				Port: "aaa",
			},
			wantErr: true,
		},
		{
			name: "invalid trace ports 4",
			trace: &v1alpha1.TracePorts{
				Port:     "80",
				Protocol: "aaa",
				EndPort:  "4444444444",
			},
			wantErr: true,
		},
		{
			name: "invalid trace ports 5",
			trace: &v1alpha1.TracePorts{
				Port:     "80",
				Protocol: "aaa",
				EndPort:  "aa",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := TracePort(tt.trace); (err != nil) != tt.wantErr {
				t.Errorf("TracePorts() name = %s error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}
