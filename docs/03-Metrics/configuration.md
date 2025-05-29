# Metrics Configuration

You can enable/disable metrics by including/omitting their Plugin from `enabledPlugins` in either Retina's [helm installation](../02-Installation/01-Setup.md) or [ConfigMap](../02-Installation/03-Config.md).

Via [MetricsConfiguration CRD](../05-Concepts/CRDs/MetricsConfiguration.md), you can further customize the following for your enabled plugins:

- Which metrics to include
- Which metadata to include for a metric.

**Note**: If you enable [Annotations](./annotations.md), you cannot use the `MetricsConfiguration` CRD to specify which Pods to observe.
