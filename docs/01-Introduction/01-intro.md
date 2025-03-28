# What is Retina?

## Introduction

Retina is a cloud-agnostic, open-source **Kubernetes Network Observability platform** which enables the use of Hubble as a control plane regardless of the underlying OS or CNI.

Retina can help with DevOps, SecOps and compliance use cases.

It provides a **centralized hub for monitoring application and network health and security** (do we provide security?), catering to Cluster Network/Security Administrators and DevOps Engineers.

Retina **collects customizable telemetry**, which can be exported to **multiple storage options** (such as Prometheus, Azure Monitor, etc.) and **visualized in a variety of ways** (like Grafana, Azure Log Analytics, etc.).

![High Level Architecture](./img/Retina%20Arch.png "High Level Architecture")

## Features

- **[eBPF](https://ebpf.io/what-is-ebpf#what-is-ebpf) based** - Leverages eBPF technologies to collect and provide insights into your Kubernetes cluster with minimal overhead.
- **Platform Agnostic** - Works with any Cloud or On-Prem Kubernetes distribution and supports multiple OS such as Linux, Windows, Azure Linux, etc.
- **CNI Agnostic** - Works with any Container Networking Interfaces (CNIs) like Azure CNI, AWS VPC, etc.
- **Actionable Metrics** - Provides industry-standard Prometheus metrics.
- **Hubble Integration** - Integrates with Cilium's Hubble for additional network insights such as flows logs, DNS, etc
- **Packet Capture** - Distributed packet captures for deep dive troubleshooting

## Why Retina?

Retina lets you **investigate network issues on-demand** and **continuously monitor your clusters**. Here are a couple scenarios where Retina shines, minimizing pain points and investigation time.

### Use Case - Debugging Network Connectivity

*Why can't my Pods connect to each other any more?*

**Typical investigation is time-intensive** and involves manually performing packet captures, where one must first identify the Nodes involved, gain access to each Node, run `tcpdump` commands, and export the results off of each Node.

With Retina, you can **automate this process** with a **single CLI command** or CRD/YAML that can:

- Run captures on all Nodes hosting the Pods of interest.
- Upload each Node's results to a storage blob.

To begin using the CLI, see [Quick Start Installation](../02-Installation/02-CLI.md).

### Use Case - Monitoring Network Health

Retina supports actionable insights through **Prometheus** alerting, **Grafana** dashboards, and more. For instance, you can:

- Monitor dropped traffic in a namespace.
- Alert on a spike in production DNS errors.
- Watch changes in API Server latency while testing your application's scale.
- Notify your Security team if a Pod starts sending too much traffic.

## Telemetry

Retina uses two types of telemetry: metrics and captures.

### Metrics

Retina metrics provide **continuous observability** into:

- Incoming/outcoming traffic
- Dropped packets
- TCP/UDP
- DNS
- API Server latency
- Node/interface statistics

Retina provides both:

- **Basic metrics** - Node-Level (default)
- **Advanced metrics** - Pod-Level (if enabled)

For more info and a list of metrics, see [Metrics](../03-Metrics/modes/modes.md).

The same set of metrics are generated regardless of the underlying OS or CNI.

### Captures

A Retina capture **logs network traffic** and metadata **for the specified Nodes/Pods**.

Captures are **on-demand** and can be output to multiple destinations. For more info, see [Captures](../04-Captures/01-overview.md).

## What is Hubble?

Hubble is a fully distributed networking and security observability platform designed for cloud-native workloads. It’s built on top of [Cilium](https://cilium.io/get-started/) and [eBPF](https://ebpf.io/what-is-ebpf/), which allows it to provide deep visibility into the communication and behavior of services and the networking infrastructure.

You can read the official documentation here - [What is Hubble?](https://docs.cilium.io/en/stable/overview/intro/#what-is-hubble)

Both Hubble and Retina, are listed as emerging [eBPF Applications](https://ebpf.io/applications/)!

Hubble has historically been quite tightly coupled with Cilium. This led to challenges if you wanted to use another CNI, or perhaps go beyond Linux. Retina bridges this gap, and enables the use of a Hubble control plane on any CNI and across both Linux and Windows.

Check out our talk from KubeCon 2024 which goes into this topic even further - [Hubble Beyond Cilium - Anubhab Majumdar & Mathew Merrick, Microsoft](https://www.youtube.com/watch?v=cnNUfQKhYiM)

## Minimum System Requirements

The following are known system requirements for installing Retina:

- Minimum Linux Kernel Version: v5.4.0
