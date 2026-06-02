# Development

This document provides steps to set up your dev environment and start contributing to the Retina project. You can find the complete documentation on [retina.sh](https://retina.sh)

## Quick start

Retina uses a forking workflow. To contribute, fork the repository and create a branch for your changes.

The easiest way to set up your Development Environment is to use the provided GitHub Codespaces configuration.

## Environment Config

Below is a list of required tools and dependencies you need to set up your local development environment for Retina.

- [Go](https://go.dev/doc/install)
- [Docker](https://docs.docker.com/engine/install/)
- [Helm](https://helm.sh/docs/intro/install)
- jq: `sudo apt install jq`
- If you want to use [ghcr.io](https://github.com/features/packages) as container registry, login following instructions on [authenticating with a personal access token](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry#authenticating-with-a-personal-access-token-classic)

Once you have set up your environment fork the repository and create a branch for your changes.

### LLVM/Clang Installation

To manually configure your DevEnv you will need `llvm-strip` and `clang`.

To install `clang`:

```bash
sudo apt install clang-16
```

To install `llvm-strip`:

```bash
export LLVM_VERSION=16
curl -sL https://apt.llvm.org/llvm.sh  | sudo bash -s "$LLVM_VERSION"
```

Test that `llvm-strip` and `clang` are in your `PATH`:

```bash
which clang
which llvm-strip
```

If these commands fail (there is no output), then see if you have a binary with the version as a suffix (e.g. `/usr/bin/clang-16` and `/usr/bin/llvm-strip-16`).
Then create a symbolic link to the versioned binary like:

```bash
sudo ln -s /usr/bin/clang-16 /usr/bin/clang
sudo ln -s /usr/bin/llvm-strip-16 /usr/bin/llvm-strip
```

## Building and Testing

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
make retina-image # also pushes to image registry
make retina-operator-image
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

### Test

```bash
make test # run unit-tests locally
make test-image # run tests in docker container
```

### Publishing Images and Charts

To publish images to GHCR in your forked repository, simply push to `main`, or publish a tag. The `container-publish` action will run automatically and push images to your GitHub packages registry.

These registries are private by default; to pull images from your registry anonymously, [navigate to "Package Settings" for each published image repository and set the visibility to "Public"](https://docs.github.com/en/packages/learn-github-packages/configuring-a-packages-access-control-and-visibility#configuring-access-to-packages-for-your-personal-account).

Alternatively, configure authenticated access to your registry using a [GitHub Personal Access Token](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry#authenticating-to-the-container-registry).

### Deploying on Kubernetes Cluster

1. Create Kubernetes cluster.
2. Install Retina using Helm:

   ```bash
   helm upgrade --install retina oci://ghcr.io/$YOUR_ORG/retina/charts/retina \
       --version $YOUR_VERSION \
        --set image.tag=$YOUR_VERSION \
        --set operator.tag=$YOUR_VERSION \
        --set logLevel=info \
        --set enabledPlugin_linux="\[dropreason\,packetforward\,linuxutil\,dns\]"
   ```

### Verify Deployment

Check `Retina` deployment and logs

```bash
$ kubectl -n kube-system get po
NAME                                                     READY   STATUS    RESTARTS   AGE
retina-agent-kq54d                                       1/1     Running   0          88s
...
$
$ kubectl -n kube-system logs retina-agent-kq54d -f
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
networkobservability_drop_bytes{direction="unknown",reason="IPTABLE_RULE_DROP"} 480
networkobservability_drop_count{direction="unknown",reason="IPTABLE_RULE_DROP"} 12
networkobservability_forward_bytes{direction="egress"} 1.28357355e+08
networkobservability_forward_bytes{direction="ingress"} 3.9520696e+08
networkobservability_forward_count{direction="egress"} 126462
networkobservability_forward_count{direction="ingress"} 156793
...
```

### Dashboard/Prometheus/Grafana

Install `Prometheus` and `Grafana` onto the cluster to visualize metrics.

Documentation for these technologies:

- [Prometheus](https://prometheus.io/docs/introduction/overview/)
- [Grafana](https://grafana.com/grafana/)

### Cleanup

Uninstall `Retina`:

```bash
make helm-uninstall
```

## Opening a Pull Request

When you're ready to open a pull request, please ensure that your branch is up-to-date with the `main` branch, updates relevant docs and tests, and passes all tests and lints.

### Cryptographic Signing of Commits

In order to certify the provenance of commits and defend against impersonation, we require that all commits be cryptographically signed.
Documentation for setting up Git and GitHub to sign your commits can be found in the [GitHub documentation on signing commits](https://docs.github.com/en/authentication/managing-commit-signature-verification/signing-commits).
Additional information about Git's use of GPG can be found in the [Git documentation on signing your work](https://git-scm.com/book/en/v2/Git-Tools-Signing-Your-Work)

> To configure your Git client to sign commits by default for a local repository, run `git config --add commit.gpgsign true`.

For **GitHub Codespaces** users, please follow [this doc](https://docs.github.com/en/codespaces/managing-your-codespaces/managing-gpg-verification-for-github-codespaces) to configure GitHub to automatically use GPG to sign commits you make in your Codespaces.

### Developer Certificate of Origin (DCO)

Contributions to Retina must contain a Developer Certificate of Origin within their constituent commits.
This can be accomplished by providing a `-s` flag to `git commit` as documented in the [Git commit documentation](https://git-scm.com/docs/git-commit#Documentation/git-commit.txt--s).
This will add a `Signed-off-by` trailer to your Git commit, affirming your acceptance of the Contributor License Agreement.

### Updating Documentation

The documentation available on [retina.sh](https://retina.sh) can be found within the [docs](https://github.com/microsoft/retina/tree/main/docs) folder in the repository.

The diagrams used are created with [Excalidraw](https://excalidraw.com/). The source `.excalidraw` files are stored within the repository, alongside their `.png` equivalent.

### GitHub issues and Good First Issue

You can find the open issues on the repo's [GitHub issues board](https://github.com/microsoft/retina/issues)
If you are a first-time contributor, you can find the issues that are suitable for newcomers by finding the [issues labeled as "good first issue"](https://github.com/microsoft/retina/labels/good%20first%20issue)
