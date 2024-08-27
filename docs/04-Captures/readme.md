# Captures

## Overview

Retina Capture allows users to capture network traffic/metadata for the specified Nodes/Pods.

Captures are on-demand and can be output to the host filesystem, a storage blob, etc.

## Usage

There are two methods for triggering a Capture:

- [CLI command](#option-1-retina-cli) or
- [CRD/YAML configuration](#option-2-capture-crd-custom-resource-definition).

### Option 1: Retina CLI

Available after [Installing Retina CLI](../02-Installation/02-CLI.md).

See [Capture Command](](../04-Captures/02-CLI.md) for more details.

#### Example

This example captures network traffic for all Linux Nodes, storing the output in the folder */mnt/capture* on each Node.

```shell
kubectl-retina capture create --name capture-test --host-path /mnt/capture --node-selectors "kubernetes.io/os=linux"
```

#### Architecture

For each Capture, a Kubernetes Job is created for each relevant Node (the Node could be selected and/or could be hosting a selected Pod).
The Job's worker Pod runs for the specified duration, captures and wraps the network information into a tarball, and copies the tarball to the specified output location(s).
As a special case, a Kubernetes secret will be created containing a storage blob SAS for security concerns, then mounted to the Pod.

A random hashed name is assigned to each Retina Capture to uniquely label it.

![Overview of Retina Capture without operator](img/capture-architecture-without-operator.png "Overview of Retina Capture without operator")

### Option 2: Capture CRD (Custom Resource Definition)

Available after after [installing Retina](../02-Installation/01-Setup.md) with capture support.

See [Capture CRD](../05-Concepts/CRDs/Capture.md) for more details.

#### Managed Storage Account

To simply the user experience, a managed storage account is configurable when setting up Retina, which can manage the storage account, container and Kubernetes secreting container the blob SAS on behalf the user. Check [managed-storage-account.md] for more details.

#### Example

This example creates a Capture and stores the Capture artifacts into a storage account specified by Blob SAS URL.

Create a secret to store blob SAS URL:

```bash
kubectl create secret generic blob-sas-url --from-file=blob-upload-url=./blob-upload-url.txt
```

in which blob-upload-url.txt stores the Blob SAS URL

Create a Capture specifying the secret created as blobUpload, this example will also store the artifact on the node host path

```yaml
apiVersion: retina.sh/v1alpha1
kind: Capture
metadata:
  name: capture-test
spec:
  captureConfiguration:
    captureOption:
      duration: 30s
    captureTarget:
      nodeSelector:
        matchLabels:
          kubernetes.io/hostname: aks-nodepool1-11396069-vmss000000
  outputConfiguration:
    hostPath: "/tmp/retina"
    blobUpload: blob-sas-url
```

More Retina Capture samples can be found [here](https://github.com/microsoft/retina/tree/main/samples/capture)

#### Architecture

Similarly to Option 1, a Kubernetes Job is created for each relevant Node.

![Overview of Retina Capture with operator](img/capture-architecture-with-operator.png "Overview of Retina Capture with operator")
