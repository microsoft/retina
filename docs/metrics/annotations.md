# Annotations

Annotations let you specify which Pods to observe (create metrics for).
To configure this, specify `enableAnnotations=true` in Retina's [helm installation](../installation/setup.md) or [ConfigMap](../installation/config.md).

You can then add the annotation `retina.sh/v1alpha1: observe` to either:

- individual Pods
- Namespaces (to observe all the Pods in the namespace).

An exception: currently all Pods in `kube-system` are always monitored.
