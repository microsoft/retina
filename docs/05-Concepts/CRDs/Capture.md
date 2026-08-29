# Capture

## Overview

The `Capture` CustomResourceDefinition (CRD) defines a custom resource called `Capture`, which represents the settings of a network trace.
This CRD allows users to specify the configurations for capturing network packets and storing the captured data.

To use the `Capture` CRD, [install Retina](../../02-Installation/01-Setup.md) with capture support.

## CRD Specification

The full specification for the `Capture` CRD can be found in the [Capture CRD](https://github.com/microsoft/retina/blob/main/deploy/standard/manifests/controller/helm/retina/crds/retina.sh_captures.yaml) file.

The `Capture` CRD is defined with the following specifications:

- **API Group:** retina.sh
- **API Version:** v1alpha1
- **Kind:** Capture
- **Plural:** captures
- **Singular:** capture
- **Scope:** Namespaced

### Fields

- **spec.captureConfiguration:** Specifies the configuration for capturing network packets. It includes the following properties:
  - `captureOption`: Lists options for the capture, such as duration, maximum capture size, and packet size.
  - `captureTarget`: Defines the target on which the network packets will be captured. It includes namespace, node, and pod selectors.
  - `filters`: Specifies filters for including or excluding network packets based on IP or port.
  - `includeMetadata`: Indicates whether networking metadata should be captured.
  - `tcpdumpFilter`: Allows specifying a raw tcpdump filter string.

- **spec.outputConfiguration:** Indicates where the captured data will be stored. It includes the following properties:
  - `blobUpload`: Specifies a secret containing the blob SAS URL for storing the capture data.
  - `hostPath`: Stores the capture files into the specified host filesystem.
  - `persistentVolumeClaim`: Mounts a PersistentVolumeClaim into the Pod to store capture files.
  - `s3Upload`: Specifies the configuration for uploading capture files to an S3-compatible storage service, including the bucket name, region, and optional custom endpoint.

- **status:** Describes the status of the capture, including the number of active, failed, and completed jobs, completion time, conditions, and more. Check [capture lifecycle](#capture-lifecycle) for more details.

## Usage

### Creating a Capture

To create a `Capture`, create a YAML manifest file with the desired specifications and apply it to the cluster using `kubectl apply`:

```yaml
apiVersion: retina.sh/v1alpha1
kind: Capture
metadata:
  name: example-capture
spec:
  captureConfiguration:
    captureOption:
      duration: "30s"
      maxCaptureSize: 100
      packetSize: 1500
    captureTarget:
      namespaceSelector:
        matchLabels:
          app: target-app
  outputConfiguration:
    hostPath: /captures
    blobUpload: blob-sas-url
    s3Upload:
      bucket: retina-bucket
      region: ap-northeast-2
      path: retina/captures
      secretName: capture-s3-upload-secret
---
apiVersion: v1
kind: Secret
metadata:
  name: capture-s3-upload-secret
data:
  s3-access-key-id: <based-encode-s3-access-key-id>
  s3-secret-access-key: <based-encode-s3-secret-access-key>
```

### Capture Lifecycle

Once a Capture is created, the capture controller inside retina-operator is responsible for managing the lifecycle of the Capture.
A Capture can be turned into error when errors happens like no required selector is specified, or InProgress when created workload are running, or completed when all workloads are completed.
In implementation, the complete status is defined by setting complete status condition to true, and InProgress is defined as a false complete status condition.

#### Examples of Capture status

- No allowed selectors are specified

```yaml
Status:
  Conditions:
    Last Transition Time:  2023-10-23T06:17:39Z
    Message:               Neither NodeSelector nor NamespaceSelector&PodSelector is set.
    Reason:                otherError
    Status:                True
    Type:                  error
```

- Capture is running in progress

```yaml
Status:
  Active:  2
  Conditions:
    Last Transition Time:  2023-10-23T06:33:56Z
    Message:               2/2 Capture jobs are in progress, waiting for completion
    Reason:                JobsInProgress
    Status:                False
    Type:                  complete
```

- Capture is completed

```yaml
Status:
  Completion Time:  2023-10-23T06:34:40Z
  Conditions:
    Last Transition Time:  2023-10-23T06:34:40Z
    Message:               All 2 Capture jobs are completed
    Reason:                JobsCompleted
    Status:                True
    Type:                  complete
  Succeeded:               2
```
