// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
//go:build !windows

package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
)

const (
	tenantNamespace = "tenant-a"

	// tenant holds only the documented namespace-scoped Capture rights.
	tenantUser = "tenant"
	// nodeCapturer additionally holds the cluster-scoped capture-nodes verb.
	nodeCapturerUser = "node-capturer"
	// namespaceCapturer additionally holds the cluster-scoped capture-namespaces verb.
	namespaceCapturerUser = "namespace-capturer"
	// namespaceGrantedUser holds both verbs, but only through RoleBindings in the
	// tenant namespace, which must not be enough.
	namespaceGrantedUser = "namespace-granted"
	// wildcardUser holds verbs ["*"] on captures cluster-wide, which the docs
	// state already includes both verbs.
	wildcardUser = "wildcard"
	// operatorUser is the operator's ServiceAccount, bound to the chart's operator ClusterRole.
	operatorUser = "system:serviceaccount:kube-system:retina-operator"
	// lookalikeOperatorUser is a ServiceAccount with the operator's name in the
	// tenant namespace, which a tenant could create; it must not be exempt.
	lookalikeOperatorUser = "system:serviceaccount:tenant-a:retina-operator"

	captureFinalizer = "kappio.io/capture-cleanup"
)

type captureTargetPolicyEnv struct {
	ctx     context.Context
	cfg     *rest.Config
	scheme  *runtime.Scheme
	admin   client.Client
	clients map[string]client.Client
}

// TestCaptureAdmissionPolicyEnforcement installs the rendered chart policy into a
// real kube-apiserver (RBAC authorization) and checks who may create and update
// node-wide and cross-namespace Captures.
func TestCaptureAdmissionPolicyEnforcement(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping envtest-based test with -short")
	}
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		t.Skip("KUBEBUILDER_ASSETS is not set; run via `make test`, or set it to the output of `go tool sigs.k8s.io/controller-runtime/tools/setup-envtest use -p path`")
	}

	env := setupCaptureTargetPolicyEnv(t)

	t.Run("create", func(t *testing.T) { testCaptureTargetPolicyCreate(t, env) })
	t.Run("update", func(t *testing.T) { testCaptureTargetPolicyUpdate(t, env) })
	t.Run("operator updates with dropped empty fields", func(t *testing.T) { testCaptureTargetPolicyOperatorDroppedFields(t, env) })
}

