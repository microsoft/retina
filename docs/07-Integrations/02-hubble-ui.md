# Hubble UI

When Retina is [deployed with Hubble control plane](../02-Installation/01-Setup.md#hubble-control-plane), Hubble UI provides access to a graphical service map.

## Overview

[Hubble UI](https://github.com/cilium/hubble-ui) is a web-based graphical user interface for Hubble, providing a visual representation of network flows and service maps within your Kubernetes cluster. It offers a comprehensive visualization of your Kubernetes network, allowing you to:

- View and filter network flows in real-time
- Examine service dependency maps
- Diagnose network issues visually
- Monitor network policies and their effects

Hubble UI consists of two components:
1. A **backend** service that connects to Hubble Relay and processes the network flow data
2. A **frontend** web application that presents the data in an interactive interface

We covered the description of Hubble in the overview of [Hubble CLI](./01-hubble-cli.md). Both Hubble CLI and UI rely on the same Retina eBPF data plane to provide access to networking observability.

## Prerequisites

Before installing Hubble UI, ensure you have:

- A Kubernetes cluster
- Retina installed (see [Quick Installation](../02-Installation/01-Setup.md))
- Hubble enabled with Hubble Relay running

Hubble Relay is a required component as Hubble UI connects to it to retrieve flow data.

## Installation Steps

### Installing with Retina and Hubble Control Plane

The recommended way to install Hubble UI is via the OCI Helm chart provided with Retina:

1. Get the latest Retina version:

```shell
VERSION=$( curl -sL https://api.github.com/repos/microsoft/retina/releases/latest | jq -r .name)
helm upgrade --install retina oci://ghcr.io/microsoft/retina/charts/retina-hubble \
  --version $VERSION \
  --namespace kube-system \
  --set hubble.relay.enabled=true \
  --set hubble.ui.enabled=true \
  --set hubble.tls.enabled=false \
  --set hubble.relay.tls.server.enabled=false \
  --set hubble.tls.auto.enabled=false
```

> **Important**: When using the OCI Helm chart, you must use a valid version that exists in the repository. Using `$VERSION` as shown above will pull the latest release, but specifying a non-existent version like `v1.0.0` will result in a "not found" error.

### Installing with Cilium

While Cilium provides Hubble UI capabilities, the direct installation of Hubble UI as a standalone component requires Cilium to be installed in your cluster. The Microsoft Retina implementation does not have this dependency.

For Cilium users, you would typically enable Hubble UI as part of the Cilium installation:

2. Install or upgrade with Hubble UI enabled:

```shell
helm install cilium cilium/cilium \
  --namespace kube-system \
  --set hubble.relay.enabled=true \
  --set hubble.ui.enabled=true
```

> **Note**: The Cilium Helm repository doesn't contain a standalone `hubble-ui` chart. Hubble UI must be installed as part of the main Cilium installation.

### Using a Specific Version

If you want to install a specific version of the Retina Helm chart:

```shell
helm upgrade --install retina oci://ghcr.io/microsoft/retina/charts/retina-hubble \
  --version v1.0.0 \
  --namespace kube-system \
  --set hubble.relay.enabled=true \
  --set hubble.ui.enabled=true \
  --set hubble.tls.enabled=false \
  --set hubble.relay.tls.server.enabled=false \
  --set hubble.tls.auto.enabled=false
```

### Configuration Options

Hubble UI can be customized with the following Helm values:

| Parameter | Description | Default |
|-----------|-------------|---------|
| `hubble.ui.enabled` | Enable Hubble UI | `true` |
| `hubble.ui.replicas` | Number of UI replicas to deploy | `1` |
| `hubble.ui.service.type` | Service type (ClusterIP or NodePort) | `ClusterIP` |
| `hubble.ui.ingress.enabled` | Enable ingress for Hubble UI | `false` |
| `hubble.ui.standalone.enabled` | Deploy UI in standalone mode | `false` |

For more configuration options, see the [Helm values file](https://github.com/microsoft/retina/blob/main/deploy/hubble/manifests/controller/helm/retina/values.yaml).

## Verify Installation

After installation, verify that Hubble UI is running properly:

1. Check that the Hubble UI pods are running:

```shell
kubectl get pods -n kube-system -l k8s-app=hubble-ui
```

Expected output:
```
NAME                         READY   STATUS    RESTARTS   AGE
hubble-ui-788b95d4b5-2xsqm   2/2     Running   0          1m
```

2. Check that the Hubble UI service is available:

```shell
kubectl get svc -n kube-system -l k8s-app=hubble-ui
```

Expected output:
```
NAME        TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)    AGE
hubble-ui   ClusterIP   10.96.118.225   <none>        80/TCP     1m
```

## Example

To access Hubble UI from your local machine, port-forward Hubble UI service.

```sh
kubectl port-forward -n kube-system svc/hubble-ui 8081:80
```

Hubble UI should now be accessible on [http://localhost:8081](http://localhost:8081)

![Hubble UI home](./img/hubble-ui-home.png "Hubble UI home")

By selecting a specific namespace and adding the required label-based filtering and/or verdict, we can access the graphical service map and the flows table with detailed information regarding networking traffic for the specific selection.

![Hubble UI service map](./img/hubble-ui-service-map.png "Hubble UI service map")


## Troubleshooting

### 1. Hubble UI Cannot Connect to Hubble Relay

If Hubble UI cannot connect to Hubble Relay, check:

1. Ensure Hubble Relay is running:

```shell
kubectl get pods -n kube-system -l k8s-app=hubble-relay
```

2. Check Hubble UI backend logs for connection errors:

```shell
kubectl logs -n kube-system -l k8s-app=hubble-ui -c backend
```

3. Restart the Hubble UI with Relay

```shell
cilium hubble disable

# Enable Hubble with relay and ui paramater
cilium hubble enable --relay --ui
```

### 2. TLS Issues in Standalone Mode

When using standalone mode with TLS enabled for Hubble Relay:

1. Ensure you've configured the certificates volume correctly.
2. Verify the secrets exist and are properly formatted.

## Get Support

If you continue to experience issues with Hubble UI:

- Check the [Retina troubleshooting guide](../06-Troubleshooting/basic-metrics.md)
- Open an issue on the [Retina GitHub repository](https://github.com/microsoft/retina)
- Join the [Retina community discussions](https://github.com/microsoft/retina/discussions)
