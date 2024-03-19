# Setup

Clone the [Retina repo](https://github.com/microsoft/retina), then run a `make` command below for your scenario.

Note: you can also run captures with just the [CLI](./cli.md).

## Installation

### Basic Mode

```shell
make helm-install
```

### Basic Mode (with Capture support)

```shell
make helm-install-with-operator
```

### Advanced Mode with Remote Context (with Capture support)

See [Metric Modes](../metrics/modes.md).

```shell
make helm-install-advanced-remote-context
```

### Advanced Mode with Local Context (with Capture support)

See [Metric Modes](../metrics/modes.md).

```shell
make helm-install-advanced-local-context
```

## Next Steps: Configuring Prometheus/Grafana

- [Unmanaged Prometheus/Grafana](./prometheus-unmanaged.md)