func setupCaptureTargetPolicyEnv(t *testing.T) *captureTargetPolicyEnv {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join(filepath.FromSlash(retinaChartPath), "crds")},
		ErrorIfCRDPathMissing: true,
	}
	cfg, err := testEnv.Start()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testEnv.Stop()) })

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, retinav1alpha1.AddToScheme(scheme))

	admin, err := client.New(cfg, client.Options{Scheme: scheme})
	require.NoError(t, err)

	env := &captureTargetPolicyEnv{ctx: ctx, cfg: cfg, scheme: scheme, admin: admin, clients: map[string]client.Client{}}

	rendered, err := renderRetinaChart(t, minAdmissionPolicyKubeVersion, operatorEnabled(nil))
	require.NoError(t, err)

	for _, obj := range decodeManifest(t, rendered[captureAdmissionPolicyTmpl]) {
		require.NoError(t, admin.Create(ctx, obj), "creating %s %s", obj.GetKind(), obj.GetName())
	}
	// Use the chart's real operator RBAC, so the test proves the operator itself
	// still works without the new verbs.
	for _, obj := range decodeManifest(t, rendered[operatorTmpl]) {
		if obj.GetKind() == "ClusterRole" || obj.GetKind() == "ClusterRoleBinding" {
			require.NoError(t, admin.Create(ctx, obj), "creating %s %s", obj.GetKind(), obj.GetName())
		}
	}

	require.NoError(t, admin.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: tenantNamespace}}))
	require.NoError(t, admin.Create(ctx, &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "capture-editor"},
		Rules: []rbacv1.PolicyRule{{
			APIGroups: []string{"retina.sh"},
			Resources: []string{"captures"},
			Verbs:     []string{"create", "get", "list", "watch", "update", "patch", "delete"},
		}},
	}))
	require.NoError(t, admin.Create(ctx, roleBinding("capture-editors", "capture-editor",
		tenantUser, nodeCapturerUser, namespaceCapturerUser, namespaceGrantedUser, lookalikeOperatorUser)))
	require.NoError(t, admin.Create(ctx, clusterRoleBinding("node-capturer", captureNodesClusterRole, nodeCapturerUser)))
	require.NoError(t, admin.Create(ctx, clusterRoleBinding("namespace-capturer", captureNamespacesClusterRole, namespaceCapturerUser)))
	require.NoError(t, admin.Create(ctx, roleBinding("namespaced-capture-nodes", captureNodesClusterRole, namespaceGrantedUser)))
	require.NoError(t, admin.Create(ctx, roleBinding("namespaced-capture-namespaces", captureNamespacesClusterRole, namespaceGrantedUser)))
	require.NoError(t, admin.Create(ctx, &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "capture-wildcard"},
		Rules:      []rbacv1.PolicyRule{{APIGroups: []string{"retina.sh"}, Resources: []string{"captures"}, Verbs: []string{"*"}}},
	}))
	require.NoError(t, admin.Create(ctx, clusterRoleBinding("capture-wildcard", "capture-wildcard", wildcardUser)))

	for _, user := range []string{
		tenantUser, nodeCapturerUser, namespaceCapturerUser, namespaceGrantedUser, wildcardUser, operatorUser, lookalikeOperatorUser,
	} {
		impersonated := rest.CopyConfig(cfg)
		impersonated.Impersonate = rest.ImpersonationConfig{UserName: user}
		c, err := client.New(impersonated, client.Options{Scheme: scheme})
		require.NoError(t, err)
		env.clients[user] = c
	}

	// The policy and RBAC are enforced once the apiserver's informers have
	// synced them; wait until both a denial and a grant are observed.
	require.Eventually(t, func() bool {
		denied := env.clients[tenantUser].Create(ctx, newCapture("probe", nodeTarget()), client.DryRunAll)
		allowed := env.clients[nodeCapturerUser].Create(ctx, newCapture("probe", nodeTarget()), client.DryRunAll)
		return isPolicyDenial(denied, "capture-nodes") && allowed == nil
	}, 60*time.Second, 250*time.Millisecond, "admission policy was never enforced")

	return env
}

func testCaptureTargetPolicyCreate(t *testing.T, env *captureTargetPolicyEnv) {
	tests := []struct {
		name       string
		user       string
		target     retinav1alpha1.CaptureTarget
		deniedVerb string
	}{
		{name: "tenant podSelector in own namespace", user: tenantUser, target: podTarget()},
		{name: "tenant podNames in own namespace", user: tenantUser, target: retinav1alpha1.CaptureTarget{PodNames: []string{"web-0"}}},
		{name: "tenant nodeSelector", user: tenantUser, target: nodeTarget(), deniedVerb: "capture-nodes"},
		{
			name:       "tenant empty nodeSelector (matches every node)",
			user:       tenantUser,
			target:     retinav1alpha1.CaptureTarget{NodeSelector: &metav1.LabelSelector{}},
			deniedVerb: "capture-nodes",
		},
		{name: "tenant namespaceSelector", user: tenantUser, target: namespaceTarget(), deniedVerb: "capture-namespaces"},
		{name: "node-capturer nodeSelector", user: nodeCapturerUser, target: nodeTarget()},
		{name: "node-capturer namespaceSelector", user: nodeCapturerUser, target: namespaceTarget(), deniedVerb: "capture-namespaces"},
		{name: "namespace-capturer namespaceSelector", user: namespaceCapturerUser, target: namespaceTarget()},
		{name: "namespace-capturer nodeSelector", user: namespaceCapturerUser, target: nodeTarget(), deniedVerb: "capture-nodes"},
		{name: "namespace-scoped grant is not enough for nodeSelector", user: namespaceGrantedUser, target: nodeTarget(), deniedVerb: "capture-nodes"},
		{
			name:       "namespace-scoped grant is not enough for namespaceSelector",
			user:       namespaceGrantedUser,
			target:     namespaceTarget(),
			deniedVerb: "capture-namespaces",
		},
		{name: "wildcard verbs cluster-wide nodeSelector", user: wildcardUser, target: nodeTarget()},
		{name: "operator creating node-wide Capture is not exempt", user: operatorUser, target: nodeTarget(), deniedVerb: "capture-nodes"},
		{name: "wildcard verbs cluster-wide namespaceSelector", user: wildcardUser, target: namespaceTarget()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := env.clients[tt.user].Create(env.ctx, newCapture("create-check", tt.target), client.DryRunAll)
			if tt.deniedVerb == "" {
				require.NoError(t, err)
				return
			}
			require.True(t, isPolicyDenial(err, tt.deniedVerb), "expected policy denial mentioning %q, got: %v", tt.deniedVerb, err)
		})
	}

	t.Run("cluster-admin nodeSelector and namespaceSelector", func(t *testing.T) {
		require.NoError(t, env.admin.Create(env.ctx, newCapture("admin-node", nodeTarget()), client.DryRunAll))
		require.NoError(t, env.admin.Create(env.ctx, newCapture("admin-namespace", namespaceTarget()), client.DryRunAll))
	})
}

