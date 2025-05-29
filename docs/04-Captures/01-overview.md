# Overview

Retina Capture allows users to perform distributed packet captures across the cluster, based on specified Nodes/Pods and other supported filters.

Captures are on-demand and can be output to persistent storage such as the host filesystem, a storage blob or PVC.

There are two methods for triggering a Capture:

- [CLI command](./02-cli.md)
- [CRD/YAML configuration](./03-crd.md)

It is also possible to set up a managed storage account when setting up Retina.

- [Managed Storage Account](../04-Captures/04-managed-storage-account.md#setup)
