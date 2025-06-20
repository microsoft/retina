// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package capture

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	pointerUtil "k8s.io/utils/pointer"

	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/capture/file"
	captureUtils "github.com/microsoft/retina/pkg/capture/utils"
	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/label"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/telemetry"
)

const (
	retinaAgentImageForTest = "ghcr.io/microsoft/retina/retina-agent:v0.0.1-pre"
	apiServerURL            = "https://retina-test-c4528d-zn0ugsna.hcp.southeastasia.azmk8s.io:443"
)

func NewCaptureToPodTranslatorForTest(kubeClient kubernetes.Interface) *CaptureToPodTranslator {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	config := config.CaptureConfig{
		CaptureDebug:              true,
		CaptureImageVersion:       "v0.0.1-pre",
		CaptureImageVersionSource: captureUtils.VersionSourceOperatorImageVersion,
		CaptureJobNumLimit:        10,
	}

	captureToPodTranslator := NewCaptureToPodTranslator(kubeClient, log.Logger().Named("test"), config)
	captureToPodTranslator.Apiserver = apiServerURL
	return captureToPodTranslator
}

func Test_CaptureTargetsOnNode_AddPod(t *testing.T) {
	cases := []struct {
		existingTargets          map[string]CaptureTarget
		newTargetNode            string
		newTargetPodIPs          []string
		wantCaptureTargetsOnNode CaptureTargetsOnNode
	}{
		{
			existingTargets: map[string]CaptureTarget{},
			newTargetNode:   "node1",
			newTargetPodIPs: []string{"10.225.0.4"},
			wantCaptureTargetsOnNode: CaptureTargetsOnNode{
				"node1": {PodIpAddresses: []string{"10.225.0.4"}},
			},
		},
		{
			existingTargets: map[string]CaptureTarget{
				"node1": {PodIpAddresses: []string{"10.225.0.4"}},
			},
			newTargetNode:   "node1",
			newTargetPodIPs: []string{"10.225.0.5"},
			wantCaptureTargetsOnNode: CaptureTargetsOnNode{
				"node1": {PodIpAddresses: []string{"10.225.0.4", "10.225.0.5"}},
			},
		},
		{
			existingTargets: map[string]CaptureTarget{
				"node1": {PodIpAddresses: []string{"10.225.0.4"}},
			},
			newTargetNode:   "node2",
			newTargetPodIPs: []string{"10.225.0.5"},
			wantCaptureTargetsOnNode: CaptureTargetsOnNode{
				"node1": {PodIpAddresses: []string{"10.225.0.4"}},
				"node2": {PodIpAddresses: []string{"10.225.0.5"}},
			},
		},
	}
	for _, c := range cases {
		gotCaptureTargetsOnNode := CaptureTargetsOnNode{}
		for k, v := range c.existingTargets {
			gotCaptureTargetsOnNode[k] = v
		}
		gotCaptureTargetsOnNode.AddPod(c.newTargetNode, c.newTargetPodIPs)
		if diff := cmp.Diff(c.wantCaptureTargetsOnNode, gotCaptureTargetsOnNode); diff != "" {
			t.Errorf("AddPod() mismatch (-want, +got):\n%s", diff)
		}
	}
}

func Test_CaptureTargetsOnNode_AddNodeInterface(t *testing.T) {
	cases := []struct {
		existingTargets          map[string]CaptureTarget
		newTargetNode            string
		wantCaptureTargetsOnNode CaptureTargetsOnNode
	}{
		{
			existingTargets: map[string]CaptureTarget{},
			newTargetNode:   "node1",
			wantCaptureTargetsOnNode: CaptureTargetsOnNode{
				"node1": {CaptureNodeInterface: true},
			},
		},
		{
			existingTargets: map[string]CaptureTarget{"node1": {CaptureNodeInterface: true}},
			newTargetNode:   "node1",
			wantCaptureTargetsOnNode: CaptureTargetsOnNode{
				"node1": {CaptureNodeInterface: true},
			},
		},
	}
	for _, c := range cases {
		gotCaptureTargetsOnNode := CaptureTargetsOnNode{}
		for k, v := range c.existingTargets {
			gotCaptureTargetsOnNode[k] = v
		}
		gotCaptureTargetsOnNode.AddNodeInterface(c.newTargetNode)

		if diff := cmp.Diff(c.wantCaptureTargetsOnNode, gotCaptureTargetsOnNode); diff != "" {
			t.Errorf("AddPod() mismatch (-want, +got):\n%s", diff)
		}
	}
}

