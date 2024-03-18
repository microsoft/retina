# Retina

[![goreport][goreport-img]][goreport] ![GitHub release][release-img] [![retina-publish][godoc-badge]][godoc] ![license]

[![retina-test][retina-test-image-badge]][retina-test-image] [![retinash][retinash-badge]][retinash] [![retina-publish][retina-publish-badge]][retina-publish] ![retina-codeql-img][retina-codeql-badge] ![retina-golangci-lint-img][retina-golangci-lint-badge]

## Overview

Retina is a cloud-agnostic, open-source **Kubernetes network observability platform** that provides a **centralized hub for monitoring application health, network health, and security**. It provides actionable insights to cluster network administrators, cluster security administrators, and DevOps engineers navigating DevOps, SecOps, and compliance use cases.

Retina **collects customizable telemetry**, which can be exported to **multiple storage options** (such as Prometheus, Azure Monitor, and other vendors) and **visualized in a variety of ways** (like Grafana, Azure Log Analytics, and other vendors).

## Features

- **[eBPF](https://ebpf.io/what-is-ebpf#what-is-ebpf)-based** Network Observability platform for Kubernetes workloads.
- **On-Demand** and **Configurable**.
- Actionable, industry-standard **Prometheus metrics**.
- Streamlined **Packet Captures** for deep dives.
- **Cloud-agnostic**, supporting multiple OS (like Linux, Windows, Azure Linux).

## Why Retina?

Retina lets you **investigate network issues on-demand** and **continuously monitor your clusters**. For scenarios where Retina shines, see the intro docs [here](https://retina.sh/docs/intro)

## Documentation

See [retina.sh](http://retina.sh) for documentation and examples.

## Capabilities

Retina has two major features:

- [Metrics](https://retina.sh/docs/metrics/modes)
- [Captures](https://retina.sh/docs/captures)

### Metrics Quick Install Guide

Prerequisites: Go, Helm

1. Clone the repo, then install Retina on your Kubernetes cluster

    ```bash
    make helm-install
    ```

2. Follow steps in [Using Prometheus and Grafana](https://retina.sh/docs/installation/prometheus-unmanaged) to set up metrics collection and visualization.

### Captures Quick Start Guide

#### Captures via CLI

Currently, Retina CLI only supports Linux.

- Option 1: Download from Release

  Download `kubectl-retina` from the latest [Retina release](https://github.com/microsoft/retina/releases).
  Feel free to move the binary to `/usr/local/bin/`, or add it to your `PATH` otherwise.

- Option 2: Build from source

  Requirements:

  - go 1.21 or newer
  - GNU make

  Clone the Retina repo and execute:

  ```shell
  make install-kubectl-retina
  ```

Execute Retina:

```shell
kubectl-retina capture create --help
```

For further CLI documentation, see [Capture with Retina CLI](../captures/cli.md).

#### Captures via CRD

Prerequisites: Go, Helm

1. Clone the repo, then install Retina with Capture operator support on your Kubernetes cluster

    ```bash
    make helm-install-with-operator
    ```

2. Follow steps in [Capture CRD](https://retina.sh/docs/captures/#option-2-capture-crd-custom-resource-definition) for documentation of the CRD and examples for setting up Captures.

## Contributing

This project welcomes contributions and suggestions.  Most contributions require you to agree to a
Contributor License Agreement (CLA) declaring that you have the right to, and actually do, grant us
the rights to use your contribution. For details, visit <https://cla.opensource.microsoft.com>.

When you submit a pull request, a CLA bot will automatically determine whether you need to provide
a CLA and decorate the PR appropriately (e.g., status check, comment). Simply follow the instructions
provided by the bot. You will only need to do this once across all repos using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or
contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.

[Read more about how to begin contributing here.](https://retina.sh/docs/contributing)

### Office Hours and Community Meetings

We host a periodic open community meeting. [Read more here.](https://retina.sh/docs/contributing/#office-hours-and-community-meetings)

## Trademarks

This project may contain trademarks or logos for projects, products, or services. Authorized use of Microsoft
trademarks or logos is subject to and must follow [Microsoft's Trademark & Brand Guidelines](https://www.microsoft.com/en-us/legal/intellectualproperty/trademarks/usage/general).
Use of Microsoft trademarks or logos in modified versions of this project must not cause confusion or imply Microsoft sponsorship.
Any use of third-party trademarks or logos are subject to those third-party's policies.

## License

See the [LICENSE](LICENSE).

## Code of Conduct

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/). For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.

## Contact

"Retina Devs" <retina@microsoft.com>

[goreport-img]: https://goreportcard.com/badge/github.com/microsoft/retina
[goreport]: https://goreportcard.com/report/github.com/microsoft/retina
[godoc]: https://godoc.org/github.com/microsoft/retina
[godoc-badge]: https://godoc.org/github.com/microsoft/retina?status.svg
[release-img]: https://img.shields.io/github/v/release/microsoft/retina.svg
[license]: https://img.shields.io/badge/license-MIT-blue?link=https%3A%2F%2Fgithub.com%2Fmicrosoft%2Fretina%2Fblob%2Fmain%2FLICENSE
[retina-test-image-badge]: https://github.com/microsoft/retina/actions/workflows/test.yaml/badge.svg?branch=main
[retina-test-image]: https://github.com/microsoft/retina/actions/workflows/test.yaml?query=branch%3Amain
[retinash-badge]: https://github.com/microsoft/retina/actions/workflows/docs.yaml/badge.svg?branch=main
[retinash]: https://retina.sh/
[retina-publish-badge]: https://github.com/microsoft/retina/actions/workflows/images.yaml/badge.svg?branch=main
[retina-publish]: https://github.com/microsoft/retina/actions/workflows/images.yaml?query=branch%3Amain
[retina-codeql-badge]: https://github.com/microsoft/retina/actions/workflows/codeql.yaml/badge.svg?branch=main
[retina-golangci-lint-badge]: https://github.com/microsoft/retina/actions/workflows/golangci-lint.yaml/badge.svg?branch=main
