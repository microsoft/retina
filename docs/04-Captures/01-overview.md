# Overview

Retina Capture allows users to perform distributed packet captures across the cluster, based on specified Nodes/Pods and other supported filters.

Captures are on-demand and can be output to persistent storage such as the host filesystem, a storage blob or PVC.

There are two methods for triggering a Capture:

- [CLI command](./02-cli.md)
- [CRD/YAML configuration](./03-crd.md)

It is also possible to set up a managed storage account when setting up Retina.

- [Managed Storage Account](../04-Captures/04-managed-storage-account.md#setup)

## Capture Jobs

A packet capture can cover multiple Nodes. This can be explicitly specified by using `node-selectors`. It could also be implicit - for example when using `pod-selectors` and the targetted Pods are hosted across different Nodes.

Whenever a capture is initiated, a Kubernetes Job is created on each relevant Node.

The Job's worker Pod runs for the specified duration, captures and wraps the network information into a tarball. It then copies the tarball to the specified output location(s).

As a special case, a Kubernetes secret will be created containing a storage blob SAS for security concerns, then mounted to the Pod.

A random hashed name is assigned to each Retina Capture job to uniquely label it. For example, a capture named `sample-capture` could result in a job called `sample-capture-s7n8q`.

Corresponding architecture diagrams are present within the [CLI command](./02-cli.md) and [CRD/YAML configuration](./03-crd.md) docs.
