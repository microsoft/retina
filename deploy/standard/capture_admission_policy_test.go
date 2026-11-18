// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package deploy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"helm.sh/helm/v4/pkg/chart/common"
	chartutil "helm.sh/helm/v4/pkg/chart/common/util"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
	"helm.sh/helm/v4/pkg/engine"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
)

const (
	retinaChartPath               = "manifests/controller/helm/retina"
	captureAdmissionPolicyTmpl    = "retina/templates/capture-admission-policy.yaml"
	operatorTmpl                  = "retina/templates/operator.yaml"
	captureTargetScopePolicyName  = "retina-operator-capture-target-scope"
	captureNodesClusterRole       = "retina-operator-capture-nodes"
	captureNamespacesClusterRole  = "retina-operator-capture-namespaces"
	minAdmissionPolicyKubeVersion = "v1.30.0"
)

// renderRetinaChart renders every template of the standard Retina chart for the
// given Kubernetes version, with overrides merged onto the chart's default values.
func renderRetinaChart(t *testing.T, kubeVersion string, overrides map[string]any) (map[string]string, error) {
	t.Helper()
	return renderRetinaChartWithBaseValues(t, kubeVersion, nil, overrides)
}

// renderRetinaChartWithBaseValues is renderRetinaChart, but lets mutateBase edit
// the chart's own values first. `helm upgrade --reuse-values` renders the new
// chart against the previous release's values instead of the new defaults, which
// is simulated by removing keys the previous chart version did not have.
func renderRetinaChartWithBaseValues(t *testing.T, kubeVersion string, mutateBase func(map[string]any), overrides map[string]any) (map[string]string, error) {
	t.Helper()

	chrt, err := loader.Load(filepath.FromSlash(retinaChartPath))
	require.NoError(t, err)
	if mutateBase != nil {
		mutateBase(chrt.Values)
	}

	kv, err := common.ParseKubeVersion(kubeVersion)
	require.NoError(t, err)
	caps := &common.Capabilities{KubeVersion: *kv, APIVersions: common.DefaultVersionSet}

	vals, err := chartutil.ToRenderValues(chrt, overrides, common.ReleaseOptions{Name: "retina", Namespace: "kube-system", IsInstall: true}, caps)
	require.NoError(t, err)

	rendered, err := engine.Render(chrt, vals)
	if err != nil {
		return nil, fmt.Errorf("rendering retina chart: %w", err)
	}
	return rendered, nil
}

func operatorEnabled(extra map[string]any) map[string]any {
	vals := map[string]any{"operator": map[string]any{"enabled": true}}
	for k, v := range extra {
		vals[k] = v
	}
	return vals
}

// decodeManifest splits a rendered multi-document YAML template into objects,
// skipping empty documents.
func decodeManifest(t *testing.T, manifest string) []*unstructured.Unstructured {
	t.Helper()

	var objs []*unstructured.Unstructured
	dec := utilyaml.NewYAMLOrJSONDecoder(bytes.NewBufferString(manifest), 4096)
	for {
		obj := &unstructured.Unstructured{}
		err := dec.Decode(&obj.Object)
		if errors.Is(err, io.EOF) {
			return objs
		}
		require.NoError(t, err)
		if len(obj.Object) > 0 {
			objs = append(objs, obj)
		}
	}
}

func objectsByKind(t *testing.T, manifest string) map[string][]*unstructured.Unstructured {
	t.Helper()

	byKind := map[string][]*unstructured.Unstructured{}
	for _, obj := range decodeManifest(t, manifest) {
		byKind[obj.GetKind()] = append(byKind[obj.GetKind()], obj)
	}
	return byKind
}

func fromUnstructured[T any](t *testing.T, obj *unstructured.Unstructured) *T {
	t.Helper()

	out := new(T)
	require.NoError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, out))
	return out
}