// Each case creates its own Capture, so cases are independent and can be run
// individually with -run.
func testCaptureTargetPolicyUpdate(t *testing.T, env *captureTargetPolicyEnv) {
	tests := []struct {
		name   string
		target retinav1alpha1.CaptureTarget
		// createdBy is the user creating the Capture; empty means cluster-admin.
		createdBy string
		// finalizer pre-populates the operator's finalizer at creation.
		finalizer bool
		// deletedBy is the user who deletes the Capture before the update when
		// wantGone is set; empty means cluster-admin.
		deletedBy string
		actor     string
		mutate    func(*retinav1alpha1.Capture)
		// status sends the update to the status subresource instead.
		status     bool
		deniedVerb string
		// wantGone expects the Capture to be removed after the update (finalizer released).
		wantGone bool
	}{
		{
			name:   "operator adds finalizer to node-wide Capture",
			target: nodeTarget(), actor: operatorUser,
			mutate: func(c *retinav1alpha1.Capture) { c.Finalizers = append(c.Finalizers, captureFinalizer) },
		},
		{
			name:   "operator sets managed blob secret on node-wide Capture",
			target: nodeTarget(), actor: operatorUser,
			mutate: func(c *retinav1alpha1.Capture) { c.Spec.OutputConfiguration.BlobUpload = ptr.To("managed-blob-secret") },
		},
		{
			name:   "operator updates status of node-wide Capture",
			target: nodeTarget(), actor: operatorUser, status: true,
			mutate: func(c *retinav1alpha1.Capture) { c.Status.Active = 1 },
		},
		{
			name:   "tenant updates labels of node-wide Capture",
			target: nodeTarget(), actor: tenantUser,
			mutate: func(c *retinav1alpha1.Capture) { c.Labels = map[string]string{"team": "a"} },
		},
		{
			name:   "tenant changes blob secret of node-wide Capture",
			target: nodeTarget(), actor: tenantUser, deniedVerb: "capture-nodes",
			mutate: func(c *retinav1alpha1.Capture) { c.Spec.OutputConfiguration.BlobUpload = ptr.To("tenant-secret") },
		},
		{
			name:   "tenant changes duration of node-wide Capture",
			target: nodeTarget(), actor: tenantUser, deniedVerb: "capture-nodes",
			mutate: func(c *retinav1alpha1.Capture) {
				c.Spec.CaptureConfiguration.CaptureOption.Duration = &metav1.Duration{Duration: time.Hour}
			},
		},
		{
			name:   "node-capturer changes blob secret of node-wide Capture",
			target: nodeTarget(), actor: nodeCapturerUser,
			mutate: func(c *retinav1alpha1.Capture) { c.Spec.OutputConfiguration.BlobUpload = ptr.To("other-secret") },
		},
		{
			name:   "operator changes nodeSelector (operator updates are exempt)",
			target: nodeTarget(), actor: operatorUser,
			mutate: func(c *retinav1alpha1.Capture) {
				c.Spec.CaptureConfiguration.CaptureTarget.NodeSelector.MatchLabels["kubernetes.io/hostname"] = "node-1"
			},
		},
		{
			name:   "look-alike operator ServiceAccount in tenant namespace is not exempt",
			target: nodeTarget(), actor: lookalikeOperatorUser, deniedVerb: "capture-nodes",
			mutate: func(c *retinav1alpha1.Capture) { c.Spec.OutputConfiguration.BlobUpload = ptr.To("tenant-secret") },
		},
		{
			name:   "tenant changes nodeSelector",
			target: nodeTarget(), actor: tenantUser, deniedVerb: "capture-nodes",
			mutate: func(c *retinav1alpha1.Capture) {
				c.Spec.CaptureConfiguration.CaptureTarget.NodeSelector.MatchLabels["kubernetes.io/hostname"] = "victim-node"
			},
		},
		{
			name:   "node-capturer changes nodeSelector",
			target: nodeTarget(), actor: nodeCapturerUser,
			mutate: func(c *retinav1alpha1.Capture) {
				c.Spec.CaptureConfiguration.CaptureTarget.NodeSelector.MatchLabels["kubernetes.io/hostname"] = "node-1"
			},
		},
		{
			name:   "tenant changes podSelector of cross-namespace Capture",
			target: namespaceTarget(), actor: tenantUser, deniedVerb: "capture-namespaces",
			mutate: func(c *retinav1alpha1.Capture) {
				c.Spec.CaptureConfiguration.CaptureTarget.PodSelector = &metav1.LabelSelector{MatchLabels: map[string]string{"app": "payments"}}
			},
		},
		{
			name:   "tenant changes hostPath of cross-namespace Capture",
			target: namespaceTarget(), actor: tenantUser, deniedVerb: "capture-namespaces",
			mutate: func(c *retinav1alpha1.Capture) { c.Spec.OutputConfiguration.HostPath = ptr.To("elsewhere") },
		},
		{
			name:   "operator adds finalizer to cross-namespace Capture",
			target: namespaceTarget(), actor: operatorUser,
			mutate: func(c *retinav1alpha1.Capture) { c.Finalizers = append(c.Finalizers, captureFinalizer) },
		},
		{
			name:   "tenant updates labels of cross-namespace Capture",
			target: namespaceTarget(), actor: tenantUser,
			mutate: func(c *retinav1alpha1.Capture) { c.Labels = map[string]string{"team": "a"} },
		},
		{
			name:   "node-capturer changes namespaceSelector",
			target: namespaceTarget(), actor: nodeCapturerUser, deniedVerb: "capture-namespaces",
			mutate: func(c *retinav1alpha1.Capture) {
				c.Spec.CaptureConfiguration.CaptureTarget.NamespaceSelector.MatchLabels["team"] = "c"
			},
		},
		{
			name:   "namespace-capturer changes namespaceSelector",
			target: namespaceTarget(), actor: namespaceCapturerUser,
			mutate: func(c *retinav1alpha1.Capture) {
				c.Spec.CaptureConfiguration.CaptureTarget.NamespaceSelector.MatchLabels["team"] = "c"
			},
		},
		{
			name:   "tenant changes podSelector of pod-scoped Capture",
			target: podTarget(), createdBy: tenantUser, actor: tenantUser,
			mutate: func(c *retinav1alpha1.Capture) {
				c.Spec.CaptureConfiguration.CaptureTarget.PodSelector = &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}}
			},
		},
		{
			name:   "tenant changes output of pod-scoped Capture",
			target: podTarget(), createdBy: tenantUser, actor: tenantUser,
			mutate: func(c *retinav1alpha1.Capture) { c.Spec.OutputConfiguration.BlobUpload = ptr.To("tenant-secret") },
		},
		{
			name:   "tenant turns pod-scoped Capture into node-wide",
			target: podTarget(), createdBy: tenantUser, actor: tenantUser, deniedVerb: "capture-nodes",
			mutate: func(c *retinav1alpha1.Capture) { c.Spec.CaptureConfiguration.CaptureTarget = nodeTarget() },
		},
		{
			name:   "tenant turns pod-scoped Capture into cross-namespace",
			target: podTarget(), createdBy: tenantUser, actor: tenantUser, deniedVerb: "capture-namespaces",
			mutate: func(c *retinav1alpha1.Capture) { c.Spec.CaptureConfiguration.CaptureTarget = namespaceTarget() },
		},
		{
			name:   "operator removes finalizer from deleted node-wide Capture",
			target: nodeTarget(), finalizer: true, actor: operatorUser, wantGone: true,
			mutate: func(c *retinav1alpha1.Capture) { c.Finalizers = nil },
		},
		// DELETE is not matched by the policy: a tenant may always stop a Capture,
		// even one they could not have created.
		{
			name:   "tenant deletes cross-namespace Capture and operator finalizes it",
			target: namespaceTarget(), finalizer: true, deletedBy: tenantUser, actor: operatorUser, wantGone: true,
			mutate: func(c *retinav1alpha1.Capture) { c.Finalizers = nil },
		},
	}

	for i := range tests {
		tt := &tests[i]
		t.Run(tt.name, func(t *testing.T) {
			capture := newCapture("", tt.target)
			capture.GenerateName = "update-"
			if tt.finalizer {
				capture.Finalizers = []string{captureFinalizer}
			}
			require.NoError(t, env.client(tt.createdBy).Create(env.ctx, capture))

			if tt.wantGone {
				require.NoError(t, env.client(tt.deletedBy).Delete(env.ctx, capture))
			}

			err := env.updateCapture(tt.actor, capture.Name, tt.status, func(c *retinav1alpha1.Capture) {
				if tt.wantGone {
					require.NotNil(t, c.DeletionTimestamp, "Capture should be terminating")
				}
				tt.mutate(c)
			})
			if tt.deniedVerb != "" {
				require.True(t, isPolicyDenial(err, tt.deniedVerb), "expected policy denial mentioning %q, got: %v", tt.deniedVerb, err)
				return
			}
			require.NoError(t, err)

			if tt.wantGone {
				err := env.admin.Get(env.ctx, client.ObjectKeyFromObject(capture), &retinav1alpha1.Capture{})
				require.True(t, apierrors.IsNotFound(err), "Capture should be gone after finalizer removal, got: %v", err)
			}
		})
	}
}

