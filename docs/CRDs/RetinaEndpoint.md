# RetinaEndpoint CRD

## Overview

The `RetinaEndpoint` CustomResourceDefinition (CRD) defines a custom resource called `RetinaEndpoint`, which is a representation of a Kubernetes Pod. This CRD allows users to define and manage additional information about Pod  beyond the default Kubernetes metadata as well as reduce the complexity.

In large-scale API servers, each Retina Pod needs to learn about cluster state, which requires the API server to push a lot of data for each list and watch event. To address this issue, we need to reduce the Pod object and convert it to a more manageable `RetinaEndpoint` object.

## CRD Specification

The full specification for the `RetinaEndpoint` CRD can be found in the [RetinaEndpoint CRD]( https://github.com/microsoft/retina/blob/main/deploy/legacy/manifests/controller/helm/retina/crds/retina.sh_retinaendpoints.yaml) file.

The `RetinaEndpoint` CRD is defined with the following specifications:

- **API Group:** retina.sh
- **API Version:** v1alpha1
- **Kind:** RetinaEndpoint
- **Plural:** retinaendpoints
- **Singular:** retinaendpoint
- **Scope:** Namespaced

### Fields

- **spec.containers:** An array of container objects associated with the endpoint. Each container object has an `id` and a `name`.

- **spec.labels:** A set of labels to apply to the endpoint.

- **spec.nodeIP:** The IP address of the node associated with the endpoint.

- **spec.ownerReferences:** An array of owner references indicating the resources that own the endpoint.

- **spec.podIP:** The IP address of the Pod associated with the endpoint.

## Usage

### Creating a RetinaEndpoint

To create a `RetinaEndpoint`, create a YAML manifest file with the desired specifications and apply it to the cluster using `kubectl apply`:

```yaml
apiVersion: retina.sh/v1alpha1
kind: RetinaEndpoint
metadata:
  name: example-retinaendpoint
  namespace: test
  labels:
    app: example-app
spec:
  containers:
    - id: container-1
      name: web
  nodeIP: 192.168.1.10
  ownerReferences:
    - apiVersion: v1
      kind: Pod
      name: example-pod
  podIP: 10.1.2.3

```

## RetinaEndpoint Workflow

In high-scale Kubernetes environments, the scenario where each DaemonSet Pod monitors all Pods within a cluster can lead to a significant volume of data propagation. This situation can potentially strain the API server and cause performance issues.
To address this challenge, an **Operator Pod** is introduced to streamline data flow.

The **Operator** plays a pivotal role in transforming extensive Pod objects into succinct `RetinaEndpoints`. These `RetinaEndpoints` contain only the crucial fields required for Retina functionality. This approach efficiently reduces data size, enabling rapid propagation across all nodes. The subsequent sections delve into the intricacies of this process.

### Workflow Steps

1. **Operator's Monitoring**: When advanced features are enabled, the **Operator** component will monitor the Kubernetes cluster for Pods. It is the only one that can watch and update the `RetinaEndpoint` Resource.

2. **Data Extraction and Transformation**: The **Operator** extracts relevant Pod information and uses this data to generate a corresponding `RetinaEndpoint` CustomResource (CR) object.

3. **Data Backup**: The newly created `RetinaEndpoint` CR object is backed up to the Kubernetes API server by the **Operator**.

4. **Daemon's Role**: The K8s watchers in daemon set Pods will list and read `RetinaEndpoints` and Service objects from the API server.

5. **Data Enrichment**: Subsequently, these K8s watchers convert these objects into an internal cache, enriching kernel data for efficient usage.
