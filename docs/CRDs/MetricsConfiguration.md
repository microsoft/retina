# MetricsConfiguration CRD

## Overview

Retina by default emits node level metrics, however, customers can apply `MetricsConfiguration`  custom resource definition (CRD) to enable advanced metrics and they can define in which namespaces the advanced metrics to be enabled.  

## CRD Specification

The full specification for the `MetricsConfiguration` CRD can be found in the [MetricsConfiguration CRD](https://github.com/microsoft/retina/blob/main/deploy/legacy/manifests/controller/helm/retina/crds/retina.sh_metricsconfigurations.yaml) file.

The `MetricsConfiguration` CRD is defined with the following specifications:

- **API Group:** retina.sh
- **API Version:** v1alpha1
- **Kind:** MetricsConfiguration
- **Plural:** metricsconfigurations
- **Singular:** metricsconfiguration
- **Scope:** Cluster

### Fields

- **spec.contextOptions:** Specifies the configuration for retina plugin metrics context. It includes the following properties:
  - `additionalLabels`: Represents additional context labels to be collected, such as Direction (ingress/egress).
  - `destinationLabels`: Represents the destination context labels, such as IP, Pod, port, workload (deployment/replicaset/statefulset/daemonset).
  - `metricName`: Indicates the name of the metric.
  - `sourceLabels`: Represents the source context labels, such as IP, Pod, port.

- **spec.namespaces:** Specifies the namespaces to include or exclude in metric collection. It includes the following properties:
  - `exclude`: Specifies namespaces to be excluded from metric collection.
  - `include`: Specifies namespaces to be included in metric collection.

- **status:** Describes the status of the metrics configuration, including the last known specification, reason, and state.

## Usage

### Creating a MetricsConfiguration

To create a `MetricsConfiguration`, create a YAML manifest file with the desired specifications and apply it to the cluster using `kubectl apply`:

```yaml
apiVersion: retina.sh/v1alpha1
kind: MetricsConfiguration
metadata:
  name: metricsconfigcrd
spec:
  contextOptions:
    - metricName: drop_count
      sourceLabels:
        - ip
        - podname
        - port
      additionalLabels:
        - direction
    - metricName: forward_count
      sourceLabels:
        - ip
        - podname
        - port
      additionalLabels:
        - direction
  namespaces:
    include:
      - default
      - kube-system
```

## Validation of MetricsConfiguration CRD

The **Operator Pod** acts as a validator for customer-applied CRDs. It reads metrics and/or traces CRDs, validates options, and updates the status of the applied CRDs accordingly.

### Status States

The status of the CRD can be one of the following:

1. **Initiated (optional)**: The initial state upon CRD application. The Operator may set it to "Initiated" when the CRD is first applied. This state can be skipped for simplicity.

2. **Accepted**: After validating the CRD options, the Operator transitions the status to "Accepted" if everything is valid.

3. **Error**: In case of validation issues, the Operator updates the status to "Error" along with a reason for the CRD's invalidity.

### Interaction with Daemon Pods

Daemon Pods wait for the "Accepted" status before applying configurations. This ensures only validated configurations are processed, reducing errors.

After validation, the following section is added to the CRD:

```yaml
status:
  state: Initialized/Accepted/Errorred
  reason: <error reason if any>
  acceptedSpec: <Operator accepted last known spec>
```
