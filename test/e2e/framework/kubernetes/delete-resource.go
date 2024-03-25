package kubernetes

import (
	"context"
	"fmt"
	"log"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var ErrDeleteNilResource = fmt.Errorf("cannot create nil resource")

type ResourceType string

const (
	DaemonSet          ResourceType = "DaemonSet"
	Deployment         ResourceType = "Deployment"
	StatefulSet        ResourceType = "StatefulSet"
	Service            ResourceType = "Service"
	ServiceAccount     ResourceType = "ServiceAccount"
	Role               ResourceType = "Role"
	RoleBinding        ResourceType = "RoleBinding"
	ClusterRole        ResourceType = "ClusterRole"
	ClusterRoleBinding ResourceType = "ClusterRoleBinding"
	ConfigMap          ResourceType = "ConfigMap"
	NetworkPolicy      ResourceType = "NetworkPolicy"
	Secret             ResourceType = "Secret"
	Unknown            ResourceType = "Unknown"
)

// Parameters can only be strings, heres to help add guardrails
func TypeString(resourceType ResourceType) string {
	ResourceTypes := map[ResourceType]string{
		DaemonSet:          "DaemonSet",
		Deployment:         "Deployment",
		StatefulSet:        "StatefulSet",
		Service:            "Service",
		ServiceAccount:     "ServiceAccount",
		Role:               "Role",
		RoleBinding:        "RoleBinding",
		ClusterRole:        "ClusterRole",
		ClusterRoleBinding: "ClusterRoleBinding",
		ConfigMap:          "ConfigMap",
		NetworkPolicy:      "NetworkPolicy",
		Secret:             "Secret",
		Unknown:            "Unknown",
	}
	str, ok := ResourceTypes[resourceType]
	if !ok {
		return ResourceTypes[Unknown]
	}
	return str
}

type DeleteKubernetesResource struct {
	ResourceType       string // can't use enum, breaks parameter parsing, all must be strings
	ResourceName       string
	ResourceNamespace  string
	KubeConfigFilePath string
}

func (d *DeleteKubernetesResource) Run() error {
	config, err := clientcmd.BuildConfigFromFlags("", d.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutSeconds*time.Second)
	defer cancel()

	res := ResourceType(d.ResourceType)

	var resource runtime.Object

	switch res {
	case DaemonSet:
		resource = &appsv1.DaemonSet{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      d.ResourceName,
				Namespace: d.ResourceNamespace,
			},
		}
	case Deployment:
		resource = &appsv1.Deployment{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      d.ResourceName,
				Namespace: d.ResourceNamespace,
			},
		}
	case StatefulSet:
		resource = &appsv1.StatefulSet{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      d.ResourceName,
				Namespace: d.ResourceNamespace,
			},
		}
	case Service:
		resource = &v1.Service{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      d.ResourceName,
				Namespace: d.ResourceNamespace,
			},
		}
	case ServiceAccount:
		resource = &v1.ServiceAccount{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      d.ResourceName,
				Namespace: d.ResourceNamespace,
			},
		}
	case Role:
		resource = &rbacv1.Role{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      d.ResourceName,
				Namespace: d.ResourceNamespace,
			},
		}
	case RoleBinding:
		resource = &rbacv1.RoleBinding{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      d.ResourceName,
				Namespace: d.ResourceNamespace,
			},
		}
	case ClusterRole:
		resource = &rbacv1.ClusterRole{
			ObjectMeta: metaV1.ObjectMeta{
				Name: d.ResourceName,
			},
		}
	case ClusterRoleBinding:
		resource = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metaV1.ObjectMeta{
				Name: d.ResourceName,
			},
		}
	case ConfigMap:
		resource = &v1.ConfigMap{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      d.ResourceName,
				Namespace: d.ResourceNamespace,
			},
		}
	case NetworkPolicy:
		resource = &networkingv1.NetworkPolicy{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      d.ResourceName,
				Namespace: d.ResourceNamespace,
			},
		}
	case Secret:
		resource = &v1.Secret{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      d.ResourceName,
				Namespace: d.ResourceNamespace,
			},
		}
	case Unknown:
		return fmt.Errorf("unknown resource type: %s: %w", d.ResourceType, ErrUnknownResourceType)
	default:
		return ErrUnknownResourceType
	}

	err = DeleteResource(ctx, resource, clientset)
	if err != nil {
		return fmt.Errorf("error deleting resource: %w", err)
	}

	return nil
}

func (d *DeleteKubernetesResource) Stop() error {
	return nil
}

func (d *DeleteKubernetesResource) Prevalidate() error {
	restype := ResourceType(d.ResourceType)
	if restype == Unknown {
		return ErrUnknownResourceType
	}

	return nil
}

