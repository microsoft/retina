# Managed Dalec release E2E

Retina's standalone Azure DevOps pipeline is defined in
`.pipelines/retina-dalec-e2e.yml` in this repository. It does not use ACN's
Function App or existing ACN E2E/release pipelines.

## Release flow

1. Managed Dalec resolves the selected Retina release and builds binaries,
   eBPF objects, and images using the existing component templates.
2. The Dalec component test hook queues this new pipeline at an explicitly
   configured, immutable Retina **test-code commit**.
3. The pipeline downloads `build-*/runtime-variables.json` from that exact
   originating Dalec build. All five components and the Windows agent target
   must be built together. It resolves staging tags to SHA256 digests and
   rejects missing companions, conflicting images, inconsistent versions,
   missing platforms, or a triggering image absent from those artifacts.
4. A dedicated, run-owned AKS cluster executes CLI packaging/version checks,
   standard metrics, advanced-profile upgrade, Hubble metrics, Windows HNS
   metrics, and Linux capture create/download/delete against candidate images.
   The CLI itself executes from the candidate container, not a source rebuild.
5. Test failure, cancellation, timeout, or invalid input fails the Dalec hook.
   Successful E2E permits Dalec's subsequent staging workflow to continue.
   Spec PR approval/merge and normal Managed Dalec signing/promotion/MCR
   publication remain separate gates; this pipeline publishes no release.

The initial onboarding selects upstream `v1.2.9`, resolved to commit
`e3947ab0f8b13a5317f3378184e953e781ed9599`. This is the **product-source commit**,
not the separate Retina test-code commit configured below. The existing
Dalec build templates are not rewritten by this E2E integration.

## Component cohort and MCR endpoints

Managed Dalec builds five Retina components, plus a sixth `windowscross` build
target that produces the Windows agent image from the same `retina-agent`
component directory. `hack/dalec/prepare-images.py` keys each candidate off the
Dalec spec's parent directory name, so these names are the contract between this
repository and `specs/containernetworking/` on the Dalec side:

| Component | Candidate platforms | Published endpoint |
| --- | --- | --- |
| `retina-agent` | linux/amd64, linux/arm64, windows/amd64 | `mcr.microsoft.com/containernetworking/v2/retina-agent` |
| `retina-init` | linux/amd64, linux/arm64 | `mcr.microsoft.com/containernetworking/v2/retina-init` |
| `retina-operator` | linux/amd64, linux/arm64 | `mcr.microsoft.com/containernetworking/v2/retina-operator` |
| `kubectl-retina` | linux/amd64, linux/arm64 | `mcr.microsoft.com/containernetworking/v2/kubectl-retina` |
| `kubectl-retina-shell` | linux/amd64, linux/arm64 | `mcr.microsoft.com/containernetworking/v2/kubectl-retina-shell` |

The `containernetworking/v2/` namespace is new. These are five new MCR
repositories requiring onboarding and Discovery Portal decisions, not tag bumps
on the existing `containernetworking/` images; `kubectl-retina-shell` has no
pre-Dalec endpoint at all. Candidates reach this pipeline as
`<registry>.azurecr.io/staging/build/<component>:<version>@sha256:<digest>` and
are never published by it.

Explicitly outside this cohort:

- `retina-shell`. It is a distinct image from `kubectl-retina-shell`, and
  `cli/cmd/shell.go` still defaults to
  `mcr.microsoft.com/containernetworking/retina-shell`. Until it is onboarded,
  `retina shell` and `retina bpftrace` keep pulling the non-Dalec image, so
  this pipeline cannot exercise them against candidates. Onboarding it later
  requires a sixth component directory, a `RETINA_SHELL_IMAGE` environment
  name, and matching `COMPONENTS`/`ENV_NAMES` entries in
  `hack/dalec/prepare-images.py`.
- The Retina Enterprise (ACNS) images `retina-agent-enterprise` and
  `retina-operator-enterprise`.
- Hubble relay and Hubble UI images, and the E2E test workload images, which
  are pulled from their upstream sources during the run.

## One-time setup

Create a **new** YAML pipeline in the `msazure` Azure DevOps organization,
normally in `CloudNativeCompute`, connected to `microsoft/retina`. Select the
published implementation branch and `.pipelines/retina-dalec-e2e.yml`. Record
the new pipeline definition ID. No CI/PR triggers are enabled.

Provision and authorize these distinct service connections:

| Connection | Purpose and permissions |
|---|---|
| `retina-dalec-e2e` | New pipeline's Azure identity: create/delete its resource groups, VNets and AKS clusters; obtain AKS admin credentials; read/pull staging ACR images; assign AcrPull on those registries to its newly created kubelet identity. Scope registry role-assignment permissions appropriately. |
| `Dalec-to-Retina-E2E` | Dalec test-hook identity: Azure DevOps queue and view builds on the new pipeline. Azure authentication must allow obtaining an ADO token and the service principal must be admitted to the organization/project. |

