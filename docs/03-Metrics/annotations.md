# Annotations

Annotations let you specify which Pods to observe (create metrics for).
To configure this, specify `enableAnnotations=true` in Retina's [helm installation](../02-Installation/01-Setup.md) or [ConfigMap](../02-Installation/03-Config.md).

You can then add the annotation `retina.sh: observe` to either:

- individual Pods
- Namespaces (to observe all the Pods in the namespace).

An exception: currently all Pods in `kube-system` are always monitored.
