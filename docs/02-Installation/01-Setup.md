# Setup

Install the helm chart below for your scenario.

Note: you can also run captures with just the [CLI](./02-CLI.md).

## Installation

Requires Helm version >= v3.8.0. The installation of Retina depends control plane and mode. The choice of control plane is between "legacy", which is the original implementation of Retina control plane and Hubble. The metrics dimensions available depend on the mode selection, see [Legacy Metric Modes](../03-Metrics/modes/modes.md) for an explaination of available mode. Modes are available for "legacy" control plane only. For Hubble control plane metrics, see [Hubble metrics](../03-Metrics/02-hubble_metrics.md) documentation.

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

### Hubble control plane

```shell
VERSION=$( curl -sL https://api.github.com/repos/microsoft/retina/releases/latest | jq -r .name)
helm upgrade --install retina oci://ghcr.io/microsoft/retina/charts/retina-hubble \
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

## Next Steps: Configuring Prometheus and Grafana

- [Prometheus](./04-prometheus.md)
- [Grafana](./05-grafana.md)
