# Retina Config

## Overview

Retina config allows you to define the level of granularity and filtering of Retina metrics, as well as additional options related to retina-agent and retina-operator. For example, we can update Retina config to enable captures, annotations, and define data aggregation.

### Default Configuration

Default settings for each component are specified in [Values file](../../deploy/standard/manifests/controller/helm/retina/values.yaml).

### Deployed Configuration

Configuration of an active Retina deployment can be seen in `retina-config` and `retina-operator-config` configmaps.

```shell
kubectl get configmap retina-config -n kube-system -o yaml
kubectl get configmap retina-operator-config -n kube-system -o yaml
```

### Updating Configuration

If the Retina installation was done via Helm, configuration updates should be done via `helm upgrade` defining the specific attribute name and value as part of the command.

The example below enables gathering of advance pod-level metrics.

```shell
VERSION=$( curl -sL https://api.github.com/repos/microsoft/retina/releases/latest | jq -r .name)
helm upgrade --install retina oci://ghcr.io/microsoft/retina/charts/retina \
    --version $VERSION \
    --namespace kube-system \
    --set image.tag=$VERSION \
    --set operator.tag=$VERSION \
    --set logLevel=info \
    --set enabledPlugin_linux="\[dropreason\,packetforward\,linuxutil\,dns\]"
    --set enablePodLevel=true
```

## General Configuration

Apply to both Agent and Operator.

* `enableTelemetry`: Enables telemetry for the agent for managed AKS clusters. Requires `buildinfo.ApplicationInsightsID` to be set if enabled.
* `remoteContext`: Enables Retina to watch Pods on the cluster.

## Agent Configuration

