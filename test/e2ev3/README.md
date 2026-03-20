# Retina E2E Tests (v3)

End-to-end tests built on [go-workflow](https://github.com/Azure/go-workflow), a DAG-based test orchestration framework.

## Prerequisites

- Go 1.24+
- Docker (required for the Kind provider)

## Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `TAG` | No | `git describe` | Image tag. If unset, images are built from source. |
| `IMAGE_NAMESPACE` | No | `microsoft/retina` | Image namespace |
| `IMAGE_REGISTRY` | No | `ghcr.io` | Container registry |
| `AZURE_SUBSCRIPTION_ID` | Azure only | — | Azure subscription ID |
| `AZURE_LOCATION` | Azure only | — | Azure region (fallback: `LOCATION`) |
| `AZURE_RESOURCE_GROUP` | Azure only | — | Resource group name |
| `CLUSTER_NAME` | Azure only | — | AKS cluster name |
| `HELM_DRIVER` | No | `secrets` | Helm storage driver |

## Test Flags

| Flag | Default | Description |
|---|---|---|
| `-provider` | `azure` | Infrastructure provider: `azure` or `kind` |
| `-kubeconfig` | `""` | Path to an existing kubeconfig (skips infra creation) |
| `-create-infra` | `true` | Create infrastructure before tests |
| `-delete-infra` | `true` | Delete infrastructure after tests |

## Running Tests

All commands are run from `test/e2ev3/`.

### Make Targets

```bash
make test-e2e                     # Run all scenarios
make test-basic-metrics           # Drop, TCP, DNS
make test-advanced-metrics        # DNS, latency
make test-hubble-metrics          # Hubble drop, TCP, DNS, flows
make test-capture                 # Packet capture
make test-basic-metrics-exp       # Experimental basic metrics
make test-advanced-metrics-exp    # Experimental advanced metrics
```

The default provider is `kind`. When no `TAG` is set, images are built from source automatically using `git describe` as the tag (agent, init, and operator for linux/amd64). For Kind, images are built locally; for Azure, they are built and pushed to the registry.

Override with Make variables:

```bash
# Use an existing Kind cluster
make test-basic-metrics KUBECONFIG=$HOME/.kube/config CREATE_INFRA=false DELETE_INFRA=false

# Run against Azure
make test-e2e PROVIDER=azure
```

### Kind (Local)

With no environment variables, images are built from source and loaded onto a new Kind cluster:

```bash
make test-e2e
```

Or with an explicit tag pointing at pre-built images:

```bash
TAG=v0.0.1 \
IMAGE_NAMESPACE=retina \
IMAGE_REGISTRY=ghcr.io/microsoft \
  go test -v -tags e2e ./test/e2ev3/ \
    -provider=kind \
    -timeout 60m
```

Use an existing Kind cluster:

```bash
TAG=v0.0.1 \
IMAGE_NAMESPACE=retina \
IMAGE_REGISTRY=ghcr.io/microsoft \
  go test -v -tags e2e ./test/e2ev3/ \
    -provider=kind \
    -kubeconfig=$HOME/.kube/config \
    -create-infra=false \
    -delete-infra=false \
    -timeout 60m
```

### Azure (AKS)

Create an AKS cluster, run all scenarios, and tear down:

```bash
TAG=v0.0.1 \
IMAGE_NAMESPACE=retina \
IMAGE_REGISTRY=ghcr.io/microsoft \
AZURE_SUBSCRIPTION_ID=<sub-id> \
AZURE_LOCATION=eastus2 \
AZURE_RESOURCE_GROUP=retina-e2e-rg \
CLUSTER_NAME=retina-e2e \
  go test -v -tags e2e ./test/e2ev3/ \
    -provider=azure \
    -timeout 120m
```

Use an existing AKS cluster:

```bash
TAG=v0.0.1 \
IMAGE_NAMESPACE=retina \
IMAGE_REGISTRY=ghcr.io/microsoft \
  go test -v -tags e2e ./test/e2ev3/ \
    -kubeconfig=$HOME/.kube/config \
    -create-infra=false \
    -delete-infra=false \
    -timeout 120m
```

### Running a Specific Sub-Test

> **Note:** The test pipeline runs as a single `flow.Pipe` — there are no Go
> sub-tests. The individual Makefile targets (`test-basic-metrics`, etc.)
> currently run the full pipeline. To run a subset, use `-kubeconfig` to point
> at an existing cluster and comment out unwanted steps in
> `retina_e2e_test.go`.

## Workflow Structure

Each scenario follows the same DAG pattern:

```
create → exec → validate (retry with backoff) → cleanup (always)
```

- **Create** — Provision resources (pods, network policies).
- **Exec** — Generate traffic (curl, nslookup).
- **Validate** — Port-forward to Retina or Hubble and assert Prometheus metrics. Retried with exponential backoff.
- **Cleanup** — Delete resources. Runs even if validation fails via `When(flow.Always)`.

## Directory Layout

```
test/e2ev3/
├── retina_e2e_test.go              # Test entry point (declarative pipeline)
├── Makefile                        # Make targets
├── config/                         # E2E config, flags, paths, shared params
│   ├── e2e.go                      # Config types, env loading, E2EParams
│   └── load_step.go                # config.Step — resolves config + image tag
├── pkg/
│   ├── images/                     # Image loading interface + images.Step
│   │   ├── build/                  # Build images from source + build.Step
│   │   └── load/                   # Load images onto clusters (Kind sideload vs registry pull)
│   ├── infra/                      # Infrastructure orchestration + infra.Workflow
│   │   └── providers/
│   │       ├── azure/              # AKS cluster provisioning (ARM templates)
│   │       └── kind/               # Kind cluster lifecycle (native SDK)
│   ├── kubernetes/                 # Reusable K8s steps (Helm, pods, port-forward, exec)
│   ├── prometheus/                 # Prometheus metric scraping and validation
│   └── utils/                      # Shared utilities
└── workflows/
    ├── basicmetrics/               # Drop, TCP, DNS scenarios
    │   └── experimental/           # Experimental basic metrics (conntrack, forward, etc.)
    ├── advancedmetrics/            # DNS, latency scenarios (upgraded Helm profile)
    │   └── experimental/           # Experimental advanced metrics (drop, forward, etc.)
    ├── hubblemetrics/              # Hubble drop, TCP, DNS, flow scenarios
    └── capture/                    # Packet capture validation
```
