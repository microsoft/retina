package kubernetes

import (
	"context"
	"fmt"
	"log"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

var (
	ErrUnknownResourceType = fmt.Errorf("unknown resource type")
	ErrCreateNilResource   = fmt.Errorf("cannot create nil resource")
)

func CreateResource(ctx context.Context, obj runtime.Object, clientset *kubernetes.Clientset) error { //nolint:gocyclo //this is just boilerplate code
	if obj == nil {
		return ErrCreateNilResource
	}

	switch o := obj.(type) {
	case *appsv1.DaemonSet:
		log.Printf("Creating/Updating DaemonSet \"%s\" in namespace \"%s\"...\n", o.Name, o.Namespace)
		client := clientset.AppsV1().DaemonSets(o.Namespace)
		_, err := client.Get(ctx, o.Name, metaV1.GetOptions{})
		if errors.IsNotFound(err) {
			_, err = client.Create(ctx, o, metaV1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create DaemonSet \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
			}
			return nil
		}
		_, err = client.Update(ctx, o, metaV1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create/update DaemonSet \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
		}

	case *appsv1.Deployment:
		log.Printf("Creating/Updating Deployment \"%s\" in namespace \"%s\"...\n", o.Name, o.Namespace)
		client := clientset.AppsV1().Deployments(o.Namespace)
		_, err := client.Get(ctx, o.Name, metaV1.GetOptions{})
		if errors.IsNotFound(err) {
			_, err = client.Create(ctx, o, metaV1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create Deployment \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
			}
			return nil
		}
		_, err = client.Update(ctx, o, metaV1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create/update Deployment \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
		}

	case *appsv1.StatefulSet:
		log.Printf("Creating/Updating StatefulSet \"%s\" in namespace \"%s\"...\n", o.Name, o.Namespace)
		client := clientset.AppsV1().StatefulSets(o.Namespace)
		_, err := client.Get(ctx, o.Name, metaV1.GetOptions{})
		if errors.IsNotFound(err) {
			_, err = client.Create(ctx, o, metaV1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create StatefulSet \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
			}
			return nil
		}
		_, err = client.Update(ctx, o, metaV1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create/update StatefulSet \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
		}

	case *v1.Service:
		log.Printf("Creating/Updating Service \"%s\" in namespace \"%s\"...\n", o.Name, o.Namespace)
		client := clientset.CoreV1().Services(o.Namespace)
		_, err := client.Get(ctx, o.Name, metaV1.GetOptions{})
		if errors.IsNotFound(err) {
			_, err = client.Create(ctx, o, metaV1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create Service \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
			}
			return nil
		}
		_, err = client.Update(ctx, o, metaV1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create/update Service \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
		}

	case *v1.ServiceAccount:
		log.Printf("Creating/Updating ServiceAccount \"%s\" in namespace \"%s\"...\n", o.Name, o.Namespace)
		client := clientset.CoreV1().ServiceAccounts(o.Namespace)
		_, err := client.Get(ctx, o.Name, metaV1.GetOptions{})
		if errors.IsNotFound(err) {
			_, err = client.Create(ctx, o, metaV1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create ServiceAccount \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
			}
			return nil
		}
		_, err = client.Update(ctx, o, metaV1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create/update ServiceAccount \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
		}

	case *rbacv1.Role:
		log.Printf("Creating/Updating Role \"%s\" in namespace \"%s\"...\n", o.Name, o.Namespace)
		client := clientset.RbacV1().Roles(o.Namespace)
		_, err := client.Get(ctx, o.Name, metaV1.GetOptions{})
		if errors.IsNotFound(err) {
			_, err = client.Create(ctx, o, metaV1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create Role \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
			}
			return nil
		}
		_, err = client.Update(ctx, o, metaV1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create/update Role \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
		}

	case *rbacv1.RoleBinding:
		log.Printf("Creating/Updating RoleBinding \"%s\" in namespace \"%s\"...\n", o.Name, o.Namespace)
		client := clientset.RbacV1().RoleBindings(o.Namespace)
		_, err := client.Get(ctx, o.Name, metaV1.GetOptions{})
		if errors.IsNotFound(err) {
			_, err = client.Create(ctx, o, metaV1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create RoleBinding \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
			}
			return nil
		}
		_, err = client.Update(ctx, o, metaV1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create/update RoleBinding \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
		}

	case *rbacv1.ClusterRole:
		log.Printf("Creating/Updating ClusterRole \"%s\"...\n", o.Name)
		client := clientset.RbacV1().ClusterRoles()
		_, err := client.Get(ctx, o.Name, metaV1.GetOptions{})
		if errors.IsNotFound(err) {
			_, err = client.Create(ctx, o, metaV1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create ClusterRole \"%s\": %w", o.Name, err)
			}
			return nil
		}
		_, err = client.Update(ctx, o, metaV1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create/update ClusterRole \"%s\": %w", o.Name, err)
		}

	case *rbacv1.ClusterRoleBinding:
		log.Printf("Creating/Updating ClusterRoleBinding \"%s\"...\n", o.Name)
		client := clientset.RbacV1().ClusterRoleBindings()
		_, err := client.Get(ctx, o.Name, metaV1.GetOptions{})
		if errors.IsNotFound(err) {
			_, err = client.Create(ctx, o, metaV1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create ClusterRoleBinding \"%s\": %w", o.Name, err)
			}
			return nil
		}
		_, err = client.Update(ctx, o, metaV1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create/update ClusterRoleBinding \"%s\": %w", o.Name, err)
		}

	case *v1.ConfigMap:
		log.Printf("Creating/Updating ConfigMap \"%s\" in namespace \"%s\"...\n", o.Name, o.Namespace)
		client := clientset.CoreV1().ConfigMaps(o.Namespace)
		_, err := client.Get(ctx, o.Name, metaV1.GetOptions{})
		if errors.IsNotFound(err) {
			_, err = client.Create(ctx, o, metaV1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create ConfigMap \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
			}
			return nil
		}
		_, err = client.Update(ctx, o, metaV1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create/update ConfigMap \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
		}

	case *networkingv1.NetworkPolicy:
		log.Printf("Creating/Updating NetworkPolicy \"%s\" in namespace \"%s\"...\n", o.Name, o.Namespace)
		client := clientset.NetworkingV1().NetworkPolicies(o.Namespace)
		_, err := client.Get(ctx, o.Name, metaV1.GetOptions{})
		if errors.IsNotFound(err) {
			_, err = client.Create(ctx, o, metaV1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create NetworkPolicy \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
			}
			return nil
		}
		_, err = client.Update(ctx, o, metaV1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create/update NetworkPolicy \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
		}

	case *v1.Secret:
		log.Printf("Creating/Updating Secret \"%s\" in namespace \"%s\"...\n", o.Name, o.Namespace)
		client := clientset.CoreV1().Secrets(o.Namespace)
		_, err := client.Get(ctx, o.Name, metaV1.GetOptions{})
		if errors.IsNotFound(err) {
			_, err = client.Create(ctx, o, metaV1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create Secret \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
			}
			return nil
		}
		_, err = client.Update(ctx, o, metaV1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create/update Secret \"%s\" in namespace \"%s\": %w", o.Name, o.Namespace, err)
		}

	default:
		return fmt.Errorf("unknown object type: %T, err: %w", obj, ErrUnknownResourceType)
	}
	return nil
}
