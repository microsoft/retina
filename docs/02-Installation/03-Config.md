# Retina Config

## Overview

To customize metrics and other options, modify the `retina-config` ConfigMap. Default settings for each component are specified in *deploy/legacy/manifests/controller/helm/retina/values.yaml*.

## Agent Config

* `enableTelemetry`: Enables telemetry for the agent for managed AKS clusters. Requires `buildinfo.ApplicationInsightsID` to be set if enabled.
* `enablePodLevel`: Enables gathering of advanced pod-level metrics, attaching pods' metadata to Retina's metrics.
* `remoteContext`: Enables Retina to watch Pods on the cluster.
* `enableAnnotations`: Enables gathering of metrics for annotated resources. Resources can be annotated with `retina.sh=observe`. Requires the operator and `enableRetinaEndpoint` to be enabled.
* `enabledPlugin`: List of enabled plugins.
* `metricsInterval`: Interval for gathering metrics (in seconds). (@deprecated, use `metricsIntervalDuration` instead)
* `metricsIntervalDuration`: Interval for gathering metrics (in `time.Duration`).
* `bypassLookupIPOfInterest`: If true, plugins like `packetparser` and `dropreason` will bypass IP lookup, generating an event for each packet regardless. `enableAnnotations` will not work if this is true.
* `dataAggregationLevel`: Defines the level of data aggregation for Retina. See [Data Aggregation](../05-Concepts/data-aggregation.md) for more details.

## Operator Config

* `installCRDs`: Allows the operator to manage the installation of Retina-related CRDs.
* `enableTelemetry`: Enables telemetry for the operator in managed AKS clusters. Requires `buildinfo.ApplicationInsightsID` to be set if enabled.
* `captureDebug`: Toggles debug mode for captures. If true, the operator uses the image from the test container registry for the capture workload. Refer to *pkg/capture/utils/capture_image.go* for details on how the debug capture image version is selected.
* `captureJobNumLimit`: Sets the maximum number of jobs that can be created for each Capture.
* `enableRetinaEndpoint`: Allows the operator to monitor and update the cache with Pod metadata.
* `enableManagedStorageAccount`: Enables the use of a managed storage account for storing artifacts.
