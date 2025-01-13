# Architecture

## Overview

- Retina Agent daemon set
- Retina Operator

## Data Plane

## Control Plane

### Hubble

#### What is Hubble?

Hubble is a fully distributed networking and security observability platform designed for cloud-native workloads. It’s built on top of [Cilium](https://cilium.io/get-started/) and [eBPF](https://ebpf.io/what-is-ebpf/), which allows it to provide deep visibility into the communication and behavior of services and the networking infrastructure.

You can read the official documentation here - [What is Hubble?](https://docs.cilium.io/en/stable/overview/intro/#what-is-hubble)

Both Hubble and Retina, are listed as emerging [eBPF Applications](https://ebpf.io/applications/)!

#### Hubble Beyond Cilium

Hubble has historically been quite tightly coupled with Cilium. This led to challenges if you wanted to use another CNI, or perhaps go beyond Linux.

Retina bridges this gap, and enables the use of a Hubble control plane on any CNI and across both Linux and Windows.

Check out our talk from KubeCon 2024 which goes into this topic even further - [Hubble Beyond Cilium - Anubhab Majumdar & Mathew Merrick, Microsoft](https://www.youtube.com/watch?v=cnNUfQKhYiM)

### Legacy

This is the Legacy control plane.
