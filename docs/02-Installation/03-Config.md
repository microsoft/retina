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

This section applies only to the Hubble control plane chart (`retina-hubble`, see [Hubble Control Plane](./01-Setup.md#hubble-control-plane)). The standard control plane chart (`retina`) does not run `manage-cilium-crds`, so these settings have no effect there.

#### When to Use

Use these settings when all of the following are true:

* Linux nodes run Cilium, and Windows nodes run a different CNI, so the Cilium agent does not create CiliumEndpoints for Windows Pods.
* cilium-operator manages CiliumIdentities (`identity-management-mode: operator`).
* You want Windows Pods to have CiliumEndpoints with the same security identities as Linux Pods that have the same identity labels (namespace, Pod labels, and service account). Cilium on Linux nodes and Hubble can then map Windows Pod IPs to those identities.

These settings do not enable network policy enforcement on Windows nodes. Retina only publishes CiliumEndpoints for Windows Pods. Policy is enforced only where Cilium runs, on Linux nodes.

Skip this section for Linux-only clusters and for clusters where the Retina operator manages all CiliumEndpoints and CiliumIdentities. Both flags are disabled by default, and existing behavior is unchanged unless they are set.

The operator flags are:

* `--windows-only-endpoints`: The operator creates CiliumEndpoints only for Pods on Windows nodes (node label `kubernetes.io/os=windows`). Pods on other nodes are left to the Cilium agent, so the operator does not compete with it.
* `--reuse-operator-identities`: CiliumEndpoints reference the CiliumIdentities created by cilium-operator instead of identities allocated by the Retina operator. cilium-operator does not garbage collect identities that these endpoints use.

Enable both flags together.

#### Prerequisites

* Retina is installed, or will be installed, with the `retina-hubble` chart and `operator.enabled=true`.
* Cilium CRDs are installed and cilium-operator is running.
* `kube-system/cilium-config` sets `identity-management-mode: operator`.
* `kube-system/cilium-config` does not set `label-prefix-file`. A label prefix file is not supported, because identity labels would not match cilium-operator's.
* If `kube-system/cilium-config` sets `labels`, it is a valid label filter. The operator uses the same `labels` and `cluster-name` values (default `default`) as cilium-operator to compute identity labels.
* The Retina operator can read `kube-system/cilium-config`. The chart grants this with the `retina-operator-cilium-config` Role and RoleBinding.

To check the Cilium settings:

```shell
kubectl get configmap cilium-config -n kube-system \
    -o jsonpath='{.data.identity-management-mode}{"\n"}{.data.label-prefix-file}{"\n"}{.data.labels}{"\n"}{.data.cluster-name}{"\n"}'
```

The first line must be `operator`, and the second line must be empty.

#### Configuration

The flags are operator arguments in `operator.container.args`. Helm replaces lists instead of merging them, so a values file must contain the full argument list, including the default arguments and any custom arguments.

For a new installation, create `windows-cilium-values.yaml`:

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

Pass it to the install command from [Hubble Control Plane](./01-Setup.md#hubble-control-plane) by adding `-f windows-cilium-values.yaml`.

For an existing installation, keep the current release values. The installed chart and operator image must be a Retina release that includes these flags. `--reuse-values` keeps the installed `operator.tag`, and the RBAC rules for these flags come from the chart. If the release is older, upgrade the chart and the image tags to a newer release first, as in [Hubble Control Plane](./01-Setup.md#hubble-control-plane). Otherwise, the operator fails to start with an unknown flag error.

1. Get the current operator arguments:

    ```shell
    helm get values retina -n kube-system -a -o json | jq -r '.operator.container.args[]'
    ```

2. Create `windows-cilium-values.yaml` with all of those arguments under `operator.container.args`, plus `--windows-only-endpoints=true` and `--reuse-operator-identities=true`.

3. Upgrade the release. `--reuse-values` keeps the other values set earlier, and `-f` replaces only the argument list:

    ```shell
    VERSION=$(helm get metadata retina -n kube-system -o json | jq -r .version)
    helm upgrade retina oci://ghcr.io/microsoft/retina/charts/retina-hubble \
        --version $VERSION \
        --namespace kube-system \
        --reuse-values \
        -f windows-cilium-values.yaml
    ```

To disable the feature, update `windows-cilium-values.yaml` so it has the full operator argument list without the two flags, and run the same `helm upgrade --reuse-values -f windows-cilium-values.yaml` command. Running `helm upgrade --reuse-values` without the file keeps both flags.

Without these flags, the Retina operator goes back to its default mode. It creates CiliumEndpoints for all Pods, including Linux Pods that Cilium manages, and allocates its own identities instead of reusing cilium-operator identities. This is not a neutral rollback. Don't use the default mode in a cluster where Cilium manages Linux endpoints.

#### Verification

1. Check that the operator runs with both flags and has reused cilium-operator identities:

    ```shell
    kubectl get deployment retina-operator -n kube-system -o jsonpath='{.spec.template.spec.containers[0].args}{"\n"}'
    kubectl logs -n kube-system deploy/retina-operator | grep "reusing cilium-operator identities"
    ```

2. Check that a Windows Pod has a CiliumEndpoint with its address and an identity:

    ```shell
    NS=<namespace>
    POD=<windows-pod>
    kubectl get ciliumendpoint -n $NS $POD \
        -o jsonpath='{.status.identity.id}{"\n"}{.status.networking.addressing}{"\n"}'
    ```

3. Check that the identity was created by cilium-operator for the Pod's labels:

    ```shell
    ID=$(kubectl get ciliumendpoint -n $NS $POD -o jsonpath='{.status.identity.id}')
    kubectl get ciliumidentities.cilium.io $ID -o jsonpath='{.security-labels}{"\n"}'
    ```

    The labels must include the Pod's namespace (`k8s:io.kubernetes.pod.namespace`), service account (`k8s:io.cilium.k8s.policy.serviceaccount`), and labels, after the `labels` filter is applied. If a Linux Pod in the same namespace has the same labels and service account, its CiliumEndpoint has the same `status.identity.id`.

#### Troubleshooting

The operator does not exit if a prerequisite is missing. Instead, it keeps running, does not manage Windows CiliumEndpoints, and rereads `kube-system/cilium-config` with backoff (starting at 1 second, up to 1 minute) until the configuration is valid. Each new error is logged once at error level:

```text
failed to set up --reuse-operator-identities; Windows CiliumEndpoints are not managed until kube-system/cilium-config is fixed
```

The `error` field gives the cause, such as the wrong `identity-management-mode`, a `label-prefix-file` being set, or an invalid `labels` value. After you fix `cilium-config`, the operator recovers automatically without a restart and logs `reusing cilium-operator identities`.

To check the state, search the operator logs:

```shell
kubectl logs -n kube-system deploy/retina-operator | grep -E "reuse-operator-identities|reusing cilium-operator identities"
```

If cilium-operator has not created the identity for a Pod yet, the Pod is requeued and retried, and the operator logs `identity not yet created by cilium-operator, requeuing pod`. This message is expected briefly after a Pod starts. If it continues, check that cilium-operator is running and that `identity-management-mode` is `operator`.

If a Windows Pod has no CiliumEndpoint, check that its node has the label `kubernetes.io/os=windows`, that the Pod has an IP address, and that the Pod does not use `hostNetwork: true`.