func Test_CaptureToPodTranslator_GetCaptureTargetsOnNode(t *testing.T) {
	ctx, cancel := TestContext(t)
	defer cancel()

	cases := []struct {
		name                     string
		captureTarget            retinav1alpha1.CaptureTarget
		nodeList                 *corev1.NodeList
		namespaceList            *corev1.NamespaceList
		podList                  *corev1.PodList
		wantCaptureTargetsOnNode *CaptureTargetsOnNode
		wantErr                  bool
	}{
		{
			name:                     "No selector is specified",
			captureTarget:            retinav1alpha1.CaptureTarget{},
			wantCaptureTargetsOnNode: nil,
			wantErr:                  true,
		},
		{
			name: "either node or pod selector is specified",
			captureTarget: retinav1alpha1.CaptureTarget{
				NodeSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{corev1.LabelHostname: "node1"},
				},
				PodSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"type": "server"},
				},
			},
			wantCaptureTargetsOnNode: nil,
			wantErr:                  true,
		},
		{
			name: "Get target through node selector when MatchExpressions is set",
			captureTarget: retinav1alpha1.CaptureTarget{
				NodeSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{{
						Key:      corev1.LabelHostname,
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"node1"},
					}},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{corev1.LabelHostname: "node1"}}}},
			},
			wantCaptureTargetsOnNode: &CaptureTargetsOnNode{
				"node1": CaptureTarget{
					CaptureNodeInterface: true,
				},
			},
			wantErr: false,
		},
		{
			name: "Get target through node selector when MatchLabels is set",
			captureTarget: retinav1alpha1.CaptureTarget{
				NodeSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{corev1.LabelOSStable: "linux"},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{corev1.LabelOSStable: "linux"}}}},
			},
			wantCaptureTargetsOnNode: &CaptureTargetsOnNode{
				"node1": CaptureTarget{
					CaptureNodeInterface: true,
				},
			},
			wantErr: false,
		},
		{
			name: "Get target through node selector when MatchLabels and MatchExpressions are both set and met",
			captureTarget: retinav1alpha1.CaptureTarget{
				NodeSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{corev1.LabelOSStable: "linux"},
					MatchExpressions: []metav1.LabelSelectorRequirement{{
						Key:      corev1.LabelHostname,
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"node1"},
					}},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{corev1.LabelOSStable: "linux", corev1.LabelHostname: "node1"}}}},
			},
			wantCaptureTargetsOnNode: &CaptureTargetsOnNode{
				"node1": CaptureTarget{
					CaptureNodeInterface: true,
				},
			},
			wantErr: false,
		},
		{
			name: "Get target through node selector and raise error when MatchLabels and MatchExpressions are both set but not met",
			captureTarget: retinav1alpha1.CaptureTarget{
				NodeSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{corev1.LabelOSStable: "linux"},
					MatchExpressions: []metav1.LabelSelectorRequirement{{
						Key:      corev1.LabelHostname,
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"node1"},
					}},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{corev1.LabelOSStable: "windows", corev1.LabelHostname: "node1"}}}},
			},
			wantCaptureTargetsOnNode: nil,
			wantErr:                  true,
		},
		{
			name: "Get target through pod selector",
			captureTarget: retinav1alpha1.CaptureTarget{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"name": "test-capture-ns"},
				},
				PodSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"type": "test-capture-pod"},
				},
			},
			namespaceList: &corev1.NamespaceList{
				Items: []corev1.Namespace{{ObjectMeta: metav1.ObjectMeta{Name: "test-capture-ns", Labels: map[string]string{"name": "test-capture-ns"}}}},
			},
			podList: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "test-capture-pod", Namespace: "test-capture-ns", Labels: map[string]string{"type": "test-capture-pod"}},
						Spec:       corev1.PodSpec{NodeName: "node1"},
						Status: corev1.PodStatus{
							PodIP:  "10.225.0.4",
							PodIPs: []corev1.PodIP{{IP: "10.225.0.4"}},
						},
					},
				},
			},
			wantCaptureTargetsOnNode: &CaptureTargetsOnNode{
				"node1": CaptureTarget{
					PodIpAddresses: []string{"10.225.0.4"},
				},
			},
			wantErr: false,
		},
		{
			name: "Get target through pod selector for dual-stack cluster",
			captureTarget: retinav1alpha1.CaptureTarget{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"name": "test-capture-ns"},
				},
				PodSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"type": "test-capture-pod"},
				},
			},
			namespaceList: &corev1.NamespaceList{
				Items: []corev1.Namespace{{ObjectMeta: metav1.ObjectMeta{Name: "test-capture-ns", Labels: map[string]string{"name": "test-capture-ns"}}}},
			},
			podList: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "test-capture-pod", Namespace: "test-capture-ns", Labels: map[string]string{"type": "test-capture-pod"}},
						Spec:       corev1.PodSpec{NodeName: "node1"},
						Status: corev1.PodStatus{
							PodIP: "10.225.0.4",
							PodIPs: []corev1.PodIP{
								{IP: "10.225.0.4"},
								{IP: "fd5c:d9f1:79c5:fd83::21e"},
							},
						},
					},
				},
			},
			wantCaptureTargetsOnNode: &CaptureTargetsOnNode{
				"node1": CaptureTarget{
					PodIpAddresses: []string{"10.225.0.4", "fd5c:d9f1:79c5:fd83::21e"},
				},
			},
			wantErr: false,
		},
		{
			name: "Get target through pod selector when multiple pods in one node",
			captureTarget: retinav1alpha1.CaptureTarget{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"name": "test-capture-ns"},
				},
				PodSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"type": "test-capture-pod"},
				},
			},
			namespaceList: &corev1.NamespaceList{
				Items: []corev1.Namespace{{ObjectMeta: metav1.ObjectMeta{Name: "test-capture-ns", Labels: map[string]string{"name": "test-capture-ns"}}}},
			},
			podList: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "test-capture-pod1", Namespace: "test-capture-ns", Labels: map[string]string{"type": "test-capture-pod"}},
						Spec:       corev1.PodSpec{NodeName: "node1"},
						Status: corev1.PodStatus{
							PodIP:  "10.225.0.4",
							PodIPs: []corev1.PodIP{{IP: "10.225.0.4"}},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "test-capture-pod2", Namespace: "test-capture-ns", Labels: map[string]string{"type": "test-capture-pod"}},
						Spec:       corev1.PodSpec{NodeName: "node1"},
						Status: corev1.PodStatus{
							PodIP:  "10.225.0.5",
							PodIPs: []corev1.PodIP{{IP: "10.225.0.5"}},
						},
					},
				},
			},
			wantCaptureTargetsOnNode: &CaptureTargetsOnNode{
				"node1": CaptureTarget{
					PodIpAddresses: []string{"10.225.0.4", "10.225.0.5"},
				},
			},
			wantErr: false,
		},
		{
			name: "Get target through pod selector and raise error when no targets are selected",
			captureTarget: retinav1alpha1.CaptureTarget{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"name": "test-capture-ns"},
				},
				PodSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"type": "test-capture-pod"},
				},
			},
			namespaceList: &corev1.NamespaceList{
				Items: []corev1.Namespace{{ObjectMeta: metav1.ObjectMeta{Name: "test-capture-ns", Labels: map[string]string{"name": "test-capture-ns"}}}},
			},
			podList: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "test-capture-pod", Namespace: "test-capture-ns-different", Labels: map[string]string{"type": "test-capture-pod"}},
						Spec:       corev1.PodSpec{NodeName: "node1"},
						Status: corev1.PodStatus{
							PodIP:  "10.225.0.4",
							PodIPs: []corev1.PodIP{{IP: "10.225.0.4"}},
						},
					},
				},
			},
			wantCaptureTargetsOnNode: nil,
			wantErr:                  true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			objects := []runtime.Object{}
			if tt.nodeList != nil {
				objects = append(objects, tt.nodeList)
			}
			if tt.namespaceList != nil {
				objects = append(objects, tt.namespaceList)
			}
			if tt.podList != nil {
				objects = append(objects, tt.podList)
			}

			k8sClient := fakeclientset.NewSimpleClientset(objects...)
			captureToPodTranslator := NewCaptureToPodTranslatorForTest(k8sClient)
			gotCaptureTargetsOnNode, err := captureToPodTranslator.getCaptureTargetsOnNode(ctx, tt.captureTarget)
			if tt.wantErr != (err != nil) {
				t.Errorf("getCaptureTargetsOnNode() want(%t) error, got error %s", tt.wantErr, err)
			}
			if tt.wantErr {
				return
			}
			if diff := cmp.Diff(tt.wantCaptureTargetsOnNode, gotCaptureTargetsOnNode); diff != "" {
				t.Errorf("getCaptureTargetsOnNode() mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func Test_CaptureToPodTranslator_updateCaptureTargetsOSOnNode(t *testing.T) {
	ctx, cancel := TestContext(t)
	defer cancel()

	cases := []struct {
		name                     string
		captureTargetsOnNode     *CaptureTargetsOnNode
		nodeList                 *corev1.NodeList
		wantCaptureTargetsOnNode *CaptureTargetsOnNode
		wantErr                  bool
	}{
		{
			name: "update target node OS",
			captureTargetsOnNode: &CaptureTargetsOnNode{
				"node1": CaptureTarget{
					PodIpAddresses: []string{"10.225.0.4", "10.225.0.5"},
				},
				"node2": CaptureTarget{
					PodIpAddresses: []string{"10.225.1.4", "10.225.1.5"},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{corev1.LabelOSStable: "windows", corev1.LabelHostname: "node1"}},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node2", Labels: map[string]string{corev1.LabelOSStable: "linux", corev1.LabelHostname: "node1"}},
					},
				},
			},
			wantCaptureTargetsOnNode: &CaptureTargetsOnNode{
				"node1": CaptureTarget{
					PodIpAddresses: []string{"10.225.0.4", "10.225.0.5"},
					OS:             "windows",
				},
				"node2": CaptureTarget{
					PodIpAddresses: []string{"10.225.1.4", "10.225.1.5"},
					OS:             "linux",
				},
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			objects := []runtime.Object{}
			if tt.nodeList != nil {
				objects = append(objects, tt.nodeList)
			}

			k8sClient := fakeclientset.NewSimpleClientset(objects...)
			captureToPodTranslator := NewCaptureToPodTranslatorForTest(k8sClient)
			err := captureToPodTranslator.updateCaptureTargetsOSOnNode(ctx, tt.captureTargetsOnNode)
			if tt.wantErr != (err != nil) {
				t.Errorf("updateCaptureTargetsOSOnNode() want(%t) error, got error %s", tt.wantErr, err)
			}
			if tt.wantErr {
				return
			}
			if diff := cmp.Diff(tt.wantCaptureTargetsOnNode, tt.captureTargetsOnNode); diff != "" {
				t.Errorf("updateCaptureTargetsOSOnNode() mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func Test_CaptureToPodTranslator_ObtainCaptureJobPodEnv(t *testing.T) {
	var packetSize int = 96
	cases := []struct {
		name       string
		capture    retinav1alpha1.Capture
		wantJobEnv map[string]string
		wantErr    bool
	}{
		{
			name:    "output configuration is empty",
			capture: retinav1alpha1.Capture{},
			wantErr: true,
		},
		{
			name: "use hostpath",
			capture: retinav1alpha1.Capture{
				Spec: retinav1alpha1.CaptureSpec{
					OutputConfiguration: retinav1alpha1.OutputConfiguration{
						HostPath: pointerUtil.String("/tmp/capture"),
					},
				},
			},
			wantJobEnv: map[string]string{
				string(captureConstants.CaptureOutputLocationEnvKeyHostPath): "/tmp/capture",
				captureConstants.IncludeMetadataEnvKey:                       "false",
			},
		},
		{
			name: "use pvc",
			capture: retinav1alpha1.Capture{
				Spec: retinav1alpha1.CaptureSpec{
					OutputConfiguration: retinav1alpha1.OutputConfiguration{
						PersistentVolumeClaim: pointerUtil.String("capture-pvc"),
					},
				},
			},
			wantJobEnv: map[string]string{
				string(captureConstants.CaptureOutputLocationEnvKeyPersistentVolumeClaim): "capture-pvc",
				captureConstants.IncludeMetadataEnvKey:                                    "false",
			},
		},
		{
			name: "use blob",
			capture: retinav1alpha1.Capture{
				Spec: retinav1alpha1.CaptureSpec{
					OutputConfiguration: retinav1alpha1.OutputConfiguration{
						BlobUpload: pointerUtil.String("sasURL"),
					},
				},
			},
			wantJobEnv: map[string]string{
				captureConstants.IncludeMetadataEnvKey: "false",
			},
		},
		{
			name: "include metadata",
			capture: retinav1alpha1.Capture{
				Spec: retinav1alpha1.CaptureSpec{
					OutputConfiguration: retinav1alpha1.OutputConfiguration{
						PersistentVolumeClaim: pointerUtil.String("capture-pvc"),
					},
					CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
						IncludeMetadata: true,
					},
				},
			},
			wantJobEnv: map[string]string{
				captureConstants.IncludeMetadataEnvKey:                                    "true",
				string(captureConstants.CaptureOutputLocationEnvKeyPersistentVolumeClaim): "capture-pvc",
			},
		},
		{
			name: "packet size",
			capture: retinav1alpha1.Capture{
				Spec: retinav1alpha1.CaptureSpec{
					OutputConfiguration: retinav1alpha1.OutputConfiguration{
						PersistentVolumeClaim: pointerUtil.String("capture-pvc"),
					},
					CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
						IncludeMetadata: true,
						CaptureOption: retinav1alpha1.CaptureOption{
							PacketSize: &packetSize,
						},
					},
				},
			},
			wantJobEnv: map[string]string{
				captureConstants.IncludeMetadataEnvKey:                                    "true",
				string(captureConstants.CaptureOutputLocationEnvKeyPersistentVolumeClaim): "capture-pvc",
				captureConstants.PacketSizeEnvKey:                                         strconv.Itoa(packetSize),
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			k8sClient := fakeclientset.NewSimpleClientset()
			log.SetupZapLogger(log.GetDefaultLogOpts())
			captureToPodTranslator := NewCaptureToPodTranslatorForTest(k8sClient)
			jobEnv, err := captureToPodTranslator.ObtainCaptureJobPodEnv(tt.capture)

			if tt.wantErr != (err != nil) {
				t.Errorf("ObtainCaptureJobPodEnv() want(%t) error, got error %s", tt.wantErr, err)
			}

			if diff := cmp.Diff(tt.wantJobEnv, jobEnv); diff != "" {
				t.Errorf("ObtainCaptureJobPodEnv() mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func Test_CaptureToPodTranslator_RenderJob_NodeSelected(t *testing.T) {
	ctx, cancel := TestContext(t)
	defer cancel()

	cases := []struct {
		name                string
		captureTargetOnNode *CaptureTargetsOnNode
		env                 map[string]string
		wantAffinities      []*corev1.Affinity
		wantErr             bool
	}{
		{
			name:                "no nodes are selected",
			captureTargetOnNode: &CaptureTargetsOnNode{},
			env:                 map[string]string{},
			wantErr:             true,
		},
		{
			name:                "single node is selected",
			captureTargetOnNode: &CaptureTargetsOnNode{"node1": {}},
			env:                 map[string]string{},
			wantAffinities: []*corev1.Affinity{
				{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      corev1.LabelHostname,
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"node1"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:                "multiple nodes are selected",
			captureTargetOnNode: &CaptureTargetsOnNode{"node1": {}, "node2": {}},
			env:                 map[string]string{},
			wantAffinities: []*corev1.Affinity{
				{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      corev1.LabelHostname,
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"node1"},
										},
									},
								},
							},
						},
					},
				},
				{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      corev1.LabelHostname,
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"node2"},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			k8sClient := fakeclientset.NewSimpleClientset()
			log.SetupZapLogger(log.GetDefaultLogOpts())
			captureToPodTranslator := NewCaptureToPodTranslatorForTest(k8sClient)

			startTime := time.Now()

			hostPath := "/tmp/capture" // nolint:goconst // Test case needs a var

			err := captureToPodTranslator.initJobTemplate(ctx, &retinav1alpha1.Capture{
				Spec: retinav1alpha1.CaptureSpec{
					CaptureConfiguration: retinav1alpha1.CaptureConfiguration{},
					OutputConfiguration: retinav1alpha1.OutputConfiguration{
						HostPath: &hostPath,
					},
				},
				Status: retinav1alpha1.CaptureStatus{
					StartTime: &metav1.Time{Time: startTime},
				},
			})
			if err != nil {
				t.Errorf("initJobTemplate() want no error, got error %s", err)
			}
			jobs, err := captureToPodTranslator.renderJob(tt.captureTargetOnNode, tt.env)

			if tt.wantErr != (err != nil) {
				t.Errorf("renderJob() want(%t) error, got error %s", tt.wantErr, err)
			}
			var affinities []*corev1.Affinity
			for _, job := range jobs {
				affinities = append(affinities, job.Spec.Template.Spec.Affinity)
			}

			cmpOption := cmpopts.SortSlices(
				func(affinity1, affinity2 *corev1.Affinity) bool {
					nodeName1 := affinity1.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Values[0]
					nodeName2 := affinity2.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Values[0]
					return nodeName1 < nodeName2
				},
			)

			if diff := cmp.Diff(tt.wantAffinities, affinities, cmpOption); diff != "" {
				t.Errorf("renderJob() mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func Test_CaptureToPodTranslator_ValidateCapture(t *testing.T) {
	captureName := "capture-test"
	hostPath := "/tmp/capture"
	nodeName := "node-name"
	cases := []struct {
		name    string
		capture retinav1alpha1.Capture
		wantErr bool
	}{
		{
			name: "raise error when target is not specified",
			capture: retinav1alpha1.Capture{
				ObjectMeta: metav1.ObjectMeta{
					Name: captureName,
				},
				Spec: retinav1alpha1.CaptureSpec{
					CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
						CaptureOption: retinav1alpha1.CaptureOption{
							Duration: &metav1.Duration{Duration: 10 * time.Second},
						},
					},
					OutputConfiguration: retinav1alpha1.OutputConfiguration{
						HostPath: &hostPath,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "raise error when duration and maxcaptureSize is not specified",
			capture: retinav1alpha1.Capture{
				ObjectMeta: metav1.ObjectMeta{
					Name: captureName,
				},
				Spec: retinav1alpha1.CaptureSpec{
					CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
						CaptureTarget: retinav1alpha1.CaptureTarget{
							NodeSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"nodename": nodeName,
								},
							},
						},
					},
					OutputConfiguration: retinav1alpha1.OutputConfiguration{
						HostPath: &hostPath,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "raise error when output configuration is not specified",
			capture: retinav1alpha1.Capture{
				ObjectMeta: metav1.ObjectMeta{
					Name: captureName,
				},
				Spec: retinav1alpha1.CaptureSpec{
					CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
						CaptureTarget: retinav1alpha1.CaptureTarget{
							NodeSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"nodename": nodeName,
								},
							},
						},
						CaptureOption: retinav1alpha1.CaptureOption{
							Duration: &metav1.Duration{Duration: 10 * time.Second},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "validation is ok when all are set",
			capture: retinav1alpha1.Capture{
				ObjectMeta: metav1.ObjectMeta{
					Name: captureName,
				},
				Spec: retinav1alpha1.CaptureSpec{
					CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
						CaptureTarget: retinav1alpha1.CaptureTarget{
							NodeSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"nodename": nodeName,
								},
							},
						},
						CaptureOption: retinav1alpha1.CaptureOption{
							Duration: &metav1.Duration{Duration: 10 * time.Second},
						},
					},
					OutputConfiguration: retinav1alpha1.OutputConfiguration{
						HostPath: &hostPath,
					},
				},
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			k8sClient := fakeclientset.NewSimpleClientset()
			captureToPodTranslator := NewCaptureToPodTranslatorForTest(k8sClient)
			err := captureToPodTranslator.validateCapture(&tt.capture)
			if tt.wantErr != (err != nil) {
				t.Errorf("validateCapture() want(%t) error, got error %s", tt.wantErr, err)
			}
			if tt.wantErr {
				return
			}
		})
	}
}

func isIgnorableEnvVar(envVar corev1.EnvVar) bool {
	return envVar.Name == captureConstants.CaptureStartTimestampEnvKey
}

func Test_CaptureToPodTranslator_TranslateCaptureToJobs(t *testing.T) {
	ctx, cancel := TestContext(t)
	defer cancel()

	captureName := "capture-test"
	hostPath := "/tmp/capture"
	timestamp := file.Now()
	pvc := "capture-pvc"
	backoffLimit := int32(0)
	rootUser := int64(0)
	tcpdumpFilter := "-i eth0"
	captureFolderHostPathType := corev1.HostPathDirectoryOrCreate
	commonJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", captureName),
			Labels: map[string]string{
				label.CaptureNameLabel: captureName,
				label.AppLabel:         captureConstants.CaptureContainername,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						label.CaptureNameLabel: captureName,
						label.AppLabel:         captureConstants.CaptureContainername,
					},
					Annotations: map[string]string{
						captureConstants.CaptureFilenameAnnotationKey: (&file.CaptureFilename{
							CaptureName:    captureName,
							NodeHostname:   "node1",
							StartTimestamp: timestamp,
						}).String(),
						captureConstants.CaptureTimestampAnnotationKey: file.TimeToString(timestamp),
					},
				},
				Spec: corev1.PodSpec{
					HostNetwork:                   true,
					HostIPC:                       true,
					TerminationGracePeriodSeconds: pointerUtil.Int64(1800),
					Containers: []corev1.Container{
						{
							Name:            captureConstants.CaptureContainername,
							Image:           retinaAgentImageForTest,
							ImagePullPolicy: corev1.PullIfNotPresent,
							SecurityContext: &corev1.SecurityContext{
								RunAsUser: &rootUser,
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{
										"NET_ADMIN", "SYS_ADMIN",
									},
								},
							},
							Command: []string{captureConstants.CaptureContainerEntrypoint},
							Env: []corev1.EnvVar{
								{
									Name: telemetry.EnvPodName,
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name:  captureConstants.ApiserverEnvKey,
									Value: apiServerURL,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("300Mi"),
								},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      corev1.LabelHostname,
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"node1"},
											},
										},
									},
								},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Key:      "CriticalAddonsOnly",
							Operator: "Exists",
						},
						{
							Effect:   "NoExecute",
							Operator: "Exists",
						},
						{
							Effect:   "NoSchedule",
							Operator: "Exists",
						},
					},
				},
			},
		},
	}
	cases := []struct {
		name         string
		capture      retinav1alpha1.Capture
		podList      *corev1.PodList
		nodeList     *corev1.NodeList
		secret       *corev1.Secret
		volumeMounts []corev1.VolumeMount
		volumes      []corev1.Volume
		podEnv       []v1.EnvVar
		existingPVC  *corev1.PersistentVolumeClaim
		isWindows    bool
		wantErr      bool
	}{
		{
			name: "configurations: hostpath as output configurations",
			capture: retinav1alpha1.Capture{
				ObjectMeta: metav1.ObjectMeta{
					Name: captureName,
				},
				Status: retinav1alpha1.CaptureStatus{
					StartTime: timestamp,
				},
				Spec: retinav1alpha1.CaptureSpec{
					CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
						CaptureTarget: retinav1alpha1.CaptureTarget{
							NodeSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{{
									Key:      corev1.LabelHostname,
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{"node1"},
								}},
							},
						},
						CaptureOption: retinav1alpha1.CaptureOption{
							Duration: &metav1.Duration{Duration: 1 * time.Minute},
						},
					},
					OutputConfiguration: retinav1alpha1.OutputConfiguration{
						HostPath: &hostPath,
					},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{corev1.LabelHostname: "node1", corev1.LabelOSStable: "linux"}}}},
			},
			volumeMounts: []corev1.VolumeMount{
				{
					Name:      captureConstants.CaptureHostPathVolumeName,
					MountPath: hostPath,
				},
			},
			volumes: []corev1.Volume{
				{
					Name: captureConstants.CaptureHostPathVolumeName,
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: hostPath,
							Type: &captureFolderHostPathType,
						},
					},
				},
			},
			podEnv: []v1.EnvVar{
				{Name: captureConstants.CaptureDurationEnvKey, Value: "1m0s"},
				{Name: string(captureConstants.CaptureOutputLocationEnvKeyHostPath), Value: hostPath},
				{Name: captureConstants.CaptureNameEnvKey, Value: captureName},
				{Name: captureConstants.CaptureStartTimestampEnvKey, Value: file.TimeToString(timestamp)},
				{Name: captureConstants.IncludeMetadataEnvKey, Value: "false"},
				{Name: captureConstants.NodeHostNameEnvKey, Value: "node1"},
				{
					Name: telemetry.EnvPodName,
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.name",
						},
					},
				},
				{
					Name:  captureConstants.ApiserverEnvKey,
					Value: apiServerURL,
				},
			},
		},
		{
			name: "configurations: secret is not found",
			capture: retinav1alpha1.Capture{
				ObjectMeta: metav1.ObjectMeta{
					Name: captureName,
				},
				Spec: retinav1alpha1.CaptureSpec{
					CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
						CaptureTarget: retinav1alpha1.CaptureTarget{
							NodeSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{{
									Key:      corev1.LabelHostname,
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{"node1"},
								}},
							},
						},
						CaptureOption: retinav1alpha1.CaptureOption{
							Duration: &metav1.Duration{Duration: 1 * time.Second},
						},
					},
					OutputConfiguration: retinav1alpha1.OutputConfiguration{
						BlobUpload: pointerUtil.String("secretName"),
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "secretName",
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{corev1.LabelHostname: "node1", corev1.LabelOSStable: "linux"}}}},
			},
			wantErr: true,
		},
		{
			name: "configurations: pvc is not found",
			capture: retinav1alpha1.Capture{
				ObjectMeta: metav1.ObjectMeta{
					Name: captureName,
				},
				Spec: retinav1alpha1.CaptureSpec{
					CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
						CaptureTarget: retinav1alpha1.CaptureTarget{
							NodeSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{{
									Key:      corev1.LabelHostname,
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{"node1"},
								}},
							},
						},
						CaptureOption: retinav1alpha1.CaptureOption{
							Duration: &metav1.Duration{Duration: 1 * time.Second},
						},
					},
					OutputConfiguration: retinav1alpha1.OutputConfiguration{
						PersistentVolumeClaim: &pvc,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "configurations: pvc as output configurations",
			capture: retinav1alpha1.Capture{
				ObjectMeta: metav1.ObjectMeta{
					Name: captureName,
				},
				Status: retinav1alpha1.CaptureStatus{
					StartTime: timestamp,
				},
				Spec: retinav1alpha1.CaptureSpec{
					CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
						CaptureTarget: retinav1alpha1.CaptureTarget{
							NodeSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{{
									Key:      corev1.LabelHostname,
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{"node1"},
								}},
							},
						},
						CaptureOption: retinav1alpha1.CaptureOption{
							Duration: &metav1.Duration{Duration: 1 * time.Minute},
						},
					},
					OutputConfiguration: retinav1alpha1.OutputConfiguration{
						PersistentVolumeClaim: &pvc,
					},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{corev1.LabelHostname: "node1", corev1.LabelOSStable: "linux"}}}},
			},
			volumeMounts: []corev1.VolumeMount{
				{
					Name:      captureConstants.CapturePVCVolumeName,
					MountPath: captureConstants.PersistentVolumeClaimVolumeMountPathLinux,
				},
			},
			volumes: []corev1.Volume{
				{
					Name: captureConstants.CapturePVCVolumeName,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvc,
						},
					},
				},
			},
			podEnv: []v1.EnvVar{
				{Name: captureConstants.CaptureDurationEnvKey, Value: "1m0s"},
				{Name: captureConstants.CaptureNameEnvKey, Value: captureName},
				{Name: captureConstants.CaptureStartTimestampEnvKey, Value: file.TimeToString(timestamp)},
				{Name: captureConstants.IncludeMetadataEnvKey, Value: "false"},
				{Name: string(captureConstants.CaptureOutputLocationEnvKeyPersistentVolumeClaim), Value: pvc},
				{Name: captureConstants.NodeHostNameEnvKey, Value: "node1"},
				{
					Name: telemetry.EnvPodName,
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.name",
						},
					},
				},
				{
					Name:  captureConstants.ApiserverEnvKey,
					Value: apiServerURL,
				},
			},
			existingPVC: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: pvc,
				},
			},
		},
		{
			name:      "configurations: pvc as output configurations for Windows",
			isWindows: true,
			capture: retinav1alpha1.Capture{
				ObjectMeta: metav1.ObjectMeta{
					Name: captureName,
				},
				Status: retinav1alpha1.CaptureStatus{
					StartTime: timestamp,
				},
				Spec: retinav1alpha1.CaptureSpec{
					CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
						CaptureTarget: retinav1alpha1.CaptureTarget{
							NodeSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{{
									Key:      corev1.LabelHostname,
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{"node1"},
								}},
							},
						},
						CaptureOption: retinav1alpha1.CaptureOption{
							Duration: &metav1.Duration{Duration: 1 * time.Minute},
						},
					},
					OutputConfiguration: retinav1alpha1.OutputConfiguration{
						PersistentVolumeClaim: &pvc,
					},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{corev1.LabelHostname: "node1", corev1.LabelOSStable: "windows"}}}},
			},
			volumeMounts: []corev1.VolumeMount{
				{
					Name:      captureConstants.CapturePVCVolumeName,
					MountPath: captureConstants.PersistentVolumeClaimVolumeMountPathWin,
				},
			},
			volumes: []corev1.Volume{
				{
					Name: captureConstants.CapturePVCVolumeName,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvc,
						},
					},
				},
			},
			podEnv: []v1.EnvVar{
				{Name: captureConstants.CaptureDurationEnvKey, Value: "1m0s"},
				{Name: captureConstants.CaptureNameEnvKey, Value: captureName},
				{Name: captureConstants.CaptureStartTimestampEnvKey, Value: file.TimeToString(timestamp)},
				{Name: captureConstants.IncludeMetadataEnvKey, Value: "false"},
				{Name: string(captureConstants.CaptureOutputLocationEnvKeyPersistentVolumeClaim), Value: pvc},
				{Name: captureConstants.NodeHostNameEnvKey, Value: "node1"},
				{
					Name: telemetry.EnvPodName,
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.name",
						},
					},
				},
				{
					Name:  captureConstants.ApiserverEnvKey,
					Value: apiServerURL,
				},
			},
			existingPVC: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: pvc,
				},
			},
		},
		{
			name: "configurations: pvc and hostpath as output configurations",
			capture: retinav1alpha1.Capture{
				ObjectMeta: metav1.ObjectMeta{
					Name: captureName,
				},
				Status: retinav1alpha1.CaptureStatus{
					StartTime: timestamp,
				},
				Spec: retinav1alpha1.CaptureSpec{
					CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
						CaptureTarget: retinav1alpha1.CaptureTarget{
							NodeSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{{
									Key:      corev1.LabelHostname,
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{"node1"},
								}},
							},
						},
						CaptureOption: retinav1alpha1.CaptureOption{
							Duration: &metav1.Duration{Duration: 1 * time.Minute},
						},
					},
					OutputConfiguration: retinav1alpha1.OutputConfiguration{
						HostPath:              &hostPath,
						PersistentVolumeClaim: &pvc,
					},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{corev1.LabelHostname: "node1", corev1.LabelOSStable: "linux"}}}},
			},
			volumeMounts: []corev1.VolumeMount{
				{
					Name:      captureConstants.CaptureHostPathVolumeName,
					MountPath: hostPath,
				},
				{
					Name:      captureConstants.CapturePVCVolumeName,
					MountPath: captureConstants.PersistentVolumeClaimVolumeMountPathLinux,
				},
			},
			volumes: []corev1.Volume{
				{
					Name: captureConstants.CaptureHostPathVolumeName,
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: hostPath,
							Type: &captureFolderHostPathType,
						},
					},
				},
				{
					Name: captureConstants.CapturePVCVolumeName,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvc,
						},
					},
				},
			},
			podEnv: []v1.EnvVar{
				{Name: captureConstants.CaptureDurationEnvKey, Value: "1m0s"},
				{Name: string(captureConstants.CaptureOutputLocationEnvKeyHostPath), Value: hostPath},
				{Name: captureConstants.CaptureNameEnvKey, Value: captureName},
				{Name: captureConstants.CaptureStartTimestampEnvKey, Value: file.TimeToString(timestamp)},
				{Name: captureConstants.IncludeMetadataEnvKey, Value: "false"},
				{Name: string(captureConstants.CaptureOutputLocationEnvKeyPersistentVolumeClaim), Value: pvc},
				{Name: captureConstants.NodeHostNameEnvKey, Value: "node1"},
				{
					Name: telemetry.EnvPodName,
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.name",
						},
					},
				},
				{
					Name:  captureConstants.ApiserverEnvKey,
					Value: apiServerURL,
				},
			},
			existingPVC: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: pvc,
				},
			},
		},
		{
			name: "tcpdumpfilter: pod ip address and tcpdumpfilter coexist",
			capture: retinav1alpha1.Capture{
				ObjectMeta: metav1.ObjectMeta{
					Name: captureName,
				},
				Status: retinav1alpha1.CaptureStatus{
					StartTime: timestamp,
				},
				Spec: retinav1alpha1.CaptureSpec{
					CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
						CaptureTarget: retinav1alpha1.CaptureTarget{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"type": "test-capture-pod"},
							},
						},
						TcpdumpFilter: &tcpdumpFilter,
						CaptureOption: retinav1alpha1.CaptureOption{
							Duration: &metav1.Duration{Duration: 1 * time.Minute},
						},
					},
					OutputConfiguration: retinav1alpha1.OutputConfiguration{
						HostPath: &hostPath,
					},
				},
			},
			podList: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "test-capture-pod", Namespace: "default", Labels: map[string]string{"type": "test-capture-pod"}},
						Spec:       corev1.PodSpec{NodeName: "node1"},
						Status: corev1.PodStatus{
							PodIP:  "10.225.0.4",
							PodIPs: []corev1.PodIP{{IP: "10.225.0.4"}},
						},
					},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{corev1.LabelHostname: "node1", corev1.LabelOSStable: "linux"}}}},
			},
			volumeMounts: []corev1.VolumeMount{
				{
					Name:      captureConstants.CaptureHostPathVolumeName,
					MountPath: hostPath,
				},
			},
			volumes: []corev1.Volume{
				{
					Name: captureConstants.CaptureHostPathVolumeName,
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: hostPath,
							Type: &captureFolderHostPathType,
						},
					},
				},
			},
			podEnv: []v1.EnvVar{
				{Name: captureConstants.CaptureDurationEnvKey, Value: "1m0s"},
				{Name: string(captureConstants.CaptureOutputLocationEnvKeyHostPath), Value: hostPath},
				{Name: captureConstants.CaptureNameEnvKey, Value: captureName},
				{Name: captureConstants.CaptureStartTimestampEnvKey, Value: file.TimeToString(timestamp)},
				{Name: captureConstants.IncludeMetadataEnvKey, Value: "false"},
				{Name: captureConstants.NodeHostNameEnvKey, Value: "node1"},
				{Name: captureConstants.TcpdumpRawFilterEnvKey, Value: "-i eth0"},
				{Name: captureConstants.TcpdumpFilterEnvKey, Value: "(host 10.225.0.4)"},
				{
					Name: telemetry.EnvPodName,
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.name",
						},
					},
				},
				{
					Name:  captureConstants.ApiserverEnvKey,
					Value: apiServerURL,
				},
			},
		},
		{
			name: "tcpdumpfilter: pod ip adddress exits",
			capture: retinav1alpha1.Capture{
				ObjectMeta: metav1.ObjectMeta{
					Name: captureName,
				},
				Status: retinav1alpha1.CaptureStatus{
					StartTime: timestamp,
				},
				Spec: retinav1alpha1.CaptureSpec{
					CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
						CaptureTarget: retinav1alpha1.CaptureTarget{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"type": "test-capture-pod"},
							},
						},
						CaptureOption: retinav1alpha1.CaptureOption{
							Duration: &metav1.Duration{Duration: 1 * time.Minute},
						},
					},
					OutputConfiguration: retinav1alpha1.OutputConfiguration{
						HostPath: &hostPath,
					},
				},
			},
			podList: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "test-capture-pod", Namespace: "default", Labels: map[string]string{"type": "test-capture-pod"}},
						Spec:       corev1.PodSpec{NodeName: "node1"},
						Status: corev1.PodStatus{
							PodIP:  "10.225.0.4",
							PodIPs: []corev1.PodIP{{IP: "10.225.0.4"}},
						},
					},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{corev1.LabelHostname: "node1", corev1.LabelOSStable: "linux"}}}},
			},
			volumeMounts: []corev1.VolumeMount{
				{
					Name:      captureConstants.CaptureHostPathVolumeName,
					MountPath: hostPath,
				},
			},
			volumes: []corev1.Volume{
				{
					Name: captureConstants.CaptureHostPathVolumeName,
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: hostPath,
							Type: &captureFolderHostPathType,
						},
					},
				},
			},
			podEnv: []v1.EnvVar{
				{Name: captureConstants.CaptureDurationEnvKey, Value: "1m0s"},
				{Name: string(captureConstants.CaptureOutputLocationEnvKeyHostPath), Value: hostPath},
				{Name: captureConstants.CaptureNameEnvKey, Value: captureName},
				{Name: captureConstants.CaptureStartTimestampEnvKey, Value: file.TimeToString(timestamp)},
				{Name: captureConstants.IncludeMetadataEnvKey, Value: "false"},
				{Name: captureConstants.NodeHostNameEnvKey, Value: "node1"},
				{Name: captureConstants.TcpdumpFilterEnvKey, Value: "(host 10.225.0.4)"},
				{
					Name: telemetry.EnvPodName,
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.name",
						},
					},
				},
				{
					Name:  captureConstants.ApiserverEnvKey,
					Value: apiServerURL,
				},
			},
		},
		{
			name: "tcpdumpfilter: dual-stack Pod",
			capture: retinav1alpha1.Capture{
				ObjectMeta: metav1.ObjectMeta{
					Name: captureName,
				},
				Status: retinav1alpha1.CaptureStatus{
					StartTime: timestamp,
				},
				Spec: retinav1alpha1.CaptureSpec{
					CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
						CaptureTarget: retinav1alpha1.CaptureTarget{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"type": "test-capture-pod"},
							},
						},
						CaptureOption: retinav1alpha1.CaptureOption{
							Duration: &metav1.Duration{Duration: 1 * time.Minute},
						},
					},
					OutputConfiguration: retinav1alpha1.OutputConfiguration{
						HostPath: &hostPath,
					},
				},
			},
			podList: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "test-capture-pod", Namespace: "default", Labels: map[string]string{"type": "test-capture-pod"}},
						Spec:       corev1.PodSpec{NodeName: "node1"},
						Status: corev1.PodStatus{
							PodIP: "10.225.0.4",
							PodIPs: []corev1.PodIP{
								{IP: "10.225.0.4"},
								{IP: "fd5c:d9f1:79c5:fd83::21e"},
							},
						},
					},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{corev1.LabelHostname: "node1", corev1.LabelOSStable: "linux"}}}},
			},
			volumeMounts: []corev1.VolumeMount{
				{
					Name:      captureConstants.CaptureHostPathVolumeName,
					MountPath: hostPath,
				},
			},
			volumes: []corev1.Volume{
				{
					Name: captureConstants.CaptureHostPathVolumeName,
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: hostPath,
							Type: &captureFolderHostPathType,
						},
					},
				},
			},
			podEnv: []v1.EnvVar{
				{Name: captureConstants.CaptureDurationEnvKey, Value: "1m0s"},
				{Name: string(captureConstants.CaptureOutputLocationEnvKeyHostPath), Value: hostPath},
				{Name: captureConstants.CaptureNameEnvKey, Value: captureName},
				{Name: captureConstants.CaptureStartTimestampEnvKey, Value: file.TimeToString(timestamp)},
				{Name: captureConstants.IncludeMetadataEnvKey, Value: "false"},
				{Name: captureConstants.NodeHostNameEnvKey, Value: "node1"},
				{Name: captureConstants.TcpdumpFilterEnvKey, Value: "(host 10.225.0.4 or host fd5c:d9f1:79c5:fd83::21e)"},
				{
					Name: telemetry.EnvPodName,
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.name",
						},
					},
				},
				{
					Name:  captureConstants.ApiserverEnvKey,
					Value: apiServerURL,
				},
			},
		},
		{
			name: "netshfilter: dual-stack Pod",
			capture: retinav1alpha1.Capture{
				ObjectMeta: metav1.ObjectMeta{
					Name: captureName,
				},
				Status: retinav1alpha1.CaptureStatus{
					StartTime: timestamp,
				},
				Spec: retinav1alpha1.CaptureSpec{
					CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
						CaptureTarget: retinav1alpha1.CaptureTarget{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"type": "test-capture-pod"},
							},
						},
						CaptureOption: retinav1alpha1.CaptureOption{
							Duration: &metav1.Duration{Duration: 1 * time.Minute},
						},
					},
					OutputConfiguration: retinav1alpha1.OutputConfiguration{
						HostPath: &hostPath,
					},
				},
			},
			podList: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "test-capture-pod", Namespace: "default", Labels: map[string]string{"type": "test-capture-pod"}},
						Spec:       corev1.PodSpec{NodeName: "node1"},
						Status: corev1.PodStatus{
							PodIP: "10.225.0.4",
							PodIPs: []corev1.PodIP{
								{IP: "10.225.0.4"},
								{IP: "fd5c:d9f1:79c5:fd83::21e"},
							},
						},
					},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{corev1.LabelHostname: "node1", corev1.LabelOSStable: "windows"}}}},
			},
			volumeMounts: []corev1.VolumeMount{
				{
					Name:      captureConstants.CaptureHostPathVolumeName,
					MountPath: hostPath,
				},
			},
			volumes: []corev1.Volume{
				{
					Name: captureConstants.CaptureHostPathVolumeName,
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: hostPath,
							Type: &captureFolderHostPathType,
						},
					},
				},
			},
			podEnv: []v1.EnvVar{
				{Name: captureConstants.CaptureDurationEnvKey, Value: "1m0s"},
				{Name: string(captureConstants.CaptureOutputLocationEnvKeyHostPath), Value: hostPath},
				{Name: captureConstants.CaptureNameEnvKey, Value: captureName},
				{Name: captureConstants.CaptureStartTimestampEnvKey, Value: file.TimeToString(timestamp)},
				{Name: captureConstants.IncludeMetadataEnvKey, Value: "false"},
				{Name: captureConstants.NodeHostNameEnvKey, Value: "node1"},
				{Name: captureConstants.NetshFilterEnvKey, Value: "IPv4.Address=(10.225.0.4) IPv6.Address=(fd5c:d9f1:79c5:fd83::21e)"},
				{
					Name: telemetry.EnvPodName,
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.name",
						},
					},
				},
				{
					Name:  captureConstants.ApiserverEnvKey,
					Value: apiServerURL,
				},
			},
			isWindows: true,
		},
		{
			name: "netshfilter: pod ip address and tcpdumpfilter coexist",
			capture: retinav1alpha1.Capture{
				ObjectMeta: metav1.ObjectMeta{
					Name: captureName,
				},
				Status: retinav1alpha1.CaptureStatus{
					StartTime: timestamp,
				},
				Spec: retinav1alpha1.CaptureSpec{
					CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
						CaptureTarget: retinav1alpha1.CaptureTarget{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"type": "test-capture-pod"},
							},
						},
						TcpdumpFilter: &tcpdumpFilter,
						CaptureOption: retinav1alpha1.CaptureOption{
							Duration: &metav1.Duration{Duration: 1 * time.Minute},
						},
					},
					OutputConfiguration: retinav1alpha1.OutputConfiguration{
						HostPath: &hostPath,
					},
				},
			},
			podList: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "test-capture-pod", Namespace: "default", Labels: map[string]string{"type": "test-capture-pod"}},
						Spec:       corev1.PodSpec{NodeName: "node1"},
						Status: corev1.PodStatus{
							PodIP:  "10.225.0.4",
							PodIPs: []corev1.PodIP{{IP: "10.225.0.4"}},
						},
					},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{corev1.LabelHostname: "node1", corev1.LabelOSStable: "windows"}}}},
			},
			volumeMounts: []corev1.VolumeMount{
				{
					Name:      captureConstants.CaptureHostPathVolumeName,
					MountPath: hostPath,
				},
			},
			volumes: []corev1.Volume{
				{
					Name: captureConstants.CaptureHostPathVolumeName,
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: hostPath,
							Type: &captureFolderHostPathType,
						},
					},
				},
			},
			podEnv: []v1.EnvVar{
				{Name: captureConstants.CaptureDurationEnvKey, Value: "1m0s"},
				{Name: string(captureConstants.CaptureOutputLocationEnvKeyHostPath), Value: hostPath},
				{Name: captureConstants.CaptureNameEnvKey, Value: captureName},
				{Name: captureConstants.CaptureStartTimestampEnvKey, Value: file.TimeToString(timestamp)},
				{Name: captureConstants.IncludeMetadataEnvKey, Value: "false"},
				{Name: captureConstants.NodeHostNameEnvKey, Value: "node1"},
				{Name: captureConstants.TcpdumpRawFilterEnvKey, Value: "-i eth0"},
				{Name: captureConstants.NetshFilterEnvKey, Value: "IPv4.Address=(10.225.0.4)"},
				{
					Name: telemetry.EnvPodName,
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.name",
						},
					},
				},
				{
					Name:  captureConstants.ApiserverEnvKey,
					Value: apiServerURL,
				},
			},
			isWindows: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			objects := []runtime.Object{}
			if tt.podList != nil {
				objects = append(objects, tt.podList)
			}
			if tt.nodeList != nil {
				objects = append(objects, tt.nodeList)
			}
			if tt.existingPVC != nil {
				objects = append(objects, tt.existingPVC)
			}
			k8sClient := fakeclientset.NewSimpleClientset(objects...)
			captureToPodTranslator := NewCaptureToPodTranslatorForTest(k8sClient)
			jobs, err := captureToPodTranslator.TranslateCaptureToJobs(ctx, &tt.capture)
			if tt.wantErr != (err != nil) {
				t.Errorf("TranslateCaptureToJobs() want(%t) error, got error %s", tt.wantErr, err)
			}
			if tt.wantErr {
				return
			}
			job := commonJob.DeepCopy()
			job.Spec.Template.Spec.Containers[0].Env = tt.podEnv
			job.Spec.Template.Spec.Containers[0].VolumeMounts = tt.volumeMounts
			job.Spec.Template.Spec.Volumes = tt.volumes

			for _, env := range tt.podEnv {
				if env.Name == captureConstants.CaptureStartTimestampEnvKey {
					_, err := file.StringToTime(env.Value)
					if err != nil {
						t.Errorf("TranslateCaptureToJobs() error with capture timestamp: %v", err)
					}
				}
			}

			if tt.isWindows {
				containerAdministrator := "NT AUTHORITY\\SYSTEM"
				useHostProcess := true
				job.Spec.Template.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
					Capabilities: &corev1.Capabilities{
						Add: []corev1.Capability{
							"NET_ADMIN", "SYS_ADMIN",
						},
					},
					WindowsOptions: &corev1.WindowsSecurityContextOptions{
						HostProcess:   &useHostProcess,
						RunAsUserName: &containerAdministrator,
					},
				}
				job.Spec.Template.Spec.Containers[0].Command = []string{captureConstants.CaptureContainerEntrypointWin}
			}

			if tt.capture.Spec.OutputConfiguration.HostPath != nil {
				job.Spec.Template.Annotations[captureConstants.CaptureHostPathAnnotationKey] = hostPath
			}

			cmpOption := cmp.Options{
				cmpopts.SortSlices(func(enVar1, enVar2 corev1.EnvVar) bool { return enVar1.Name < enVar2.Name }),
				cmp.Comparer(func(x, y corev1.EnvVar) bool {
					if isIgnorableEnvVar(x) || isIgnorableEnvVar(y) {
						return true
					}
					return x.Name == y.Name && x.Value == y.Value
				}),
			}

			if diff := cmp.Diff([]*batchv1.Job{job}, jobs, cmpOption); diff != "" {
				t.Errorf("TranslateCaptureToJobs() mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func Test_CaptureToPodTranslator_TranslateCaptureToJobs_JobNumLimit(t *testing.T) {
	ctx, cancel := TestContext(t)
	defer cancel()

	timestamp := file.Now()

	captureName := "capture-test"
	hostPath := "/tmp/capture"
	cases := []struct {
		name        string
		capture     retinav1alpha1.Capture
		nodeList    *corev1.NodeList
		jobNumLimit int
		wantErr     bool
	}{
		{
			name: "job number limit does not reach",
			capture: retinav1alpha1.Capture{
				ObjectMeta: metav1.ObjectMeta{
					Name: captureName,
				},
				Status: retinav1alpha1.CaptureStatus{
					StartTime: timestamp,
				},
				Spec: retinav1alpha1.CaptureSpec{
					CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
						CaptureTarget: retinav1alpha1.CaptureTarget{
							NodeSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{{
									Key:      corev1.LabelHostname,
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{"node1", "node2"},
								}},
							},
						},
						CaptureOption: retinav1alpha1.CaptureOption{
							Duration: &metav1.Duration{Duration: 1 * time.Minute},
						},
					},
					OutputConfiguration: retinav1alpha1.OutputConfiguration{
						HostPath: &hostPath,
					},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{corev1.LabelHostname: "node1", corev1.LabelOSStable: "linux"}}},
					{ObjectMeta: metav1.ObjectMeta{Name: "node2", Labels: map[string]string{corev1.LabelHostname: "node2", corev1.LabelOSStable: "linux"}}},
				},
			},
			jobNumLimit: 2,
		},
		{
			name: "job number limit reach",
			capture: retinav1alpha1.Capture{
				ObjectMeta: metav1.ObjectMeta{
					Name: captureName,
				},
				Status: retinav1alpha1.CaptureStatus{
					StartTime: timestamp,
				},
				Spec: retinav1alpha1.CaptureSpec{
					CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
						CaptureTarget: retinav1alpha1.CaptureTarget{
							NodeSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{{
									Key:      corev1.LabelHostname,
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{"node1", "node2", "node3"},
								}},
							},
						},
						CaptureOption: retinav1alpha1.CaptureOption{
							Duration: &metav1.Duration{Duration: 1 * time.Minute},
						},
					},
					OutputConfiguration: retinav1alpha1.OutputConfiguration{
						HostPath: &hostPath,
					},
				},
			},
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{corev1.LabelHostname: "node1", corev1.LabelOSStable: "linux"}}},
					{ObjectMeta: metav1.ObjectMeta{Name: "node2", Labels: map[string]string{corev1.LabelHostname: "node2", corev1.LabelOSStable: "linux"}}},
					{ObjectMeta: metav1.ObjectMeta{Name: "node3", Labels: map[string]string{corev1.LabelHostname: "node3", corev1.LabelOSStable: "linux"}}},
				},
			},
			jobNumLimit: 2,
			wantErr:     true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			objects := []runtime.Object{}
			if tt.nodeList != nil {
				objects = append(objects, tt.nodeList)
			}

			k8sClient := fakeclientset.NewSimpleClientset(objects...)
			captureToPodTranslator := NewCaptureToPodTranslatorForTest(k8sClient)
			captureToPodTranslator.config.CaptureJobNumLimit = tt.jobNumLimit
			_, err := captureToPodTranslator.TranslateCaptureToJobs(ctx, &tt.capture)
			if tt.wantErr != (err != nil) {
				t.Errorf("TranslateCaptureToJobs() want(%t) error, got error %s", tt.wantErr, err)
			}
			if !tt.wantErr {
				return
			}

			require.IsType(t, err, CaptureJobNumExceedLimitError{}, "TranslateCaptureToJobs() does not raise CaptureJobNumExceedLimitError")
		})
	}
}

