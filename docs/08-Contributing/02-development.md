# Development

This document provides steps to set up your dev environment and start contributing to the Retina project. You can find the complete documentation on [retina.sh](https://retina.sh)

## Quick start

Retina uses a forking workflow. To contribute, fork the repository and create a branch for your changes.

### Using a devcontainer (recommended)

The easiest way to get started is to use the provided [devcontainer](https://github.com/microsoft/retina/blob/main/.devcontainer/devcontainer.json), which works with both [GitHub Codespaces](https://github.com/features/codespaces) and [VS Code Dev Containers](https://code.visualstudio.com/docs/devcontainers/containers).

The devcontainer comes pre-configured with all required tools:

- Go, clang/LLVM (for eBPF compilation), Docker, Helm, kubectl, Kind, Azure CLI, GitHub CLI, and jq
- Go modules are pre-downloaded; run `make generate` to compile eBPF programs and generate mocks before building
- A Kind cluster is created on startup for local testing
- VS Code is configured with golangci-lint and gofumpt so editor feedback matches CI

To launch in Codespaces, click **Code > Codespaces > New codespace** on the repository page. To use locally, open the repository in VS Code and select **Reopen in Container** from the command palette.

### Manual setup

If you prefer to set up your environment manually, see the requirements below.

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

#### Windows agent images

##### Local development from Linux or WSL

With Docker/Buildx and an existing GHCR login, build and push both Windows Server
2022 and 2025 development images:

```bash
make make-win TAG=dev

# Build and push only one Windows version.
make make-win YEAR=2025 TAG=dev

# Export archives instead of pushing.
make make-win TAG=dev BUILDX_ACTION=
```

Tags are `ghcr.io/<owner>/retina/retina-agent:dev-local-windows-ltsc<year>-amd64`.
Omit `TAG` for the same Git-derived base tag as Linux builds; override
`IMAGE_NAMESPACE` or `IMAGE_REGISTRY` to change the destination.
Archives are written to `artifacts/`, one per Windows version. A custom
`WINDOWS_LOCAL_ARCHIVE` requires selecting a single `YEAR`.

**Development only:** [Dockerfile.local](../../hack/windows/Dockerfile.local)
packages unsigned binaries without CVE-2013-3900 registry hardening, separately
from production build outputs. Linux can assemble but cannot run these Windows
containers; load an archive on a compatible Windows host with
`docker load --input <archive>`.

##### Native Windows packaging

On a matching Windows Docker host, package prebuilt binaries (signed for
production) without rebuilding or signing them:

```bash
make retina-image-win WINDOWS_YEARS=2025 TAG=local \
  REPO_PATH=. BINARIES_PATH=output/windows_amd64
```

To cross-compile the binaries separately, run `make build-windows-binaries`;
the default output is `output/windows_amd64`. Both Windows variants use the same
binaries, but their production Dockerfiles require native Windows registry commands.

##### Multi-platform manifests

After publishing production-style Windows and Linux images under the same base tag:

```bash
make manifest-retina-image TAG=<tag> WINDOWS_YEARS="2025 2022"
```

Keep WS2025 first: affected WS2025 containerd versions select the first Windows
variant even when `os.version` metadata is present.

For an AMD64-only development deployment, build all Linux components and both local
Windows variants, then publish their shared manifests:

```bash
export TAG=win25-test
make retina-image retina-operator-image retina-shell-image \
  kubectl-retina-image kubectl-retina-shell-image \
  PLATFORM=linux/amd64 GOARCH=amd64 OUTPUT_LOCAL= BUILDX_ACTION=--push
make make-win
for component in retina operator shell kubectl-retina kubectl-retina-shell; do
  make manifest COMPONENT="$component" LINUX_PLATFORMS=linux/amd64 \
    WINDOWS_YEARS="2025 2022" WINDOWS_TAG_PREFIX="$TAG-local"
done
```

`WINDOWS_TAG_PREFIX="$TAG-local"` selects development Windows images.
Deploy the shared `$TAG`, not a platform-specific tag; both Windows versions use
the same Windows DaemonSet. Normal releases also include Linux ARM64 by default.

The agent manifest helper preserves Windows `os.version` metadata: `10.0.20348.*` for
2022 and `10.0.26100.*` for 2025. Validate image selection, startup, metrics, and
captures on AKS nodes of both versions; unit and image-build checks do not replace
runtime validation after containerd updates.

### Scanning container images

Build and load all five Linux images locally before scanning them:

```bash
make retina-image retina-operator-image kubectl-retina-image retina-shell-image \
  PLATFORM=linux/amd64 TAG=security-review OUTPUT_LOCAL= BUILDX_ACTION=--load

for image in retina-agent retina-init retina-operator kubectl-retina retina-shell; do
  trivy image --image-src docker --scanners vuln \
    "ghcr.io/<owner>/retina/${image}:security-review-linux-amd64"
done
```

Replace `<owner>` with the owner of your fork. Scan all severities, including
unfixed findings, and keep before/after reports with the image IDs and Trivy
database timestamp when updating dependencies. Package findings do not establish
that a vulnerable function is reachable.

The agent and shell images rebuild Hubble and pwru from their versioned Go
modules using the pinned Go builder and patched `golang.org/x/net`. Updating only
the root `go.mod` does not update these independent binaries. Keep the Hubble
default in `Makefile` and `controller/Dockerfile` synchronized.

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

#### Windows E2E

The E2E suite runs once in one cluster with a WS2022 (`win22`) and a WS2025 (`win25`)
node pool, using the same multi-platform Retina image tag. Linux, Hubble and capture
scenarios run once; the Windows HNS and eBPF metric scenarios run per version, each
with a pod pinned to that version's node build. The basic phase runs the Windows
agent in HNS mode and the advanced phase in eBPF mode (`CiliumOnWindows` 0 or 1,
set on every Windows node before the agent starts). The run fails if a node's running
Retina image has the wrong `os.version`, for example a WS2025 node selecting the
WS2022 image. Azure NPM is retained for the Linux drop scenarios only.

```bash
go test -v ./test/e2e/. -run '^TestE2ERetina$' -tags=e2e -timeout 75m -count=1 \
  -args -windows-year=both -image-tag=<tag> \
  -image-registry=<registry> -image-namespace=<namespace>
```

The tag must contain both Windows images with `os.version` metadata, WS2025 first.
`-windows-year=2022` or `2025` (the `windows_year` workflow input) runs a single
version against a cluster with only that pool. Set `EVENT_WRITER_IMAGE` to override
the event-writer image.

The installer pins the eBPF runtime version used by the event-writer SDK, replaces
incompatible preinstalled eBPF drivers (as on WS2025 nodes), and exits non-zero on
failure so Kubernetes restarts the container. Use disposable clusters: the suite
modifies node drivers and runtime libraries.

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

## Dependency and Security Management

Retina uses automated dependency management and security scanning to maintain secure and up-to-date container images and dependencies.

### Dependabot Configuration

The repository uses [Dependabot](https://github.com/dependabot) to automatically track and update dependencies:

- **Docker Base Images**: Automatically monitored for security updates and new versions
- **Go Modules**: Tracked for dependency updates  
- **GitHub Actions**: Workflow dependencies are kept current

#### Docker Base Image Tracking

Retina has Dockerfiles in multiple directories, and each is tracked separately by Dependabot:

- `/controller` - Main retina controller images (daily checks)
- `/shell` - Shell utility images (daily checks)  
- `/cli` - CLI tool images (daily checks)
- `/operator` - Operator images (daily checks)
- `/test/image` - Test images (daily checks)
- `/hack/tools/kapinger` - Kapinger tool images (weekly checks)
- `/hack/tools/toolbox` - Toolbox utility images (weekly checks)
- `/hack/windows` - Local Windows images for Server 2022 and 2025 (weekly checks)

When Dependabot detects a security vulnerability (CVE) in a base image, it will automatically create a pull request to update the image SHA to a patched version.

### Adding New Dockerfiles

When adding new Dockerfiles to the repository:

1. Add the directory containing the Dockerfile to `.github/dependabot.yaml`
2. Choose an appropriate schedule: daily for critical components, weekly for tools

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
