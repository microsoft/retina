package deploy

import (
	"context"
	_ "embed"
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

const (
	RetinaCapturesYAMLpath       = "retina.sh_captures.yaml"
	RetinaEndpointsYAMLpath      = "retina.sh_retinaendpoints.yaml"
	MetricsConfigurationYAMLpath = "retina.sh_metricsconfigurations.yaml"
	TracesConfigurationYAMLpath  = "retina.sh_tracesconfigurations.yaml"
)

//go:embed manifests/controller/helm/retina/crds/retina.sh_captures.yaml
var RetinaCapturesYAML []byte

//go:embed manifests/controller/helm/retina/crds/retina.sh_retinaendpoints.yaml
var RetinaEndpointsYAML []byte

//go:embed manifests/controller/helm/retina/crds/retina.sh_metricsconfigurations.yaml
var MetricsConfgurationYAML []byte

//go:embed manifests/controller/helm/retina/crds/retina.sh_tracesconfigurations.yaml
var TracesConfigurationYAML []byte

func GetRetinaCapturesCRD() (*apiextensionsv1.CustomResourceDefinition, error) {
	retinaCapturesCRD := &apiextensionsv1.CustomResourceDefinition{}
	if err := yaml.Unmarshal(RetinaCapturesYAML, &retinaCapturesCRD); err != nil {
		fmt.Println("error unmarshalling embedded retinacaptures")
		fmt.Println(err.Error())
		return nil, errors.Wrap(err, "error unmarshalling embedded retinacaptures")
	}
	return retinaCapturesCRD, nil
}

func GetRetinaEndpointCRD() (*apiextensionsv1.CustomResourceDefinition, error) {
	retinaEndpointCRD := &apiextensionsv1.CustomResourceDefinition{}
	if err := yaml.Unmarshal(RetinaEndpointsYAML, &retinaEndpointCRD); err != nil {
		return nil, errors.Wrap(err, "error unmarshalling embedded retinaendpoints")
	}
	return retinaEndpointCRD, nil
}

func GetRetinaMetricsConfigurationCRD() (*apiextensionsv1.CustomResourceDefinition, error) {
	retinaMetricsConfigurationCRD := &apiextensionsv1.CustomResourceDefinition{}
	if err := yaml.Unmarshal(MetricsConfgurationYAML, &retinaMetricsConfigurationCRD); err != nil {
		return nil, errors.Wrap(err, "error unmarshalling embedded metricsconfiguration")
	}
	return retinaMetricsConfigurationCRD, nil
}

func GetRetinaTracesConfigurationCRD() (*apiextensionsv1.CustomResourceDefinition, error) {
	retinaTracesConfigurationCRD := &apiextensionsv1.CustomResourceDefinition{}
	if err := yaml.Unmarshal(TracesConfigurationYAML, &retinaTracesConfigurationCRD); err != nil {
		return nil, errors.Wrap(err, "error unmarshalling embedded tracesconfiguration")
	}
	return retinaTracesConfigurationCRD, nil
}

func InstallOrUpdateCRDs(
	ctx context.Context,
	enableRetinaEndpoint bool,
	apiExtensionsClient apiextv1.ApiextensionsV1Interface,
	enableTrace bool,
) (map[string]*apiextensionsv1.CustomResourceDefinition, error) {
	crds := make(map[string]*apiextensionsv1.CustomResourceDefinition, 4)

	retinaCapture, err := GetRetinaCapturesCRD()
	if err != nil {
		return nil, err
	}
	crds[retinaCapture.GetObjectMeta().GetName()] = retinaCapture

	if enableRetinaEndpoint {
		retinaEndpoint, err := GetRetinaEndpointCRD()
		if err != nil {
			return nil, err
		}
		crds[retinaEndpoint.GetObjectMeta().GetName()] = retinaEndpoint
	}

	retinaMetricsConfiguration, err := GetRetinaMetricsConfigurationCRD()
	if err != nil {
		return nil, err
	}
	crds[retinaMetricsConfiguration.GetObjectMeta().GetName()] = retinaMetricsConfiguration

	if enableTrace {
		retinaTracesConfiguration, err := GetRetinaTracesConfigurationCRD()
		if err != nil {
			return nil, err
		}
		crds[retinaTracesConfiguration.GetObjectMeta().GetName()] = retinaTracesConfiguration
	}

	for name, crd := range crds {
		current, err := apiExtensionsClient.CustomResourceDefinitions().Create(ctx, crd, v1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			crds[name] = current
			continue
		}

		if !reflect.DeepEqual(crd.Spec.Versions, current.Spec.Versions) {
			crd.SetResourceVersion(current.GetResourceVersion())
			current, err = apiExtensionsClient.CustomResourceDefinitions().Update(ctx, crd, v1.UpdateOptions{})
			if err != nil {
				// on error, return the failed CRD
				return crds, errors.Wrapf(err, "failed to update %s CRD", name)
			}
			crds[name] = current
		}
	}

	return crds, nil
}