func Test_CaptureToPodTranslator_ValidateTargetSelector(t *testing.T) {
	nodeSelector := map[string]string{"agent-pool": "agent-pool"}
	namespaceSelector := map[string]string{"kubernetes.io/cluster-service": "true"}
	podSelector := map[string]string{"kubernetes.io/os": "linux"}

	cases := []struct {
		name          string
		captureTarget retinav1alpha1.CaptureTarget
		wantErr       bool
	}{
		{
			name: "Provided: nodeSelector",
			captureTarget: retinav1alpha1.CaptureTarget{
				NodeSelector: &metav1.LabelSelector{
					MatchLabels: nodeSelector,
				},
			},
			wantErr: false,
		},
		{
			name: "Provided: namespaceSelector, podSelector",
			captureTarget: retinav1alpha1.CaptureTarget{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: namespaceSelector,
				},
				PodSelector: &metav1.LabelSelector{
					MatchLabels: podSelector,
				},
			},
			wantErr: false,
		},
		{
			name: "Provided: nodeSelector, namespaceSelector, podSelector",
			captureTarget: retinav1alpha1.CaptureTarget{
				NodeSelector: &metav1.LabelSelector{
					MatchLabels: nodeSelector,
				},
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: namespaceSelector,
				},
				PodSelector: &metav1.LabelSelector{
					MatchLabels: podSelector,
				},
			},
			wantErr: true,
		},
		{
			name: "Provided: nodeSelector, namespaceSelector",
			captureTarget: retinav1alpha1.CaptureTarget{
				NodeSelector: &metav1.LabelSelector{
					MatchLabels: nodeSelector,
				},
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: namespaceSelector,
				},
			},
			wantErr: true,
		},
		{
			name: "Provided: nodeSelector, podSelector",
			captureTarget: retinav1alpha1.CaptureTarget{
				NodeSelector: &metav1.LabelSelector{
					MatchLabels: nodeSelector,
				},
				PodSelector: &metav1.LabelSelector{
					MatchLabels: podSelector,
				},
			},
			wantErr: true,
		},
		{
			name: "Provided: namespaceSelector",
			captureTarget: retinav1alpha1.CaptureTarget{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: namespaceSelector,
				},
			},
			wantErr: true,
		},
		{
			name: "Provided: podSelector",
			captureTarget: retinav1alpha1.CaptureTarget{
				PodSelector: &metav1.LabelSelector{
					MatchLabels: podSelector,
				},
			},
			wantErr: false,
		},
		{
			name:          "Provided: (none)",
			captureTarget: retinav1alpha1.CaptureTarget{},
			wantErr:       true,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			objects := []runtime.Object{}
			k8sClient := fakeclientset.NewSimpleClientset(objects...)
			captureToPodTranslator := NewCaptureToPodTranslatorForTest(k8sClient)
			err := captureToPodTranslator.validateTargetSelector(tt.captureTarget)
			if tt.wantErr != (err != nil) {
				t.Errorf("%s validateTargetSelector() want(%t) error, got error %s", tt.name, tt.wantErr, err)
			}
		})
	}
}