// client returns the impersonating client for user, or the cluster-admin client when user is empty.
func (env *captureTargetPolicyEnv) client(user string) client.Client {
	if user == "" {
		return env.admin
	}
	return env.clients[user]
}

func (env *captureTargetPolicyEnv) updateCapture(user, name string, status bool, mutate func(*retinav1alpha1.Capture)) error {
	c := &retinav1alpha1.Capture{}
	if err := env.client(user).Get(env.ctx, client.ObjectKey{Namespace: tenantNamespace, Name: name}, c); err != nil {
		return fmt.Errorf("getting Capture %s as %q: %w", name, user, err)
	}
	mutate(c)
	if status {
		if err := env.client(user).Status().Update(env.ctx, c); err != nil {
			return fmt.Errorf("updating status of Capture %s as %q: %w", name, user, err)
		}
		return nil
	}
	if err := env.client(user).Update(env.ctx, c); err != nil {
		return fmt.Errorf("updating Capture %s as %q: %w", name, user, err)
	}
	return nil
}

// isPolicyDenial reports whether err is a denial by the capture target scope
// policy (as opposed to, e.g., an RBAC denial) for the given verb. It matches the
// message defined in the chart's policy rather than the apiserver's wording.
func isPolicyDenial(err error, verb string) bool {
	return apierrors.IsForbidden(err) &&
		strings.Contains(err.Error(), "requires the cluster-scoped '"+verb+"' verb on captures.retina.sh")
}

