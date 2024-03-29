# TraceConfiguration

> **Note:** This feature is currently under experimental development.

## Overview

The `TraceConfiguration` CustomResourceDefinition (CRD) introduces a custom resource named `TraceConfiguration` that enables users to configure packet traces in a Kubernetes cluster. Packet traces can be tailored to specific use cases, offering the flexibility to capture detailed network data for debugging or continuous streaming of traces for security purposes.

## CRD Specification

The full specification for the `MetricsConfiguration` CRD can be found in the [TraceConfiguration CRD](https://github.com/microsoft/retina/blob/main/deploy/manifests/controller/helm/retina/crds/retina.sh_tracesconfigurations.yaml) file.

The `TraceConfiguration` CRD is defined with the following specifications:

- **API Group:** retina.sh
- **API Version:** v1alpha1
- **Kind:** TraceConfiguration
- **Plural:** traceconfigurations
- **Singular:** traceconfiguration
- **Scope:** Namespaced

### Fields

- **spec.traceConfigurations:** Specifies the detailed configuration options for packet tracing. It includes the following properties:
  - `captureLevel`: Specifies the capture level, which can be set to `allPackets` or `firstPacket` (default).
  - `includeLayer7Data`: Indicates whether layer 7 data (HTTP, DNS, TLS) should be included in the trace (default is `false`).
  - `from`: Specifies the source entities from which packets will be captured, including IP blocks, namespaces, pods, and more.
  - `to`: Specifies the destination entities to which packets will be captured, including IP blocks, services, and more.
  - `ports`: Specifies the ports and protocols to capture packets for.

- **spec.tracePoints:** Specifies the types of trace points to capture, such as pod, nodeToPod, and nodeToNetwork.

- **spec.outputConfiguration:** Specifies the output destination and connection configuration for trace data. It includes the following properties:
  - `destination`: Specifies the destination for trace data, which can be `stdout`, `azuretable`, `loganalytics`, or `opentelemetry`.
  - `connectionConfiguration`: Specifies connection-related configuration options.

- **status:** Describes the status of the trace configuration, including the current state, reason, and accepted specification.

## Usage

### Configuring Packet Traces

To configure packet traces, create a YAML manifest file with the desired specifications and apply it to the cluster using `kubectl apply`:

```yaml
apiVersion: retina.sh/v1alpha1
kind: TraceConfiguration
metadata:
  name: example-trace-configuration
spec:
  traceConfigurations:
    - captureLevel: firstPacket
      includeLayer7Data: true
      from:
        - ipBlock:
            cidr: 10.0.0.0/16
          except:
            - 10.0.0.5
      to:
        - namespaceSelector:
            label: value
      ports:
        - port: "80"
          protocol: TCP
  tracePoints:
    - pod
    - nodeToPod
  outputConfiguration:
    destination: stdout
```
