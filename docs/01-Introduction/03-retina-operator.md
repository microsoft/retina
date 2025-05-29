# Retina Operator

## Overview

The Retina Operator is a key control plane component responsible for orchestrating and managing the deployment, configuration, and lifecycle of Retina's functionality in a Kubernetes environment. Here is a detailed summary covering its role, function, configuration, and typical use cases:

## Role and Responsibilities

Roles and responsibilities of the operator vary based on the Retina control plane.

### General Responsibilities

**Configuration Propagation**:
The operator reads its configuration from a YAML file (default: `retina/operator-config.yaml`). It loads this configuration, applies defaults, and ensures all subcomponents (like telemetry and capture jobs) receive the correct parameters.

**Pod Metadata and Endpoint Management**:
The operator can monitor Pods and update a cache with metadata, which is used by Retina for enriching network flow data with Kubernetes context.

**Telemetry and Logging**:
The operator can send telemetry data (if enabled) and manages its logging for observability and debugging.

### Standard Control Plane

**Lifecycle Management**:
The operator manages the lifecycle of Retina custom resources, `Capture`, `RetinaEndpoint` and `MetricsConfiguration`.

**Capture Job Control**:
It is responsible for launching, managing, and limiting the number of capture jobs, which are used for network tracing and troubleshooting.

### Hubble Control Plane

**CRD Management**:
The operator can install and manage the Cilium CRDs required for Retina's operation, ensuring that all necessary resources are available and up to date. The CRDs managed by the operator are `ciliumendpoint` and `ciliumidentity`.

## Configuration

The Retina Operator is configured via a YAML configuration file (default: `operator-config.yaml`). Key configuration options include:

* `operator.installCRDs`: Boolean to control automatic CRD installation.
* `operator.enableRetinaEndpoint`: Enables monitoring and updating the cache with Pod metadata.
* `capture.captureDebug`: Enables debug mode and uses test container registry images for capture jobs.
* `capture.captureJobNumLimit`: Limits the number of concurrent capture jobs per Capture resource.
* `capture.enableManagedStorageAccount`: Enables use of managed storage accounts for storing capture artifacts.

Defaults are applied for critical settings like telemetry intervals and feature toggles. The operator also supports dynamic configuration via environment variables.

## Function and Operation

* **Startup Sequence**:
    On startup, the operator reads its configuration, initializes logging and telemetry, registers CRDs, and sets up controllers for custom resources like Captures. It also sets up leader election if HA is enabled.

* **Capture Job Reconciliation**:
    The operator watches for Capture resources, creates or deletes capture jobs as needed, and enforces job limits.

* **CRD and Resource Reconciliation**:
    Ensures all Retina-related CRDs are present and up-to-date, reconciling resources as the cluster state changes.

* **Telemetry and Metrics**:
    If enabled, collects and sends operational metrics and telemetry to Application Insights or other configured backends.

* **Endpoint Metadata Caching**:
    Maintains a cache of Pod and endpoint metadata to allow Retina to enrich network flow data with Kubernetes context (e.g., mapping IPs to Pods, Nodes, Namespaces).

## Use Cases

* **Automated Deployment and Upgrades**:
    The operator automates the rollout and updates of Retina-related resources, minimizing manual intervention.

* **Network Capture and Troubleshooting**:
    Users can define custom Capture resources to initiate network tracing/troubleshooting jobs. The operator manages the lifecycle and resource limits for these jobs.

* **Kubernetes Metadata Enrichment**:
    By maintaining an up-to-date cache of endpoint metadata, the operator allows Retina’s data plane components to provide rich context for network flows and observability.

* **High Availability**:
    Supports HA deployments to ensure continuous management of Retina resources even if an operator pod fails.

* **Telemetry and Monitoring**:
    Provides operational insights by collecting and exporting telemetry and logs.

## Summary Table

| Function/Responsibility         | Description                                                          |
| ------------------------------- | -------------------------------------------------------------------- |
| CRD Management                  | Installs and manages Retina CRDs or Cilium CRDs                                   |
| Leader Election                 | Ensures only one active operator in HA mode                          |
| Capture Job Control             | Launches and limits capture jobs                                     |
| Pod/Endpoint Metadata Caching   | Maintains live mapping of IPs to Kubernetes objects                  |
| Telemetry & Logging             | Collects and exports operational metrics and logs                    |
| Configuration Propagation       | Reads, validates, and applies configuration to Retina components     |
