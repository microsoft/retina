package hubble

import (
	"net/http"

	k8s "github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/microsoft/retina/test/e2e/framework/types"
)

func ValidateHubbleRelayService() *types.Scenario {
	name := "Validate Hubble Relay Service"
	steps := []*types.StepWrapper{
		{
			Step: &k8s.ValidateResource{
				ResourceName:      "hubble-relay-service",
				ResourceNamespace: k8s.HubbleNamespace,
				ResourceType:      k8s.ResourceTypeService,
				Labels:            "k8s-app=" + k8s.HubbleRelayApp,
			},
		},
	}

	return types.NewScenario(name, steps...)
}

func ValidateHubbleUIService(kubeConfigFilePath string) *types.Scenario {
	name := "Validate Hubble UI Services"
	steps := []*types.StepWrapper{
		{
			Step: &k8s.ValidateResource{
				ResourceName:      k8s.HubbleUIApp,
				ResourceNamespace: k8s.HubbleNamespace,
				ResourceType:      k8s.ResourceTypeService,
				Labels:            "k8s-app=" + k8s.HubbleUIApp,
			},
		},
		{
			Step: &k8s.PortForward{
				LabelSelector:         "k8s-app=hubble-ui",
				LocalPort:             "8080",
				RemotePort:            "8081",
				OptionalLabelAffinity: "k8s-app=hubble-ui",
				Endpoint:              "?namespace=default", // set as default namespace query since endpoint can't be nil
				KubeConfigFilePath:    kubeConfigFilePath,
			},
		},
		{
			Step: &k8s.ValidateHTTPResponse{
				URL:            "http://localhost:8080",
				ExpectedStatus: http.StatusOK,
			},
		},
	}

	return types.NewScenario(name, steps...)
}
