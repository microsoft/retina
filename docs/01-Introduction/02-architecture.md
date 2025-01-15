# Architecture

## Overview

In very simple terms, Retina collects metrics from the machine it's running on and hands them over to be processed and visualized elsewhere (in tools such as Prometheus, Hubble UI or Grafana).

To collect this data, Retina observes and hooks on to system events within the kernel through the use of custom eBPF plugins. The data gathered by the plugins is then transformed into `flow` objects and enriched with Kubernetes context, before being converted to metrics and exported.

## Data Plane

This section discusses how Retina collects its raw data. More specifically, it discusses how the eBPF programs and plugins are used.

The plugins have a very specific scope by design, and Retina is designed to be extendable, meaning it is easy to add in additional plugins if necessary. If there is a plugin missing for your use case, you can create your own! See our [Development page](../07-Contributing/02-development.md) for details on how to get started.

The plugins are responsible for installing the eBPF programs into the host kernel during startup. Each plugin has an associated eBPF program. These eBPF programs collect metrics from events in the kernel level, which are then passed to the user space where they are parsed and converted into a `flow` data structure. Depending on the Control Plane being used, the data will either be sent to a Retina Enricher, or written to an external channel which is consumed by a Hubble observer - more on this in the [Control Plane](#control-plane) section below.

Some examlpes of existing Retina plugins:

- Drop Reason - measures the number of packets/bytes dropped and the reason and the direction of the drop.
- DNS - counts DNS requests/responses by query, including error codes, response IPs, and other metadata.
- Packet Forward - measures packets and bytes passing through the eth0 interface of each node, along with the direction of the packets.

You can check out the rest on the [Plugins](../03-Metrics/plugins/readme.md) page.

!["Retina Data Plane"](./img/data-plane.png "Retina Data Plane")

### Plugin Lifecycle

The Plugin Manager is in charge of starting up all of the plugins, and the Watcher Manager - which in turn starts the watchers. It can also reconcile plugins, which will regenerate the eBPF code and the BPF object.

The lifecycle of a plugins themselves can be summarized as follows:

- Initialize - Initialize eBPF maps. Create sockets / qdiscs / filters etc. Load eBPF programs.
- Start - Read data from eBPF maps and arrays. Send it to the appropriate location depending on the Control Plane.
- Stop - Clean up any resources created and stop any threads.

## Control Plane

This section describes how the collected data from the Data Plane is processed, transformed and used.

Retina currently has two options for the Control Plane:

- [Hubble Control Plane](#hubble-control-plane)
- [Legacy* Control Plane](#legacy-control-plane)

(* The "Legacy" naming will soon be replaced to a more accurate description - [GitHub Issue #1115](https://github.com/microsoft/retina/issues/1115))

| Platform | Default Control Plane  |
|----------|------------------------|
| Windows  | Legacy                 |
| Linux    | Legacy                 |
| Linux - [Advanced Container Networking Services](https://aka.ms/acns)   | Hubble         |

Please refer to the [Installation](../02-Installation/01-Setup.md) page for further setup instructions.

### Hubble Control Plane

Describe the control plane.

### Legacy Control Plane

Describe the control plane.