// A Capture applied with an explicit empty field (here `podNames: []`) is stored
// with that field, but the operator's typed client drops it on Update
// (omitempty), so its finalizer updates also change the spec. The operator must
// still be able to add and remove its finalizer.
func testCaptureTargetPolicyOperatorDroppedFields(t *testing.T, env *captureTargetPolicyEnv) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": retinav1alpha1.GroupVersion.String(),
		"kind":       "Capture",
		"metadata":   map[string]any{"generateName": "empty-fields-", "namespace": tenantNamespace},
		"spec": map[string]any{
			"captureConfiguration": map[string]any{
				"captureTarget": map[string]any{
					"nodeSelector": map[string]any{"matchLabels": map[string]any{"kubernetes.io/os": "linux"}},
					"podNames":     []any{},
				},
			},
			"outputConfiguration": map[string]any{"hostPath": "captures"},
		},
	}}
	require.NoError(t, env.admin.Create(env.ctx, obj))

	stored := &unstructured.Unstructured{}
	stored.SetGroupVersionKind(obj.GroupVersionKind())
	require.NoError(t, env.admin.Get(env.ctx, client.ObjectKeyFromObject(obj), stored))
	podNames, found, err := unstructured.NestedSlice(stored.Object, "spec", "captureConfiguration", "captureTarget", "podNames")
	require.NoError(t, err)
	require.True(t, found, "precondition: the API server must store the explicit empty podNames")
	require.Empty(t, podNames)

	require.NoError(t, env.updateCapture(operatorUser, obj.GetName(), false, func(c *retinav1alpha1.Capture) {
		encoded, marshalErr := json.Marshal(c.Spec.CaptureConfiguration.CaptureTarget)
		require.NoError(t, marshalErr)
		require.NotContains(t, string(encoded), "podNames", "precondition: the typed object must serialize without the empty podNames")
		c.Finalizers = append(c.Finalizers, captureFinalizer)
	}))

	require.NoError(t, env.admin.Delete(env.ctx, obj))
	require.NoError(t, env.updateCapture(operatorUser, obj.GetName(), false, func(c *retinav1alpha1.Capture) {
		require.NotNil(t, c.DeletionTimestamp, "Capture should be terminating")
		c.Finalizers = nil
	}))
	err = env.admin.Get(env.ctx, client.ObjectKeyFromObject(obj), &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": retinav1alpha1.GroupVersion.String(), "kind": "Capture",
	}})
	require.True(t, apierrors.IsNotFound(err), "Capture should be gone after finalizer removal, got: %v", err)
}

