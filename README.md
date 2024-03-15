# Retina

[![goreport][goreport-img]][goreport] ![GitHub release][release-img] [![retina-publish][godoc-badge]][godoc] ![license] [![retina-test][retina-test-image-badge]][retina-test-image] [![retinash][retinash-badge]][retinash] [![retina-publish][retina-publish-badge]][retina-publish] [![retina-publish-ghcr][retina-publish-ghcr-badge]][retina-publish-ghcr] ![retina-codeql-img][retina-codeql-badge] ![retina-golangci-lint-img][retina-golangci-lint-badge]

## Overview

Retina is a cloud and vendor agnostic container workload observability platform which helps customers with enterprise grade DevOps, SecOps and compliance use cases. It is designed to cater to cluster network administrators, cluster security administrators and DevOps engineers by providing a centralized platform for monitoring application and network health, and security. Retina is capable of collecting telemetry data from multiple sources and aggregating it into a single time-series database. Retina is also capable of sending data to multiple destinations, such as Prometheus, Azure Monitor, and other vendors, and visualizing the data in a variety of ways, like Grafana, Azure Monitor, Azure log analytics, and more.

![Retina features](/site/static/img/retina-features.png)

## Documentation

See [retina.sh](http://retina.sh) for more information and examples.

## Capabilities

Retina is currently supported in AKS. It has two major features:

### Metrics

[Read more](https://retina.sh/docs/metrics/modes)

### Quick Install Guide

1. Create a Kubernetes cluster with a minimum of 2 nodes. Retina supports Linux (Ubuntu) and Windows (2019 and 2022) nodes.
2. Follow steps in [Using Managed Prometheus and Grafana](https://retina.sh/docs/installation/prometheus-azure-managed)

### Captures

[Read more](https://retina.sh/docs/captures)

## Contributing

[Read more](https://retina.sh/docs/contributing)

## Office Hours and Community Meetings

[Read more](https://retina.sh/docs/contributing/#office-hours-and-community-meetings)

This project welcomes contributions and suggestions.  Most contributions require you to agree to a
Contributor License Agreement (CLA) declaring that you have the right to, and actually do, grant us
the rights to use your contribution. For details, visit <https://cla.opensource.microsoft.com>.

When you submit a pull request, a CLA bot will automatically determine whether you need to provide
a CLA and decorate the PR appropriately (e.g., status check, comment). Simply follow the instructions
provided by the bot. You will only need to do this once across all repos using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or
contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.

## Trademarks

This project may contain trademarks or logos for projects, products, or services. Authorized use of Microsoft
trademarks or logos is subject to and must follow [Microsoft's Trademark & Brand Guidelines](https://www.microsoft.com/en-us/legal/intellectualproperty/trademarks/usage/general).
Use of Microsoft trademarks or logos in modified versions of this project must not cause confusion or imply Microsoft sponsorship.
Any use of third-party trademarks or logos are subject to those third-party's policies.

## License

See [LICENSE](LICENSE).

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
[retina-publish-ghcr-badge]: https://github.com/microsoft/retina/actions/workflows/images-ghcr.yaml/badge.svg?branch=main
[retina-publish-ghcr]: https://github.com/microsoft/retina/actions/workflows/images-ghcr.yaml?query=branch%3Amain
[retina-codeql-badge]: https://github.com/microsoft/retina/actions/workflows/codeql.yaml/badge.svg?branch=main
[retina-golangci-lint-badge]: https://github.com/microsoft/retina/actions/workflows/golangci-lint.yaml/badge.svg?branch=main