* `logLevel`: Define the level of logs to store.
* `enabledPlugin_linux`: List of enabled plugins.
* `metricsInterval`: Interval for gathering metrics (in seconds). (@deprecated, use `metricsIntervalDuration` instead)
* `metricsIntervalDuration`: Interval for gathering metrics (in `time.Duration`).
* `enablePodLevel`: Enables gathering of advanced pod-level metrics, attaching pods' metadata to Retina's metrics.
* `enableConntrackMetrics`: Enables conntrack metrics for packets and bytes forwarded/received.
* `enableAnnotations`: Enables gathering of metrics for annotated resources. Resources can be annotated with `retina.sh=observe`. Requires the operator and `operator.enableRetinaEndpoint` to be enabled. By enabling annotations, the agent will not use MetricsConfiguration CRD.
* `bypassLookupIPOfInterest`: If true, plugins like `packetparser` and `dropreason` will bypass IP lookup, generating an event for each packet regardless. `enableAnnotations` will not work if this is true.
* `dataAggregationLevel`: Defines the level of data aggregation for Retina. See [Data Aggregation](../05-Concepts/data-aggregation.md) for more details.
* `dataSamplingRate`: Defines the data sampling rate for `packetparser`.  See [Sampling](../03-Metrics/plugins/Linux/packetparser.md#sampling) for more details.
* `conntrackReportInterval`: Periodic interval (in `time.Duration`, default `30s`) at which conntrack reports active connections in high data aggregation mode. Values below `1s` fall back to the default. See [Report interval](../03-Metrics/plugins/Linux/packetparser.md#report-interval) for more details.
* `packetParserRingBuffer`: Selects the kernel-to-userspace transport for `packetparser`. Accepted values: `enabled` (ring buffer) or `disabled` (perf event array). `auto` is reserved for future use.
* `packetParserRingBufferSize`: Ring buffer size in bytes when `packetParserRingBuffer=enabled`. Must be a power of two between the kernel page size and 1GiB (inclusive); invalid values cause startup to fail.

## Operator Configuration

* `operator.installCRDs`: Allows the operator to manage the installation of Retina-related CRDs.
* `operator.enableRetinaEndpoint`: Allows the operator to monitor and update the cache with Pod metadata.
* `capture.captureDebug`: Toggles debug mode for captures. If true, the operator uses the image from the test container registry for the capture workload. Refer to [Capture Image file](../../pkg/capture/utils/capture_image.go) for details on how the debug capture image version is selected.
* `capture.captureJobNumLimit`: Sets the maximum number of jobs that can be created for each Capture.
* `capture.hostPathBaseDir`: Absolute directory on every node under which Capture CRs may write artifacts. The CR field `outputConfiguration.hostPath` is treated as a relative subpath name and joined under this directory; CR authors cannot influence the base. Defaults to `/var/log/retina/captures` when unset.
* `capture.admissionPolicy.enabled`: Installs a `ValidatingAdmissionPolicy` that only lets users holding the cluster-scoped `capture-nodes` / `capture-namespaces` verbs on `captures.retina.sh` create Captures that use `captureTarget.nodeSelector` / `captureTarget.namespaceSelector`. Defaults to `true`, applies only when `operator.enabled=true`, and requires Kubernetes 1.30 or later (the install fails on older clusters unless this is set to `false`; with `helm template`, pass `--kube-version`). See [Capture with CRD - Cluster Access](../04-Captures/03-crd.md#cluster-access).
* `capture.enableManagedStorageAccount`: Enables the use of a managed storage account for storing artifacts.

### Windows Nodes in Cilium Clusters

These operator flags apply to the Hubble control plane chart (`deploy/hubble/manifests/controller/helm/retina`), where the operator runs `manage-cilium-crds`. They are for clusters where Linux nodes run Cilium and Windows nodes run a different CNI, so the Cilium agent does not manage Windows Pods. Both flags are disabled by default, and existing behavior is unchanged unless they are set.

* `--windows-only-endpoints`: The operator creates CiliumEndpoints only for Pods on Windows nodes (node label `kubernetes.io/os=windows`). Pods on other nodes are skipped and left to the Cilium agent. Enable this when Cilium already runs on the Linux nodes, so the operator does not compete with the Cilium agent for Linux Pods.
* `--reuse-operator-identities`: CiliumEndpoints reference the CiliumIdentities created by cilium-operator instead of identities allocated by the Retina operator. Windows Pods then share a security identity with matching Linux Pods, so Cilium network policies select them consistently, and cilium-operator does not garbage collect the identities they use. Enable this when cilium-operator manages identities.

Enable both flags together for mixed Linux (Cilium) and Windows clusters. The flags are passed as operator arguments. Because Helm replaces lists, set the full argument list, for example in a values file:

```yaml
operator:
  container:
    args:
      - "manage-cilium-crds"
      - "--config-dir"
      - "/retina"
      - "--windows-only-endpoints=true"
      - "--reuse-operator-identities=true"
```

#### Prerequisites for `--reuse-operator-identities`

* Cilium CRDs are installed and cilium-operator is running.
* `kube-system/cilium-config` sets `identity-management-mode: operator`, so cilium-operator creates CiliumIdentities for Pods.
* `kube-system/cilium-config` does not set `label-prefix-file`. A label prefix file is not supported, because identity labels would not match cilium-operator's.
* If `kube-system/cilium-config` sets `labels`, it must be a valid label filter. The operator uses the same `labels` and `cluster-name` values (default `default`) as cilium-operator to compute identity labels.
* The operator can read `kube-system/cilium-config`. The chart grants `get` on that ConfigMap with the `retina-operator-cilium-config` Role and RoleBinding.

#### Disabled-and-Retry Behavior

The operator does not exit if a prerequisite is missing. Instead, it keeps running, does not manage Windows CiliumEndpoints, and rereads `kube-system/cilium-config` with backoff (starting at 1 second, up to 1 minute) until the configuration is valid. Each new error is logged once at error level:

```text
failed to set up --reuse-operator-identities; Windows CiliumEndpoints are not managed until kube-system/cilium-config is fixed
```

The `error` field gives the cause, such as the wrong `identity-management-mode`, a `label-prefix-file` being set, or an invalid `labels` value. After you fix `cilium-config`, the operator recovers automatically without a restart and logs:

```text
reusing cilium-operator identities
```

To check the state, search the operator logs:

```shell
kubectl logs -n kube-system deploy/retina-operator | grep -E "reuse-operator-identities|reusing cilium-operator identities"
```

If cilium-operator has not created the identity for a Pod yet, the Pod is requeued and retried, and the operator logs `identity not yet created by cilium-operator, requeuing pod`. This message is expected briefly after a Pod starts.
