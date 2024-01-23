# Metrics Configuration

You can enable/disable metrics by including/omitting their Plugin from `enabledPlugins` in either Retina's [helm installation](../installation/setup.md) or [ConfigMap](../installation/config.md).

Via [MetricsConfiguration CRD](../CRDs/MetricsConfiguration.md), you can further customize the following for your enabled plugins:
- Which metrics to include
- Which metadata to include for a metric.