Authorize the new pipeline's project Build Service identity to view the
originating Dalec pipeline and download its build artifacts, including
artifacts already published by its completed Build stage while the parent
TestAndStage stage is still running. Check cross-project job authorization
settings when the pipelines are in different projects.

The subscription needs regional quota for Ubuntu amd64 nodes, Azure Linux
FIPS nodes, Ubuntu arm64 `Standard_D4pds_v5` nodes, and Windows 2022 amd64
nodes. Default location is `westus2`; it is a queue-time parameter. Staging
ACR must be reachable from the hosted runner and cluster. This implementation
uses AKS `--attach-acr` with conventional AcrPull RBAC; ABAC-only registries
or registries in another subscription require a deliberate access adaptation.

Configure these non-secret variables on the **Dalec wrapper pipeline**:

| Variable | Value |
|---|---|
| `RETINA_E2E_PIPELINE_ID` | The newly registered pipeline ID, not an ACN pipeline ID. |
| `RETINA_E2E_SOURCE_COMMIT` | Full 40-character SHA of the reviewed, pushed Retina test implementation. |
| `RETINA_E2E_SOURCE_BRANCH` | Branch containing that commit, such as `refs/heads/itiagrawal/retina-dalec-standalone-e2e`. Default is `refs/heads/main`. |
| `RETINA_E2E_PROJECT` | New pipeline's ADO project; default `CloudNativeCompute`. |
| `RETINA_E2E_TIMEOUT_SECONDS` | Optional hook timeout; default 14400 seconds. |
| `RETINA_E2E_POLL_INTERVAL_SECONDS` | Optional polling interval; default 120 seconds. |

The companion Managed Dalec changes explicitly select `Dalec-to-Retina-E2E`
on all five Retina onboarding entries and register that literal connection
in the wrapper's exhaustive authorization list. The shared wrapper change
only authorizes an additional connection; it adds no E2E jobs there and
does not change ACN's connection or hooks.

Each component/target currently invokes its own strict queue/wait hook.
Consequently a complete cohort can start several full E2E runs and clusters.
Budget quota, parallel jobs, and cost accordingly. There is no implicit
deduplication or reuse of another component's passing result. A future shared
cohort gate must implement explicit concurrency-safe coordination.

## Manual candidate run

Queue the new pipeline with `dalecBuildId` empty, `imageVersion` such as
`1.2.9`, and this complete `imageSet` object. Every value must be a full,
digest-pinned image reference; tag-plus-digest is also accepted.

| Key | Required platforms |
|---|---|
| `retinaAgentImage` | Linux amd64 and arm64 |
| `retinaWindowsAgentImage` | Windows amd64 |
| `retinaInitImage` | Linux amd64 and arm64 |
| `retinaOperatorImage` | Linux amd64 and arm64 |
| `kubectlRetinaImage` | Linux amd64 and arm64 |
| `kubectlRetinaShellImage` | Linux amd64 and arm64 |

For example, a value has the form
`registry.azurecr.io/staging/build/retina-agent:1.2.9@sha256:<64-hex-digest>`.
Do not mix manual overrides with parent-build artifact mode. CLI version
checks accept the optional leading `v` used by release tags.

The image set is immutable during install, advanced upgrade, Hubble install,
and capture. Helm supports independent full-reference overrides for agent,
Windows agent, init, and operator. Existing GitHub E2E behavior and published
image defaults remain unchanged when those overrides are absent. Hubble
relay/UI and ordinary test workload images are outside the Retina candidate
cohort. Capture currently covers Linux, as in the existing suite; Windows is
covered by agent installation and HNS metrics, not Windows capture.

## Results and cleanup

The pipeline publishes JUnit test results plus the `retina-e2e` artifact with
the resolved image manifest, tested code commit, Go JSON test output, CLI
version output and Kubernetes diagnostics. It never publishes kubeconfig or
Kubernetes Secrets. Resources use a unique build/attempt name and a
`retinaE2ERun` ownership tag. The always-run cleanup refuses to delete a
resource group with the wrong ownership tag. Runner loss or hard termination
can still interrupt cleanup. Normal cleanup also deletes the recorded AcrPull
role assignments for this run's kubelet identity. Monitor tagged groups and remove abandoned
run-owned resources and their kubelet registry role assignments.

## Validation and activation

Local validation can check image parsing, Helm rendering/override propagation,
metadata collection, failure conditions, queue/wait behavior, shell syntax,
and Go E2E compilation without provisioning a cluster. It cannot prove live
ADO permissions, private registry access, AKS runtime behavior, Windows
execution, ESRP signing, or official publication.

Publish the reviewed Retina branch first, register/authorize the new ADO
pipeline, configure the trusted commit and IDs, and perform a manual complete
candidate run. Activate the companion Dalec hook only after that succeeds.
Do not activate a hook with an unpushed test-code commit. Rollback is to
disable the new integration or revert its Dalec configuration; it must not
silently route candidates back to ACN or substitute published Retina images.
