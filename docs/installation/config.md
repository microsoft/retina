# Retina Config

## Overview

To customize metrics and other options, you can modify Retina's ConfigMap called `retina-config`.
Defaults are specified for each component in *deploy/manifests/controller/helm/retina/values.yaml*.

## Agent Config

* `enablePodLevel`: When this toggle is set to true, Retina will gather Advanced/Pod-Level metrics. Advanced metrics can attach Pod metadata to Retina's metrics.
* `remoteContext`: When this toggle is set to true, retina will watch Pods on the cluster.
* `enableAnnotations`: When this toggle is set to true, retina will gather metrics for the annotated resources. Namespaces or Pods can be annotated with "retina.io/v1alpha=observe". The operator and enableRetinaEndpoint for the operator should be enabled.
* `enabledPlugin_linux`: Array of enabled plugins for linux.
* `enabledPlugin_win`: Array of enabled plugins for windows.
* `metricsInterval`: the interval for which metrics will be gathered.

## Operator Config

* `installCRDs`: When this toggle is set, the operator will handle installing Retina-related CRDs.
* `enableRetinaEndpoint`: When this toggle is set, the operator will watch and update the cache with Pod metadata.
