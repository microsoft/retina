// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package telemetry

import (
	kcfg "sigs.k8s.io/controller-runtime/pkg/client/config"
)

// GetK8SApiserverURLFromKubeConfig returns apiserver URL from kubeconfig.
// The apiserver URL is expected to be publicly unique identifier of the Kubernetes cluster.
// In case the kubeconfig does not exists, this identifier can be obtained for pods from kube-system namespace.
// Kubelet will populate env KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT for Pods deployed in kube-system
// namespace, which can be used to generate apiserver URL from `GetConfig()`.
func GetK8SApiserverURLFromKubeConfig() (string, error) {
	cfg, err := kcfg.GetConfig()
	if err != nil {
		return "", err
	}
	return cfg.Host, nil
}
