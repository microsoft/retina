# Managed Dalec release E2E

Retina reuses its existing **CI (Build Images)** pipeline, definition
**1195**, in `github-private/microsoft`, folder `\OneBranch\Retina OSS`:
https://github-private.visualstudio.com/microsoft/_build?definitionId=1195

Its YAML is `.azure-pipelines/ci.yml`. Set `runDalecE2E: true` to test supplied
candidate images without rebuilding them. The default remains normal CI.
The earlier standalone `.pipelines/retina-dalec-e2e.yml` and definitions
1217 / 484261 are legacy alternatives, not the activation target. No new
pipeline registration is needed. This integration does not use ACN's Function
App or existing ACN E2E/release pipelines.

## Release flow

1. Managed Dalec resolves the selected Retina release and builds binaries,
   eBPF objects, and images using the existing component templates.
2. The Dalec component test hook queues pipeline 1195 at an explicitly
   configured, immutable Retina **test-code commit**, with `runDalecE2E: true`,
   `imageVersion`, and the complete digest-pinned `imageSet`.
3. The caller must collect all five components and the Windows agent target
   from the same originating build, verify their common source version, and
   ensure the triggering image belongs to that cohort. Pipeline 1195 checks
   the exact image keys, digest syntax, release version format, registry
   access, and required platforms before provisioning AKS. It does not
   independently prove source-build provenance from an explicit image set.
   CI image builds, image pushes, and manifest creation are excluded at
   template expansion time in Dalec mode.
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

Keep definition 1195's existing YAML path, GitHub connection, default branch,
triggers, and normal PR checks unchanged. Until the implementation merges,
select `itiagrawal/retina-dalec-standalone-e2e` when queueing Dalec mode.

The originating build is `[aks-managed-dalec][build] containernetworking`,
definition ID `452291`, in `msazure/CloudNativeCompute`. That is a different
organization. `DownloadPipelineArtifact@2` cannot read across organizations,
so pipeline 1195 deliberately has no artifact-download step in Dalec mode.
The trusted caller must resolve metadata in `msazure` and pass the explicit
image set. Changing only the old hook's pipeline ID is **not sufficient**:
its organization/project, queue parameters, and metadata handoff must also
be updated before activating the hook.

Provision and authorize these distinct service connections:

| Connection | Purpose and permissions |
|---|---|
| `r2d`, or an explicitly selected `dalecAzureServiceConnection` | Pipeline 1195's Azure identity: create/delete run-owned resource groups, VNets and AKS clusters; obtain AKS admin credentials; read/pull staging ACR images; assign AcrPull on those registries to its newly created kubelet identity. Confirm these permissions; existing CI authorization does not prove staging-ACR access or role-assignment rights. |
| `Dalec-to-Retina-E2E` | Dalec test-hook identity: queue and view builds on definition 1195 in `github-private/microsoft`. Azure authentication must allow obtaining an ADO token and the identity must be admitted to that organization/project. |

The caller, not 1195's Build Service, needs access to metadata from originating
Dalec build 452291 in `msazure/CloudNativeCompute`, including artifacts already
published while its TestAndStage stage is still running. The Dalec job uses
the OneBranch Linux **host**, not its build container: the capture CLI wrapper
requires a Docker daemon. Required host tools are checked explicitly.

The subscription needs regional quota for Ubuntu amd64 nodes, Azure Linux
FIPS nodes, Ubuntu arm64 `Standard_D4pds_v5` nodes, and Windows 2022 amd64
nodes. Default location is `westus2`; it is a queue-time parameter. Staging
ACR must be reachable from the hosted runner and cluster. This implementation
uses AKS `--attach-acr` with conventional AcrPull RBAC; ABAC-only registries
or registries in another subscription require a deliberate access adaptation.

The caller's queue contract is:

```text
POST https://github-private.visualstudio.com/microsoft/_apis/pipelines/1195/runs?api-version=7.1-preview.1
```

For this organization's Run API, encode template parameter values as strings,
including JSON serialization of the `imageSet` object. For example, construct
the request body in Python as follows, then send `json.dumps(body)` with
`Content-Type: application/json` and the caller's ADO bearer token:

```python
import json

image_set = {
    "retinaAgentImage": "<digest-pinned Linux agent index>",
    "retinaWindowsAgentImage": "<digest-pinned Windows agent>",
    "retinaInitImage": "<digest-pinned init index>",
    "retinaOperatorImage": "<digest-pinned operator index>",
    "kubectlRetinaImage": "<digest-pinned CLI index>",
    "kubectlRetinaShellImage": "<digest-pinned CLI shell index>",
}
body = {
  "resources": {
    "repositories": {
      "self": {
        "refName": "refs/heads/itiagrawal/retina-dalec-standalone-e2e",
        "version": "<full reviewed test-code commit SHA>"
      }
    }
  },
  "templateParameters": {
    "runDalecE2E": "true",
    "runE2EValidation": "false",
    "dalecAzureServiceConnection": "r2d",
    "dalecLocation": "westus2",
    "imageVersion": "1.2.9",
    "imageSet": json.dumps(image_set)
  }
}
```

The placeholders above must be replaced with real values, not submitted as-is.
Wait for the returned run's terminal result; only `succeeded` permits the hook
to pass. A parked, canceled, failed, or timed-out run is not a passing result.

The companion Managed Dalec changes must select `Dalec-to-Retina-E2E`
on all five Retina onboarding entries, register that literal connection
in the wrapper's authorization list, and implement the queue contract above.
This Retina-side change does not modify the private Dalec wrapper or ACN's
connection and hooks.

Each component/target currently invokes its own strict queue/wait hook.
Consequently a complete cohort can start several full E2E runs and clusters.
Budget quota, parallel jobs, and cost accordingly. There is no implicit
deduplication or reuse of another component's passing result. A future shared
cohort gate must implement explicit concurrency-safe coordination.

## Manual candidate run

Open pipeline 1195, choose **Run pipeline**, select the implementation branch,
enable **Test Dalec candidate images without rebuilding**, leave **Publish
branch images and run E2E** unchecked, and supply `imageVersion` such as
`1.2.9` and this complete `imageSet` object. Every value must be a full,
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
Pipeline 1195 has no `dalecBuildId` parameter or parent-build artifact mode.
Do not enable both E2E modes; the request fails before testing. Empty or
malformed image sets fail the input preflight without registry access or AKS
creation. CLI version checks accept the optional leading `v` used by release tags.

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

Run the local regression checks with:

```bash
python3 -m pip install -r hack/dalec/requirements-test.txt
python3 -m unittest discover -s hack/dalec -p 'test_*.py'
```

Publish the reviewed Retina branch first, verify pipeline 1195's selected
Azure connection permissions, configure the trusted commit and queue contract,
and perform a complete candidate run. Update and activate the companion Dalec
hook only after that succeeds.
Do not activate a hook with an unpushed test-code commit. Rollback is to
disable the new integration or revert its Dalec configuration; it must not
silently route candidates back to ACN or substitute published Retina images.