func TestCaptureAdmissionPolicyRendersWhenOperatorEnabled(t *testing.T) {
	// Managed-provider version strings carry a pre-release suffix; the version gate must still accept them.
	for _, kubeVersion := range []string{minAdmissionPolicyKubeVersion, "v1.30.2-eks-a1b2c3", "v1.37.0"} {
		t.Run(kubeVersion, func(t *testing.T) {
			rendered, err := renderRetinaChart(t, kubeVersion, operatorEnabled(nil))
			require.NoError(t, err)

			byKind := objectsByKind(t, rendered[captureAdmissionPolicyTmpl])
			require.Len(t, byKind["ValidatingAdmissionPolicy"], 1)
			require.Len(t, byKind["ValidatingAdmissionPolicyBinding"], 1)
			require.Len(t, byKind["ClusterRole"], 2)
			require.Len(t, byKind, 3, "unexpected kinds rendered")

			policy := fromUnstructured[admissionregistrationv1.ValidatingAdmissionPolicy](t, byKind["ValidatingAdmissionPolicy"][0])
			require.Equal(t, captureTargetScopePolicyName, policy.Name)
			require.NotNil(t, policy.Spec.FailurePolicy)
			require.Equal(t, admissionregistrationv1.Fail, *policy.Spec.FailurePolicy)
			require.Len(t, policy.Spec.MatchConstraints.ResourceRules, 1)
			rule := policy.Spec.MatchConstraints.ResourceRules[0]
			require.Equal(t, []string{"retina.sh"}, rule.APIGroups)
			require.Equal(t, []string{"captures"}, rule.Resources)
			require.ElementsMatch(t,
				[]admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
				rule.Operations)
			require.Len(t, policy.Spec.Validations, 2)
			require.Len(t, policy.Spec.MatchConditions, 1)
			require.Contains(t, policy.Spec.MatchConditions[0].Expression,
				"request.userInfo.username != 'system:serviceaccount:kube-system:retina-operator'")

			binding := fromUnstructured[admissionregistrationv1.ValidatingAdmissionPolicyBinding](t, byKind["ValidatingAdmissionPolicyBinding"][0])
			require.Equal(t, captureTargetScopePolicyName, binding.Spec.PolicyName)
			require.Equal(t, []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny}, binding.Spec.ValidationActions)

			verbsByRole := map[string][]string{}
			for _, obj := range byKind["ClusterRole"] {
				role := fromUnstructured[rbacv1.ClusterRole](t, obj)
				require.Len(t, role.Rules, 1)
				require.Equal(t, []string{"retina.sh"}, role.Rules[0].APIGroups)
				require.Equal(t, []string{"captures"}, role.Rules[0].Resources)
				verbsByRole[role.Name] = role.Rules[0].Verbs
			}
			require.Equal(t, map[string][]string{
				captureNodesClusterRole:      {"capture-nodes"},
				captureNamespacesClusterRole: {"capture-namespaces"},
			}, verbsByRole)
		})
	}
}

func TestCaptureAdmissionPolicyNotRendered(t *testing.T) {
	tests := []struct {
		name      string
		overrides map[string]any
	}{
		{
			name:      "operator disabled (chart default)",
			overrides: nil,
		},
		{
			name:      "operator disabled with policy explicitly enabled",
			overrides: map[string]any{"capture": map[string]any{"admissionPolicy": map[string]any{"enabled": true}}},
		},
		{
			name:      "operator enabled with policy disabled",
			overrides: operatorEnabled(map[string]any{"capture": map[string]any{"admissionPolicy": map[string]any{"enabled": false}}}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered, err := renderRetinaChart(t, minAdmissionPolicyKubeVersion, tt.overrides)
			require.NoError(t, err)
			require.Empty(t, decodeManifest(t, rendered[captureAdmissionPolicyTmpl]))
		})
	}
}

func TestCaptureAdmissionPolicyFailsOnUnsupportedKubeVersion(t *testing.T) {
	for _, kubeVersion := range []string{"v1.29.9", "v1.29.15-gke.1000"} {
		t.Run(kubeVersion, func(t *testing.T) {
			_, err := renderRetinaChart(t, kubeVersion, operatorEnabled(nil))
			require.Error(t, err)
			require.ErrorContains(t, err, "requires Kubernetes >= 1.30")
			require.ErrorContains(t, err, kubeVersion)
			require.ErrorContains(t, err, "capture.admissionPolicy.enabled=false")
		})
	}
}

