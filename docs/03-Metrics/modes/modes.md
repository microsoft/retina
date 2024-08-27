---
sidebar_position: 1
---
# Metric Modes

Retina provides **three modes** with their own metrics and scale capabilities.
Each mode is **fully customizable** (only create the metrics/labels you need).

Note that below, "metric cardinality" refers to the number of time series (metric values with a unique set of labels).
The larger the cardinality, the more load induced on a Prometheus server for instance.

| Mode                                     | Description                                                                                                                                                                                                                        | Scale                                                                                                                                 | Metrics                          | Configuration                                                                                                |
| ---------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| *Basic*                                  | Metrics aggregated by Node.                                                                                                                                                                                                        | Metric cardinality proportional to number of nodes.                                                                                   | [Link to Metrics](./basic.md)    | [Link to Installation](../../02-Installation/01-Setup.md#basic-mode)                                                  |
| *Advanced/Pod-Level with remote context* | Basic metrics plus extra metrics aggregated by source and destination Pod.                                                                                                                                                         | Has scale limitations. Metric cardinality is unbounded (proportional to number of source/destination pairs, including external IPs).  | [Link to Metrics](./advanced.md) | [Link to Installation](../../02-Installation/01-Setup.md#advanced-mode-with-local-context-with-capture-support) |
| *Advanced/Pod-Level with local context*  | Basic metrics plus extra metrics aggregated by "local" Pod (source for outgoing traffic, destination for incoming traffic). Also lets you specify which Pods to observe (create metrics for) with [Annotations](../annotations.md). | Designed for scale. Metric cardinality proportional to number of Pods observed.                                                       | [Link to Metrics](./advanced.md) | [Link to Installation](../../02-Installation/01-Setup.md#advanced-mode-with-local-context-with-capture-support)  |

## Where Do Metrics Come From?

A Retina agent generates metrics on each Node.
The Linux agent inserts eBPF programs and gathers data from Linux utilities.
The Windows agent gathers data from HNS and VFP.
