# Setup

This page provides the instructions on how to install Retina via Helm.

The assumption is that a Kubernetes cluster has already been created and we have credentials to access it.

>NOTE: In case you want to test out Retina quickly and you have no clusters, you can quickly create one with [kind](https://kind.sigs.k8s.io/)

```sh
$ kind create cluster --name test-retina
Creating cluster "test-retina" ...
 ✓ Ensuring node image (kindest/node:v1.31.0) 🖼
 ✓ Preparing nodes 📦
 ✓ Writing configuration 📜
 ✓ Starting control-plane 🕹️
 ✓ Installing CNI 🔌
 ✓ Installing StorageClass 💾
Set kubectl context to "kind-test-retina"
You can now use your cluster with:

kubectl cluster-info --context kind-test-retina

Not sure what to do next? 😅  Check out https://kind.sigs.k8s.io/docs/user/quick-start/
```

## Requirements

- Helm version >= v3.8.0
- Access to a Kubernetes cluster via `kubectl`

## Control Plane and Modes

The installation of Retina can be configured using different control planes and modes.

You can choose between the "Standard" control plane (the original implementation of Retina) and Hubble.

If the "Standard" control plane is chosen, different modes are available. The available metric dimensions depend on the selected mode. For an explanation of the available modes, see [Standard Metric Modes](../03-Metrics/modes/modes.md).

Modes are not applicable to the Hubble control plane. For metrics related to the Hubble control plane, refer to the [Hubble metrics](../03-Metrics/02-hubble_metrics.md) documentation.

## Capture Support

In order to support the use of the [Capture CRD](../05-Concepts/CRDs/Capture.md), the Retina operator pod needs to be running.

Enable the operator with the `--set operator.enabled=true \` flag.

For example, this is how you could install Retina with the standard control plane and basic metric mode, with the operator.

```shell
VERSION=$( curl -sL https://api.github.com/repos/microsoft/retina/releases/latest | jq -r .name)
helm upgrade --install retina oci://ghcr.io/microsoft/retina/charts/retina \
    --version $VERSION \
    --namespace kube-system \
    --set image.tag=$VERSION \
    --set operator.tag=$VERSION \
    --set logLevel=info \
    --set operator.enabled=true \
    --set enabledPlugin_linux="\[dropreason\,packetforward\,linuxutil\,dns\]"
```

It is worth noting that even without the Retina operator pod running, captures can still be triggered with the [CLI](../04-Captures/02-cli.md).

## Installation

### Standard Control Plane

#### Basic Mode

```shell
VERSION=$( curl -sL https://api.github.com/repos/microsoft/retina/releases/latest | jq -r .name)
helm upgrade --install retina oci://ghcr.io/microsoft/retina/charts/retina \
    --version $VERSION \
    --namespace kube-system \
    --set image.tag=$VERSION \
    --set operator.tag=$VERSION \
    --set logLevel=info \
    --set enabledPlugin_linux="\[dropreason\,packetforward\,linuxutil\,dns\]"
```

#### Advanced Mode with Remote Context

```shell
VERSION=$( curl -sL https://api.github.com/repos/microsoft/retina/releases/latest | jq -r .name)
helm upgrade --install retina oci://ghcr.io/microsoft/retina/charts/retina \
    --version $VERSION \
    --namespace kube-system \
    --set image.tag=$VERSION \
    --set operator.tag=$VERSION \
    --set image.pullPolicy=Always \
    --set logLevel=info \
    --set os.windows=true \
    --set operator.enabled=true \
    --set operator.enableRetinaEndpoint=true \
    --skip-crds \
    --set enabledPlugin_linux="\[dropreason\,packetforward\,linuxutil\,dns\,packetparser\]" \
    --set enablePodLevel=true \
    --set remoteContext=true
```

#### Advanced Mode with Local Context

```shell
VERSION=$( curl -sL https://api.github.com/repos/microsoft/retina/releases/latest | jq -r .name)
helm upgrade --install retina oci://ghcr.io/microsoft/retina/charts/retina \
    --version $VERSION \
    --namespace kube-system \
    --set image.tag=$VERSION \
    --set operator.tag=$VERSION \
    --set image.pullPolicy=Always \
    --set logLevel=info \
    --set os.windows=true \
    --set operator.enabled=true \
    --set operator.enableRetinaEndpoint=true \
    --skip-crds \
    --set enabledPlugin_linux="\[dropreason\,packetforward\,linuxutil\,dns\,packetparser\]" \
    --set enablePodLevel=true \
    --set enableAnnotations=true
```

### Hubble Control Plane

```shell
VERSION=$( curl -sL https://api.github.com/repos/microsoft/retina/releases/latest | jq -r .name)
helm upgrade --install retina oci://ghcr.io/microsoft/retina/charts/retina-hubble \
        --version $VERSION \
        --namespace kube-system \
        --set os.windows=true \
        --set operator.enabled=true \
        --set operator.repository=ghcr.io/microsoft/retina/retina-operator \
        --set operator.tag=$VERSION \
        --set agent.enabled=true \
        --set agent.repository=ghcr.io/microsoft/retina/retina-agent \
        --set agent.tag=$VERSION \
        --set agent.init.enabled=true \
        --set agent.init.repository=ghcr.io/microsoft/retina/retina-init \
        --set agent.init.tag=$VERSION \
        --set logLevel=info \
        --set hubble.tls.enabled=false \
        --set hubble.relay.tls.server.enabled=false \
        --set hubble.tls.auto.enabled=false \
        --set hubble.tls.auto.method=cronJob \
        --set hubble.tls.auto.certValidityDuration=1 \
        --set hubble.tls.auto.schedule="*/10 * * * *"
```
