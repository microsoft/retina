# Managed Dalec release offloading

Managed Dalec owns the release build, signing, promotion, and publication
process. Retina keeps its existing CI and E2E validation unchanged. Offloading
the release does not require a new Retina E2E pipeline, candidate-image
overrides, or a separate candidate-validation mode.

## Validation and release boundary

1. Validate the selected Retina source using the existing **CI (Build Images)**
   pipeline (definition **1195** in `github-private/microsoft`,
   `.azure-pipelines/ci.yml`) and the other required repository checks.
2. For a full CI/E2E run outside the merge queue, queue that existing pipeline
   at the selected source commit with **Publish branch images and run E2E**
   (`runE2EValidation`) enabled. Ordinary PR validation is build-only; its E2E
   check is a placeholder. Merge-queue validation runs the full CI/E2E path.
3. Configure the Managed Dalec onboarding to build the selected Retina release
   from an immutable product-source commit. The initial selection is `v1.2.9`,
   resolved to `e3947ab0f8b13a5317f3378184e953e781ed9599`.
4. Keep signing, staging, promotion, and MCR publication in the normal Managed
   Dalec release workflow, with its own approvals and release gates.

Existing CI builds and tests its own images. A passing CI/E2E run is source
validation, not proof that the exact Dalec-produced image digests were deployed
or tested. Testing those artifacts would be a separate requirement and is not
part of this integration.

The existing pipeline is available at:
https://github-private.visualstudio.com/microsoft/_build?definitionId=1195

No new pipeline registration or change to the shared `test/e2e` suite is
required. The private Managed Dalec onboarding and release configuration are
maintained outside this repository; this document does not activate or modify
them.

## Component cohort and publication endpoints

The planned Dalec cohort contains five components. The `retina-agent`
component also has a `windowscross` target for the Windows agent.

| Component | Platforms | Planned published endpoint |
| --- | --- | --- |
| `retina-agent` | linux/amd64, linux/arm64, windows/amd64 | `mcr.microsoft.com/containernetworking/v2/retina-agent` |
| `retina-init` | linux/amd64, linux/arm64 | `mcr.microsoft.com/containernetworking/v2/retina-init` |
| `retina-operator` | linux/amd64, linux/arm64 | `mcr.microsoft.com/containernetworking/v2/retina-operator` |
| `kubectl-retina` | linux/amd64, linux/arm64 | `mcr.microsoft.com/containernetworking/v2/kubectl-retina` |
| `kubectl-retina-shell` | linux/amd64, linux/arm64 | `mcr.microsoft.com/containernetworking/v2/kubectl-retina-shell` |

These `containernetworking/v2/` endpoints require MCR onboarding and publication
verification; this document does not claim they are available. It does not
change the existing Helm or CLI image defaults to those endpoints.

`retina-shell` is distinct from `kubectl-retina-shell` and is outside this
cohort. The existing `retina shell` and `retina bpftrace` commands continue to
use their current image defaults. Retina Enterprise images, Hubble relay/UI,
and test-workload images are also outside this release cohort.

## Activation

Before enabling release offloading, verify the external Dalec onboarding,
selected source commit, component/platform coverage, signing permissions, MCR
onboarding, and publication results. This repository change alone does not
establish those permissions or complete a release.

Do not register a standalone Retina Dalec E2E pipeline or configure a
`runDalecE2E` hook: neither is part of this release-only approach. Rollback is
to disable the external release integration; existing Retina CI/E2E remains
unchanged.
