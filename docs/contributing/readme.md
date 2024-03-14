# Contributing

This project welcomes contributions and suggestions.  Most contributions require you to agree to a
Contributor License Agreement (CLA) declaring that you have the right to, and actually do, grant us
the rights to use your contribution. For details, visit [https://cla.opensource.microsoft.com](https://cla.opensource.microsoft.com).

When you submit a pull request, a CLA bot will automatically determine whether you need to provide
a CLA and decorate the PR appropriately (e.g., status check, comment). Simply follow the instructions
provided by the bot. You will only need to do this once across all repos using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or
contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.

## Development

### Configurations

Configurations are passed through `retina-config` configmap in `retina` namespace. Following configurations are currently supported:

- `apiserver.port` : the port for `retina-agent` Pod
- `logLevel` : supported - `debug`, `info`, `error`, `warn`, `panic`, `fatal`
- `enabledPlugin` : eBPF plugins to be installed in worker node. Currently supported plugins are `dropreason` and `packetforward`
- `metricsInterval` : interval, in seconds, to scrape/publish metrics

Note: Changes to configmap after retina is deployed would require re-deployment of `retina-agent`.

See the [Configuration](https://retina.sh/docs/metrics/configuration) page for further details.

### Supported Metrics Plugins

See the [Plugins](https://retina.sh/docs/metrics/plugins/packetforward) pages for a list of supported plugins.

### Pre-Requisites

```bash
export LLVM_VERSION=14
curl -sL https://apt.llvm.org/llvm.sh  | sudo bash -s "$LLVM_VERSION"
```

Download [Helm](https://helm.sh/) as well.

### Test

```bash
make retina-ut # run unit-test locally
make retina-test-image # run tests in docker container
```

### Build

Generate all mocks and BPF programs:

```bash
make all
```

Build the Retina binary:

```bash
make retina
```

To build a `retina-agent` container image with specific tag:

```bash
TAG=<tag> make retina-image
```

To build binary of a plugin and test it

```bash
# Test packetforward.
$ cd <Retina_repository>/test/plugin/packetforward
$ 
$ go build . && sudo ./packetforward
info    metrics Metrics initialized
info    packetforward   Packet forwarding metric initialized
info    packetforward   Start collecting packet forward metrics
info    test-packetforward      Started packetforward logger
error   packetforward   Error reading hash map  {"error": "lookup: key does not exist"}
debug   packetforward   Received PacketForward data     {"Data": "IngressBytes:302 IngressPackets:4 EgressBytes:11062 EgressPackets:33"}
debug   packetforward   Received PacketForward data     {"Data": "IngressBytes:898 IngressPackets:12 EgressBytes:11658 EgressPackets:41"}
debug   packetforward   Received PacketForward data     {"Data": "IngressBytes:898 IngressPackets:12 EgressBytes:23808 EgressPackets:70"}
...
```

### Deploying Locally on Kind

```bash
make kind-setup # This deploys a Kind cluster and installs azure NPM
make retina-image && make kind-install # Skip building image if already done
make kind-clean # Delete Kind cluster
```

### Deploying on Other Kubernetes Cluster

1. Create Kubernetes cluster.
2. Build and push the docker image for Retina: `make retina-image-push IMAGE_REGISTRY=<your-image-registry>`
3. Install Retina: `make helm-install IMAGE_REGISTRY=<your-image-registry>`

### Verify Deployment

Check `Retina` deployment and logs

```bash
$ kubectl -n kube-system get po
NAME                                                     READY   STATUS    RESTARTS   AGE
retina-agent-kq54d                                       1/1     Running   0          88s
...
$
$ k -n kube-system logs retina-agent-kq54d -f
info    main    Reading config ...
info    main    Initializing metrics
info    metrics Metrics initialized
info    main    Initializing Kubernetes client-go ...
info    controller-manager      Initializing controller manager ...
info    plugin-manager  Initializing plugin manager ...
info    packetforward   Packet forwarding metric initialized
...
info    dropreason      Start listening for drop reason events...
info    packetforward   Start collecting packet forward metrics
debug   packetforward   Received PacketForward data     {"Data": "IngressBytes:24688994 IngressPackets:6786 EgressBytes:370647 EgressPackets:4153"}
...
```

### Metrics

Retina generates `Prometheus` metrics and exposes them on port `10093` with path `/metrics`.

```bash
$ kubectl port-forward retina-agent-wzjld 9090:10093 -n kube-system  2>&1 >/dev/null &
$
$ ps aux | grep '[p]ort-forward'
anubhab   614516  0.0  0.1 759000 41796 pts/3    Sl+  14:34   0:00 kubectl port-forward retina-agent-wzjld 9090:10093 -n kube-system
$
$ curl http://localhost:9090/metrics | grep retina
...
# HELP retina_drop_bytes Total dropped bytes
# TYPE retina_drop_bytes gauge
retina_drop_bytes{direction="unknown",reason="IPTABLE_RULE_DROP"} 480
# HELP retina_drop_count Total dropped packets
# TYPE retina_drop_count gauge
retina_drop_count{direction="unknown",reason="IPTABLE_RULE_DROP"} 12
# HELP retina_forward_bytes Total forwarded bytes
# TYPE retina_forward_bytes gauge
retina_forward_bytes{direction="egress"} 1.28357355e+08
retina_forward_bytes{direction="ingress"} 3.9520696e+08
# HELP retina_forward_count Total forwarded packets
# TYPE retina_forward_count gauge
retina_forward_count{direction="egress"} 126462
retina_forward_count{direction="ingress"} 156793
```

### Dashboard/Prometheus/Grafana

Install `Prometheus` and `Grafana` onto the cluster to visualize metrics.

Documentation for these technologies:

- [Prometheus](https://prometheus.io/docs/introduction/overview/)
- [Grafana](https://grafana.com/grafana/)

### Cleanup

Uninstall `Retina`:

```bash
helm uninstall retina -n kube-system
```

## Contact

[Retina Devs](mailto:retina@microsoft.com)
