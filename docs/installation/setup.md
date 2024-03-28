# Setup

Clone the [Retina repo](https://github.com/microsoft/retina), then run a `make` command below for your scenario.

Note: you can also run captures with just the [CLI](./cli.md).

## Installation

When installing with helm, substitute the `version` and image `tag` arguments to the desired version, if different.

### Basic Mode

```shell
helm upgrade --install retina oci://ghcr.io/microsoft/retina/charts/retina \
    --version v0.0.2 \
    --namespace kube-system \
    --set image.tag=v0.0.2 \
    --set operator.tag=v0.0.2 \
    --set logLevel=info \
    --set enabledPlugin_linux="\[dropreason\,packetforward\,linuxutil\,dns\]"
```

### Basic Mode (with Capture support)

```shell
helm upgrade --install retina oci://ghcr.io/microsoft/retina/charts/retina \
    --version v0.0.2 \
    --namespace kube-system \
    --set image.tag=v0.0.2 \
    --set operator.tag=v0.0.2 \
    --set logLevel=info \
    --set image.pullPolicy=Always \
    --set logLevel=info \
    --set os.windows=true \
    --set operator.enabled=true \
    --set operator.enableRetinaEndpoint=true \
    --set operator.repository=$(IMAGE_REGISTRY)/$(RETINA_OPERATOR_IMAGE) \
    --skip-crds \
    --set enabledPlugin_linux="\[dropreason\,packetforward\,linuxutil\,dns\,packetparser\]"
```

### Advanced Mode with Remote Context (with Capture support)

See [Metric Modes](../metrics/modes.md).

```shell
helm upgrade --install retina oci://ghcr.io/microsoft/retina/charts/retina \
    --version v0.0.2 \
    --namespace kube-system \
    --set image.tag=v0.0.2 \
    --set operator.tag=v0.0.2 \
    --set image.pullPolicy=Always \
    --set logLevel=info \
    --set os.windows=true \
    --set operator.enabled=true \
    --set operator.enableRetinaEndpoint=true \
    --set operator.repository=$(IMAGE_REGISTRY)/$(RETINA_OPERATOR_IMAGE) \
    --skip-crds \
    --set enabledPlugin_linux="\[dropreason\,packetforward\,linuxutil\,dns\,packetparser\]" \
    --set enablePodLevel=true \
    --set remoteContext=true
```

### Advanced Mode with Local Context (with Capture support)

See [Metric Modes](../metrics/modes.md).

```shell
helm upgrade --install retina oci://ghcr.io/microsoft/retina/charts/retina \
    --version v0.0.2 \
    --namespace kube-system \
    --set image.tag=v0.0.2 \
    --set operator.tag=v0.0.2 \
    --set image.pullPolicy=Always \
    --set logLevel=info \
    --set os.windows=true \
    --set operator.enabled=true \
    --set operator.enableRetinaEndpoint=true \
    --set operator.repository=$(IMAGE_REGISTRY)/$(RETINA_OPERATOR_IMAGE) \
    --skip-crds \
    --set enabledPlugin_linux="\[dropreason\,packetforward\,linuxutil\,dns\,packetparser\]" \
    --set enablePodLevel=true \
    --set enableAnnotations=true
```

## Next Steps: Configuring Prometheus/Grafana

- [Unmanaged Prometheus/Grafana](./prometheus-unmanaged.md)