func Test_CaptureToPodTranslator_obtainTcpdumpFilters(t *testing.T) {
	cases := []struct {
		name                string
		captureConfig       retinav1alpha1.CaptureConfiguration
		wantedTcpdumpFilter string
		wantErr             bool
	}{
		{
			name:                "empty filter",
			captureConfig:       retinav1alpha1.CaptureConfiguration{},
			wantedTcpdumpFilter: "",
		},
		{
			name: "include filter is empty",
			captureConfig: retinav1alpha1.CaptureConfiguration{
				Filters: &retinav1alpha1.CaptureConfigurationFilters{
					Exclude: []string{"192.168.1.1:80", "192.168.1.2:90", "*:101", "192.168.1.3"},
				},
			},
			wantedTcpdumpFilter: "not ((port 101) or (host 192.168.1.1 and port 80) or (host 192.168.1.2 and port 90) or (host 192.168.1.3))",
		},
		{
			name: "exclude filter is empty",
			captureConfig: retinav1alpha1.CaptureConfiguration{
				Filters: &retinav1alpha1.CaptureConfigurationFilters{
					Include: []string{"192.168.0.1:80", "192.168.0.2:90", "*:100", "192.168.0.3"},
				},
			},
			wantedTcpdumpFilter: "((port 100) or (host 192.168.0.1 and port 80) or (host 192.168.0.2 and port 90) or (host 192.168.0.3))",
		},
		{
			name: "combine any ip and limited ports",
			captureConfig: retinav1alpha1.CaptureConfiguration{
				Filters: &retinav1alpha1.CaptureConfigurationFilters{
					Include: []string{"*:100", "200"},
				},
			},
			wantedTcpdumpFilter: "((port 100) or (port 200))",
		},
		{
			name: "combine limited ip with any port",
			captureConfig: retinav1alpha1.CaptureConfiguration{
				Filters: &retinav1alpha1.CaptureConfigurationFilters{
					Include: []string{"192.168.0.1", "192.168.0.2"},
				},
			},
			wantedTcpdumpFilter: "((host 192.168.0.1) or (host 192.168.0.2))",
		},
		{
			name: "include filter and exclude filter",
			captureConfig: retinav1alpha1.CaptureConfiguration{
				Filters: &retinav1alpha1.CaptureConfigurationFilters{
					Include: []string{"192.168.0.1:80", "192.168.0.2:90", "*:100", "192.168.0.3"},
					Exclude: []string{"192.168.1.1:80", "192.168.1.2:90", "*:101", "192.168.1.3"},
				},
			},
			wantedTcpdumpFilter: "((port 100) or (host 192.168.0.1 and port 80) or (host 192.168.0.2 and port 90) or (host 192.168.0.3)) and not ((port 101) or (host 192.168.1.1 and port 80) or (host 192.168.1.2 and port 90) or (host 192.168.1.3))",
		},
		{
			name: "include and exclude filters",
			captureConfig: retinav1alpha1.CaptureConfiguration{
				Filters: &retinav1alpha1.CaptureConfigurationFilters{
					Include: []string{"192.168.0.1:80"},
					Exclude: []string{"192.168.1.1:80"},
				},
			},
			wantedTcpdumpFilter: "((host 192.168.0.1 and port 80)) and not ((host 192.168.1.1 and port 80))",
		},
		{
			name: "raise error when when the port filter exceeds lower limit",
			captureConfig: retinav1alpha1.CaptureConfiguration{
				Filters: &retinav1alpha1.CaptureConfigurationFilters{
					Include: []string{"-1"},
				},
			},
			wantErr: true,
		},
		{
			name: "raise error when the port filter exceeds upper limit",
			captureConfig: retinav1alpha1.CaptureConfiguration{
				Filters: &retinav1alpha1.CaptureConfigurationFilters{
					Include: []string{"65536"},
				},
			},
			wantErr: true,
		},
		{
			name: "raise error when specifying bad ip address",
			captureConfig: retinav1alpha1.CaptureConfiguration{
				Filters: &retinav1alpha1.CaptureConfigurationFilters{
					Include: []string{"192.168.0.1:80", "192.168.0.256"},
				},
			},
			wantErr: true,
		},
		{
			name: "return empty filter when filter is empty",
			captureConfig: retinav1alpha1.CaptureConfiguration{
				Filters: &retinav1alpha1.CaptureConfigurationFilters{
					Include: []string{},
				},
			},
			wantedTcpdumpFilter: "",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			k8sClient := fakeclientset.NewSimpleClientset()
			captureToPodTranslator := NewCaptureToPodTranslatorForTest(k8sClient)
			tcpdumpFilter, err := captureToPodTranslator.obtainTcpdumpFilters(tt.captureConfig)
			if tt.wantErr != (err != nil) {
				t.Errorf("obtainTcpdumpFilters() want(%t) error, got error %s", tt.wantErr, err)
			}

			if diff := cmp.Diff(tt.wantedTcpdumpFilter, tcpdumpFilter); diff != "" {
				t.Errorf("TranslateCaptureToJobs() mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestUpdateTcpdumpFilterWithPodIPAddress(t *testing.T) {
	cases := []struct {
		name                       string
		podIPAddresses             []string
		tcpdumpFilter              string
		wantedUpdatedTcpdumpFilter string
	}{
		{
			name:                       "empty pod ip address and filter",
			podIPAddresses:             []string{},
			tcpdumpFilter:              "",
			wantedUpdatedTcpdumpFilter: "",
		},
		{
			name:                       "empty pod ip address",
			podIPAddresses:             []string{},
			tcpdumpFilter:              "((host 192.168.1.1) or (host 192.168.1.2))",
			wantedUpdatedTcpdumpFilter: "((host 192.168.1.1) or (host 192.168.1.2))",
		},
		{
			name:                       "one pod ip address and empty filter",
			podIPAddresses:             []string{"192.168.0.1"},
			tcpdumpFilter:              "",
			wantedUpdatedTcpdumpFilter: "(host 192.168.0.1)",
		},
		{
			name:                       "multiple pod ip address and empty filter",
			podIPAddresses:             []string{"192.168.0.1", "192.168.0.2"},
			tcpdumpFilter:              "",
			wantedUpdatedTcpdumpFilter: "(host 192.168.0.1 or host 192.168.0.2)",
		},
		{
			name:                       "pod ip address and filter",
			podIPAddresses:             []string{"192.168.1.1", "192.168.1.2"},
			tcpdumpFilter:              "((host 192.168.0.1) or (host 192.168.0.2))",
			wantedUpdatedTcpdumpFilter: "((host 192.168.0.1) or (host 192.168.0.2)) or (host 192.168.1.1 or host 192.168.1.2)",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			gotUpdatedTcpdumpFilter := updateTcpdumpFilterWithPodIPAddress(tt.podIPAddresses, tt.tcpdumpFilter)
			if diff := cmp.Diff(tt.wantedUpdatedTcpdumpFilter, gotUpdatedTcpdumpFilter); diff != "" {
				t.Errorf("updateTcpdumpFilterWithPodIPAddress() mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestGetNetshFilterWithPodIPAddress(t *testing.T) {
	cases := []struct {
		name              string
		podIPAddresses    []string
		wantedNetshFilter string
	}{
		{
			name:              "empty pod ip address",
			podIPAddresses:    []string{},
			wantedNetshFilter: "",
		},
		{
			name:              "one pod ip address",
			podIPAddresses:    []string{"192.168.1.1"},
			wantedNetshFilter: "IPv4.Address=(192.168.1.1)",
		},
		{
			name:              "multiple pod ip addresses",
			podIPAddresses:    []string{"192.168.1.1", "192.168.1.2"},
			wantedNetshFilter: "IPv4.Address=(192.168.1.1,192.168.1.2)",
		},
		{
			name:              "one ipv4 and ipv6 address",
			podIPAddresses:    []string{"192.168.1.1", "2001:1234:5678:9abc::5"},
			wantedNetshFilter: "IPv4.Address=(192.168.1.1) IPv6.Address=(2001:1234:5678:9abc::5)",
		},
		{
			name:              "multiple ipv4 and ipv6 addresses",
			podIPAddresses:    []string{"192.168.1.1", "192.168.1.2", "2001:1234:5678:9abc::5", "2001:1234:5678:9abc::6"},
			wantedNetshFilter: "IPv4.Address=(192.168.1.1,192.168.1.2) IPv6.Address=(2001:1234:5678:9abc::5,2001:1234:5678:9abc::6)",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			gotNetshFilter := getNetshFilterWithPodIPAddress(tt.podIPAddresses)
			if diff := cmp.Diff(tt.wantedNetshFilter, gotNetshFilter); diff != "" {
				t.Errorf("getNetshFilterWithPodIPAddress() mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
