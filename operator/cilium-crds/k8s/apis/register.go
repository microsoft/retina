// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Retina and Cilium

// NOTE: this file was copied and modified from Cilium's pkg/k8s/apis/cilium.io/client/register.go
// to create only the Cilium CRDs which are necessary for Retina (i.e. CiliumEndpoint and CiliumIdentity).

package apis

import (
	"context"
	"fmt"
	"log/slog"

	"golang.org/x/sync/errgroup"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	k8sconst "github.com/cilium/cilium/pkg/k8s/apis/cilium.io"
	apisclient "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/client"
	k8sconstv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	"github.com/cilium/cilium/pkg/k8s/apis/crdhelpers"
	"github.com/cilium/cilium/pkg/k8s/client"
	"github.com/cilium/cilium/pkg/k8s/synced"
	"github.com/cilium/cilium/pkg/versioncheck"
)

var necessaryCRDNames = []string{
	synced.CRDResourceName(k8sconstv2.CEPName),
	synced.CRDResourceName(k8sconstv2.CIDName),
}

// Define a custom error type for missing CRDs
type CRDNotFoundError struct {
	CRDName string
}

func (e *CRDNotFoundError) Error() string {
	return "CRD not found: " + e.CRDName
}

// RegisterCRDs registers all CRDs with the K8s apiserver.
func RegisterCRDs(clientset client.Clientset) error {
	if err := createCustomResourceDefinitions(clientset); err != nil {
		return fmt.Errorf("Unable to create custom resource definition: %w", err)
	}

	return nil
}

// createCustomResourceDefinitions creates our CRD objects in the Kubernetes
// cluster.
func createCustomResourceDefinitions(clientset apiextensionsclient.Interface) error {
	g, _ := errgroup.WithContext(context.Background())

	crds, err := customResourceDefinitionList()
	if err != nil {
		return fmt.Errorf("Unable to get CRD list: %w", err)
	}

	for _, crd := range crds {
		crd := crd
		g.Go(func() error {
			return createCRD(crd.Name, crd.FullName)(clientset)
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("Unable to create CRD: %w", err)
	}

	return nil
}

func customResourceDefinitionList() (map[string]*apisclient.CRDList, error) {
	crds := apisclient.CustomResourceDefinitionList()

	necessaryCRDs := make(map[string]*apisclient.CRDList, len(necessaryCRDNames))

	for _, crdName := range necessaryCRDNames {
		crd, ok := crds[crdName]
		if !ok {
			return nil, fmt.Errorf("%w", &CRDNotFoundError{CRDName: crdName})
		}

		necessaryCRDs[crdName] = crd
	}

	return necessaryCRDs, nil
}

// createCRD creates and updates a CRD.
// It should be called on agent startup but is idempotent and safe to call again.
func createCRD(crdVersionedName, crdMetaName string) func(clientset apiextensionsclient.Interface) error {
	return func(clientset apiextensionsclient.Interface) error {
		ciliumCRD := apisclient.GetPregeneratedCRD(slog.Default(), crdVersionedName)

		err := crdhelpers.CreateUpdateCRD(
			slog.Default(),
			clientset,
			constructV1CRD(crdMetaName, ciliumCRD),
			crdhelpers.NewDefaultPoller(),
			k8sconst.CustomResourceDefinitionSchemaVersionKey,
			versioncheck.MustVersion(k8sconst.CustomResourceDefinitionSchemaVersion),
		)
		if err != nil {
			return fmt.Errorf("Unable to create CRD %s: %w", crdMetaName, err)
		}
		return nil
	}
}

func constructV1CRD(
	name string,
	template apiextensionsv1.CustomResourceDefinition,
) *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				k8sconst.CustomResourceDefinitionSchemaVersionKey: k8sconst.CustomResourceDefinitionSchemaVersion,
			},
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: k8sconst.CustomResourceDefinitionGroup,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:       template.Spec.Names.Kind,
				Plural:     template.Spec.Names.Plural,
				ShortNames: template.Spec.Names.ShortNames,
				Singular:   template.Spec.Names.Singular,
			},
			Scope:    template.Spec.Scope,
			Versions: template.Spec.Versions,
		},
	}
}