// All object names, and the binding's reference to the policy, derive from
// operator.name; a mismatch would leave the binding pointing at no policy. The
// operator exemption must name the operator's actual ServiceAccount.
func TestCaptureAdmissionPolicyFollowsOperatorName(t *testing.T) {
	rendered, err := renderRetinaChart(t, minAdmissionPolicyKubeVersion,
		map[string]any{"operator": map[string]any{"enabled": true, "name": "custom-operator"}, "namespace": "retina-system"})
	require.NoError(t, err)

	byKind := objectsByKind(t, rendered[captureAdmissionPolicyTmpl])
	require.Len(t, byKind["ValidatingAdmissionPolicy"], 1)
	require.Len(t, byKind["ValidatingAdmissionPolicyBinding"], 1)

	policy := fromUnstructured[admissionregistrationv1.ValidatingAdmissionPolicy](t, byKind["ValidatingAdmissionPolicy"][0])
	binding := fromUnstructured[admissionregistrationv1.ValidatingAdmissionPolicyBinding](t, byKind["ValidatingAdmissionPolicyBinding"][0])
	require.Equal(t, "custom-operator-capture-target-scope", policy.Name)
	require.Equal(t, "custom-operator-capture-target-scope", binding.Name)
	require.Equal(t, policy.Name, binding.Spec.PolicyName)
	require.Len(t, policy.Spec.MatchConditions, 1)
	require.Contains(t, policy.Spec.MatchConditions[0].Expression,
		"request.userInfo.username != 'system:serviceaccount:retina-system:custom-operator'")

	roleNames := make([]string, 0, len(byKind["ClusterRole"]))
	for _, obj := range byKind["ClusterRole"] {
		roleNames = append(roleNames, obj.GetName())
	}
	require.ElementsMatch(t, []string{"custom-operator-capture-nodes", "custom-operator-capture-namespaces"}, roleNames)
}

func TestCaptureAdmissionPolicyOptOutOnUnsupportedKubeVersion(t *testing.T) {
	rendered, err := renderRetinaChart(t, "v1.29.9",
		operatorEnabled(map[string]any{"capture": map[string]any{"admissionPolicy": map[string]any{"enabled": false}}}))
	require.NoError(t, err)
	require.Empty(t, decodeManifest(t, rendered[captureAdmissionPolicyTmpl]))
}

// Values from a release installed before capture.admissionPolicy existed (as used
// by `helm upgrade --reuse-values`) must not break rendering, and must get the
// secure default; an explicit opt-out must still be honored.
func TestCaptureAdmissionPolicyWithPreviousReleaseValues(t *testing.T) {
	withoutAdmissionPolicy := func(vals map[string]any) {
		capture, ok := vals["capture"].(map[string]any)
		require.True(t, ok, "chart values must have a capture section")
		delete(capture, "admissionPolicy")
	}

	tests := []struct {
		name       string
		mutateBase func(map[string]any)
		overrides  map[string]any
		wantPolicy bool
	}{
		{
			name:       "admissionPolicy missing defaults to enabled",
			mutateBase: withoutAdmissionPolicy,
			overrides:  operatorEnabled(nil),
			wantPolicy: true,
		},
		{
			name:       "admissionPolicy missing with explicit opt-out",
			mutateBase: withoutAdmissionPolicy,
			overrides:  operatorEnabled(map[string]any{"capture": map[string]any{"admissionPolicy": map[string]any{"enabled": false}}}),
			wantPolicy: false,
		},
		{
			name:       "admissionPolicy present without enabled defaults to enabled",
			mutateBase: withoutAdmissionPolicy,
			overrides:  operatorEnabled(map[string]any{"capture": map[string]any{"admissionPolicy": map[string]any{}}}),
			wantPolicy: true,
		},
		{
			name:       "admissionPolicy missing with operator disabled",
			mutateBase: withoutAdmissionPolicy,
			overrides:  nil,
			wantPolicy: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered, err := renderRetinaChartWithBaseValues(t, minAdmissionPolicyKubeVersion, tt.mutateBase, tt.overrides)
			require.NoError(t, err)

			byKind := objectsByKind(t, rendered[captureAdmissionPolicyTmpl])
			if tt.wantPolicy {
				require.Len(t, byKind["ValidatingAdmissionPolicy"], 1)
				require.Len(t, byKind["ValidatingAdmissionPolicyBinding"], 1)
				return
			}
			require.Empty(t, byKind)
		})
	}
}
