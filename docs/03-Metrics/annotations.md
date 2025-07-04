# Annotations

**This feature is only available in Standard Control Plane.**

Annotations let you specify which Pods to observe (create metrics for).

To enable it, specify `enableAnnotations=true` in Retina's Standard Control Plane [helm installation](../02-Installation/01-Setup.md) or [ConfigMap](../02-Installation/03-Config.md).

You can then add the annotation `retina.sh: observe` to either:

- individual Pods
- Namespaces (to observe all the Pods in the namespace).

**Note**: If you enable Annotations, you cannot use the `MetricsConfiguration` CRD to specify which Pods to observe.
