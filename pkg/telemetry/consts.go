// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package telemetry

const (
	// PropertyApiVersion is the property name of the telemetry item that contains the APIServer endpoint.
	// apiserver is used to distinguish the telemetry items from different Kubernetes clusters and should be uniformed
	// across all the telemetry items from the same cluster with the format http(s)://<host>:<port>.
	// For example "https://retina-test-c4528d-zn0ugsna.hcp.southeastasia.azmk8s.io:443
	PropertyApiserver = "apiserver"
)

const (
	EnvPodName = "POD_NAME"
)