func DeleteResource(ctx context.Context, obj runtime.Object, clientset *kubernetes.Clientset) error { //nolint:gocyclo //this is just boilerplate code
	if obj == nil {
		return ErrCreateNilResource
	}

	switch o := obj.(type) {
	case *appsv1.DaemonSet:
		log.Printf("Deleting DaemonSet \"%s\" in namespace \"%s\"...\n", o.Name, o.Namespace)
		client := clientset.AppsV1().DaemonSets(o.Namespace)
		err := client.Delete(ctx, o.Name, metaV1.DeleteOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				log.Printf("DaemonSet \"%s\" in namespace \"%s\" does not exist\n", o.Name, o.Namespace)
				return nil
			}
			return fmt.Errorf("failed to delete DaemonSet \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
		}

	case *appsv1.Deployment:
		log.Printf("Deleting Deployment \"%s\" in namespace \"%s\"...\n", o.Name, o.Namespace)
		client := clientset.AppsV1().Deployments(o.Namespace)
		err := client.Delete(ctx, o.Name, metaV1.DeleteOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				log.Printf("Deployment \"%s\" in namespace \"%s\" does not exist\n", o.Name, o.Namespace)
				return nil
			}
			return fmt.Errorf("failed to delete Deployment \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
		}

	case *appsv1.StatefulSet:
		log.Printf("Deleting StatefulSet \"%s\" in namespace \"%s\"...\n", o.Name, o.Namespace)
		client := clientset.AppsV1().StatefulSets(o.Namespace)
		err := client.Delete(ctx, o.Name, metaV1.DeleteOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				log.Printf("StatefulSet \"%s\" in namespace \"%s\" does not exist\n", o.Name, o.Namespace)
				return nil
			}
			return fmt.Errorf("failed to delete StatefulSet \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
		}

	case *v1.Service:
		log.Printf("Deleting Service \"%s\" in namespace \"%s\"...\n", o.Name, o.Namespace)
		client := clientset.CoreV1().Services(o.Namespace)
		err := client.Delete(ctx, o.Name, metaV1.DeleteOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				log.Printf("Service \"%s\" in namespace \"%s\" does not exist\n", o.Name, o.Namespace)
				return nil
			}
			return fmt.Errorf("failed to delete Service \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
		}

	case *v1.ServiceAccount:
		log.Printf("Deleting ServiceAccount \"%s\" in namespace \"%s\"...\n", o.Name, o.Namespace)
		client := clientset.CoreV1().ServiceAccounts(o.Namespace)
		err := client.Delete(ctx, o.Name, metaV1.DeleteOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				log.Printf("ServiceAccount \"%s\" in namespace \"%s\" does not exist\n", o.Name, o.Namespace)
				return nil
			}
			return fmt.Errorf("failed to delete ServiceAccount \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
		}

	case *rbacv1.Role:
		log.Printf("Deleting Role \"%s\" in namespace \"%s\"...\n", o.Name, o.Namespace)
		client := clientset.RbacV1().Roles(o.Namespace)
		err := client.Delete(ctx, o.Name, metaV1.DeleteOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				log.Printf("Role \"%s\" in namespace \"%s\" does not exist\n", o.Name, o.Namespace)
				return nil
			}
			return fmt.Errorf("failed to delete Role \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
		}

	case *rbacv1.RoleBinding:
		log.Printf("Deleting RoleBinding \"%s\" in namespace \"%s\"...\n", o.Name, o.Namespace)
		client := clientset.RbacV1().RoleBindings(o.Namespace)
		err := client.Delete(ctx, o.Name, metaV1.DeleteOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				log.Printf("RoleBinding \"%s\" in namespace \"%s\" does not exist\n", o.Name, o.Namespace)
				return nil
			}
			return fmt.Errorf("failed to delete RoleBinding \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
		}

	case *rbacv1.ClusterRole:
		log.Printf("Deleting ClusterRole \"%s\"...\n", o.Name)
		client := clientset.RbacV1().ClusterRoles()
		err := client.Delete(ctx, o.Name, metaV1.DeleteOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				log.Printf("ClusterRole \"%s\" in namespace \"%s\" does not exist\n", o.Name, o.Namespace)
				return nil
			}
			return fmt.Errorf("failed to delete ClusterRole \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
		}

	case *rbacv1.ClusterRoleBinding:
		log.Printf("Deleting ClusterRoleBinding \"%s\"...\n", o.Name)
		client := clientset.RbacV1().ClusterRoleBindings()
		err := client.Delete(ctx, o.Name, metaV1.DeleteOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				log.Printf("ClusterRoleBinding \"%s\" in namespace \"%s\" does not exist\n", o.Name, o.Namespace)
				return nil
			}
			return fmt.Errorf("failed to delete ClusterRoleBinding \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
		}

	case *v1.ConfigMap:
		log.Printf("Deleting ConfigMap \"%s\" in namespace \"%s\"...\n", o.Name, o.Namespace)
		client := clientset.CoreV1().ConfigMaps(o.Namespace)
		err := client.Delete(ctx, o.Name, metaV1.DeleteOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				log.Printf("ConfigMap \"%s\" in namespace \"%s\" does not exist\n", o.Name, o.Namespace)
				return nil
			}
			return fmt.Errorf("failed to delete ConfigMap \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
		}

	case *networkingv1.NetworkPolicy:
		log.Printf("Deleting NetworkPolicy \"%s\" in namespace \"%s\"...\n", o.Name, o.Namespace)
		client := clientset.NetworkingV1().NetworkPolicies(o.Namespace)
		err := client.Delete(ctx, o.Name, metaV1.DeleteOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				log.Printf("NetworkPolicy \"%s\" in namespace \"%s\" does not exist\n", o.Name, o.Namespace)
				return nil
			}
			return fmt.Errorf("failed to delete NetworkPolicy \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
		}

	case *v1.Secret:
		log.Printf("Deleting Secret \"%s\" in namespace \"%s\"...\n", o.Name, o.Namespace)
		client := clientset.CoreV1().Secrets(o.Namespace)
		err := client.Delete(ctx, o.Name, metaV1.DeleteOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				log.Printf("Secret \"%s\" in namespace \"%s\" does not exist\n", o.Name, o.Namespace)
				return nil
			}
			return fmt.Errorf("failed to delete Secret \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
		}

	default:
		return fmt.Errorf("unknown object type: %T, err: %w", obj, ErrUnknownResourceType)
	}
	return nil
}