func newCapture(name string, target retinav1alpha1.CaptureTarget) *retinav1alpha1.Capture {
	return &retinav1alpha1.Capture{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: tenantNamespace},
		Spec: retinav1alpha1.CaptureSpec{
			CaptureConfiguration: retinav1alpha1.CaptureConfiguration{CaptureTarget: target},
			OutputConfiguration:  retinav1alpha1.OutputConfiguration{HostPath: ptr.To("captures")},
		},
	}
}

func nodeTarget() retinav1alpha1.CaptureTarget {
	return retinav1alpha1.CaptureTarget{
		NodeSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/os": "linux"}},
	}
}

func namespaceTarget() retinav1alpha1.CaptureTarget {
	return retinav1alpha1.CaptureTarget{
		NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"team": "b"}},
		PodSelector:       &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
	}
}

func podTarget() retinav1alpha1.CaptureTarget {
	return retinav1alpha1.CaptureTarget{
		PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
	}
}

func roleBinding(name, clusterRole string, users ...string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: tenantNamespace},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: clusterRole},
		Subjects:   userSubjects(users),
	}
}

func clusterRoleBinding(name, clusterRole string, users ...string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: clusterRole},
		Subjects:   userSubjects(users),
	}
}

func userSubjects(users []string) []rbacv1.Subject {
	subjects := make([]rbacv1.Subject, 0, len(users))
	for _, u := range users {
		subjects = append(subjects, rbacv1.Subject{Kind: rbacv1.UserKind, APIGroup: rbacv1.GroupName, Name: u})
	}
	return subjects
}
