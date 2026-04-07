// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package kind

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/microsoft/retina/test/e2ev3/pkg/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
)

const npmManifestURL = "https://raw.githubusercontent.com/Azure/azure-container-networking/master/npm/azure-npm.yaml"

// InstallNPM applies Azure Network Policy Manager to enable NetworkPolicy
// enforcement on Kind clusters.
type InstallNPM struct {
	KubeConfigFilePath string
}

func (n *InstallNPM) String() string { return "install-azure-npm" }

func (n *InstallNPM) Do(ctx context.Context) error {
	ctx, log := utils.StepLogger(ctx, n)
	log.Info("installing Azure NPM for NetworkPolicy enforcement")

	rc, err := clientcmd.BuildConfigFromFlags("", n.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("failed to build rest config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(rc)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Fetch the manifest.
	resp, err := http.Get(npmManifestURL) //nolint:noctx // simple one-shot fetch
	if err != nil {
		return fmt.Errorf("failed to fetch NPM manifest: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read NPM manifest: %w", err)
	}

	// Decode and apply each resource in the multi-doc YAML.
	decoder := scheme.Codecs.UniversalDeserializer()
	reader := yaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(body)))
	for {
		doc, readErr := reader.Read()
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return fmt.Errorf("failed to read YAML document: %w", readErr)
		}
		doc = bytes.TrimSpace(doc)
		if len(doc) == 0 {
			continue
		}

		obj, _, decodeErr := decoder.Decode(doc, nil, nil)
		if decodeErr != nil {
			return fmt.Errorf("failed to decode YAML document: %w", decodeErr)
		}

		if err := applyResource(ctx, log, obj, clientset); err != nil {
			return fmt.Errorf("failed to apply NPM resource: %w", err)
		}
	}

	// Wait for the DaemonSet pods to be ready.
	log.Info("waiting for Azure NPM DaemonSet to be ready")
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		ds, getErr := clientset.AppsV1().DaemonSets("kube-system").Get(ctx, "azure-npm", metav1.GetOptions{})
		if getErr != nil {
			return false, nil //nolint:nilerr // retry on transient errors
		}
		ready := ds.Status.DesiredNumberScheduled > 0 && ds.Status.NumberReady == ds.Status.DesiredNumberScheduled
		if !ready {
			log.Info("waiting for DaemonSet rollout",
				"ready", ds.Status.NumberReady, "desired", ds.Status.DesiredNumberScheduled)
		}
		return ready, nil
	})
	if err != nil {
		return fmt.Errorf("Azure NPM DaemonSet not ready: %w", err)
	}

	log.Info("Azure NPM installed successfully")
	return nil
}

// applyResource creates or updates a single Kubernetes resource.
func applyResource(ctx context.Context, log *slog.Logger, obj runtime.Object, cs *kubernetes.Clientset) error {
	switch o := obj.(type) {
	case *corev1.ServiceAccount:
		log.Info("applying ServiceAccount", "name", o.Name, "namespace", o.Namespace)
		_, err := cs.CoreV1().ServiceAccounts(o.Namespace).Create(ctx, o, metav1.CreateOptions{})
		if errors.IsAlreadyExists(err) {
			_, err = cs.CoreV1().ServiceAccounts(o.Namespace).Update(ctx, o, metav1.UpdateOptions{})
		}
		return err
	case *rbacv1.ClusterRole:
		log.Info("applying ClusterRole", "name", o.Name)
		_, err := cs.RbacV1().ClusterRoles().Create(ctx, o, metav1.CreateOptions{})
		if errors.IsAlreadyExists(err) {
			_, err = cs.RbacV1().ClusterRoles().Update(ctx, o, metav1.UpdateOptions{})
		}
		return err
	case *rbacv1.ClusterRoleBinding:
		log.Info("applying ClusterRoleBinding", "name", o.Name)
		_, err := cs.RbacV1().ClusterRoleBindings().Create(ctx, o, metav1.CreateOptions{})
		if errors.IsAlreadyExists(err) {
			_, err = cs.RbacV1().ClusterRoleBindings().Update(ctx, o, metav1.UpdateOptions{})
		}
		return err
	case *appsv1.DaemonSet:
		log.Info("applying DaemonSet", "name", o.Name, "namespace", o.Namespace)
		_, err := cs.AppsV1().DaemonSets(o.Namespace).Create(ctx, o, metav1.CreateOptions{})
		if errors.IsAlreadyExists(err) {
			_, err = cs.AppsV1().DaemonSets(o.Namespace).Update(ctx, o, metav1.UpdateOptions{})
		}
		return err
	case *corev1.Service:
		log.Info("applying Service", "name", o.Name, "namespace", o.Namespace)
		_, err := cs.CoreV1().Services(o.Namespace).Create(ctx, o, metav1.CreateOptions{})
		if errors.IsAlreadyExists(err) {
			_, err = cs.CoreV1().Services(o.Namespace).Update(ctx, o, metav1.UpdateOptions{})
		}
		return err
	case *corev1.ConfigMap:
		log.Info("applying ConfigMap", "name", o.Name, "namespace", o.Namespace)
		_, err := cs.CoreV1().ConfigMaps(o.Namespace).Create(ctx, o, metav1.CreateOptions{})
		if errors.IsAlreadyExists(err) {
			_, err = cs.CoreV1().ConfigMaps(o.Namespace).Update(ctx, o, metav1.UpdateOptions{})
		}
		return err
	default:
		return fmt.Errorf("unsupported resource type: %T", obj)
	}
}
