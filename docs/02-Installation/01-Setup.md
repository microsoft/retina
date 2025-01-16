# Setup

Install the helm chart below for your scenario.

Note: you can also run captures with just the [CLI](./02-CLI.md).

## Installation

### Requirements

Helm version >= v3.8.0.
Docker and Helm login to GitHub Container Registry. [Authenticating to GHCR](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry#authenticating-to-the-container-registry)

### Basic Mode

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

### Basic Mode (with Capture support)

```shell
VERSION=$( curl -sL https://api.github.com/repos/microsoft/retina/releases/latest | jq -r .name)
helm upgrade --install retina oci://ghcr.io/microsoft/retina/charts/retina \
    --version $VERSION \
    --namespace kube-system \
    --set image.tag=$VERSION \
    --set operator.tag=$VERSION \
    --set logLevel=info \
    --set image.pullPolicy=Always \
    --set logLevel=info \
    --set os.windows=true \
    --set operator.enabled=true \
    --set operator.enableRetinaEndpoint=true \
    --skip-crds \
    --set enabledPlugin_linux="\[dropreason\,packetforward\,linuxutil\,dns\,packetparser\]"
```

### Advanced Mode with Remote Context (with Capture support)

See [Metric Modes](../03-Metrics/modes/modes.md).

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

### Advanced Mode with Local Context (with Capture support)

See [Metric Modes](../03-Metrics/modes/modes.md).

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

## Next Steps: Configuring Prometheus and Grafana

- [Prometheus](./04-prometheus.md)
- [Grafana](./05-grafana.md)
