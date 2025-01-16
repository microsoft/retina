# Retina Config

## Overview

To customize metrics and other options, upgrade Retina defining the specific parameter and attribute as part of the `helm upgrade` command.

### Example

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

Default settings for each component are specified in [Values file](../../deploy/legacy/manifests/controller/helm/retina/values.yaml).

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
* `enableAnnotations`: Enables gathering of metrics for annotated resources. Resources can be annotated with `retina.sh=observe`. Requires the operator and `operator.enableRetinaEndpoint` to be enabled.
* `bypassLookupIPOfInterest`: If true, plugins like `packetparser` and `dropreason` will bypass IP lookup, generating an event for each packet regardless. `enableAnnotations` will not work if this is true.
* `dataAggregationLevel`: Defines the level of data aggregation for Retina. See [Data Aggregation](../05-Concepts/data-aggregation.md) for more details.

## Operator Configuration

* `operator.installCRDs`: Allows the operator to manage the installation of Retina-related CRDs.
* `operator.enableRetinaEndpoint`: Allows the operator to monitor and update the cache with Pod metadata.
* `capture.captureDebug`: Toggles debug mode for captures. If true, the operator uses the image from the test container registry for the capture workload. Refer to [Capture Image file](../../pkg/capture/utils/capture_image.go) for details on how the debug capture image version is selected.
* `capture.captureJobNumLimit`: Sets the maximum number of jobs that can be created for each Capture.
* `capture.enableManagedStorageAccount`: Enables the use of a managed storage account for storing artifacts.
