# Shell

>NOTE: `retina shell` is an experimental feature. The flags and behavior may change in future versions.

The `retina shell` command allows you to start an interactive shell on a Kubernetes node or pod for adhoc debugging.

This runs a container image built from the Dockerfile in the `/shell` directory, with many common networking tools installed (`ping`, `curl`, etc.), as well as specialized tools such as [bpftool](#bpftool), [pwru](#pwru) or [Inspektor Gadget](#inspektor-gadget-ig).

Currently the Retina Shell only works in Linux environments. Windows support will be added in the future.

## Getting Started

Start a shell on a node or inside a pod:

```shell
# To start a shell in a node (root network namespace):
kubectl retina shell <node-name>

# To start a shell inside a pod (pod network namespace):
kubectl retina shell -n kube-system pods/<pod-name>

# To start a shell inside of a node and mount the host file system
kubectl retina shell <node-name> --mount-host-filesystem

# To start a shell inside of a node with extra capabilities
kubectl retina shell <node-name> --capabilities=<CAPABILITIES,COMMA,SEPARATED>
```

For testing, you can override the image used by `retina shell` either with CLI arguments (`--retina-shell-image-repo` and `--retina-shell-image-version`) or environment variables (`RETINA_SHELL_IMAGE_REPO` and `RETINA_SHELL_IMAGE_VERSION`).

Run `kubectl retina shell -h` for full documentation and examples.

## Testing connectivity

Check connectivity using `ping`:

```text
root [ / ]# ping 10.224.0.4
PING 10.224.0.4 (10.224.0.4) 56(84) bytes of data.
64 bytes from 10.224.0.4: icmp_seq=1 ttl=64 time=0.964 ms
64 bytes from 10.224.0.4: icmp_seq=2 ttl=64 time=1.13 ms
64 bytes from 10.224.0.4: icmp_seq=3 ttl=64 time=0.908 ms
64 bytes from 10.224.0.4: icmp_seq=4 ttl=64 time=1.07 ms
64 bytes from 10.224.0.4: icmp_seq=5 ttl=64 time=1.01 ms

--- 10.224.0.4 ping statistics ---
5 packets transmitted, 5 received, 0% packet loss, time 4022ms
rtt min/avg/max/mdev = 0.908/1.015/1.128/0.077 ms
```

Check connectivity to apiserver using `nc` and `curl`:

```text
root [ / ]# nc -zv 10.0.0.1 443
Ncat: Version 7.95 ( https://nmap.org/ncat )
Ncat: Connected to 10.0.0.1:443.
Ncat: 0 bytes sent, 0 bytes received in 0.06 seconds.

root [ / ]# curl -k https://10.0.0.1
{
  "kind": "Status",
  "apiVersion": "v1",
  "metadata": {},
  "status": "Failure",
  "message": "Unauthorized",
  "reason": "Unauthorized",
  "code": 401
}
```

## DNS Resolution

Check DNS resolution using `dig`:

```text
root [ / ]# dig example.com +short
93.184.215.14
```

The tools `nslookup` and `drill` are also available if you prefer those.

## nftables and iptables

Accessing nftables and iptables rules requires `NET_RAW` and `NET_ADMIN` capabilities.

```text
kubectl retina shell <node-name> --capabilities NET_ADMIN,NET_RAW
```

Then you can run `iptables` and `nft`:

```text
root [ / ]# iptables -nvL | head -n 2
Chain INPUT (policy ACCEPT 1191K packets, 346M bytes)
 pkts bytes target     prot opt in     out     source               destination
       
root [ / ]# nft list ruleset | head -n 2
# Warning: table ip filter is managed by iptables-nft, do not touch!
table ip filter {
```

>NOTE: If you see the error "Operation not permitted (you must be root)", check that your `kubectl retina shell` command sets `--capabilities NET_RAW,NET_ADMIN`.

`iptables` in the shell image uses `iptables-nft`, which may or may not match the configuration on the node. For example, Azure Linux 2 maps `iptables` to `iptables-legacy`. To use the exact same `iptables` binary as installed on the node, you will need to `chroot` into the host filesystem (see below).

## Accessing the host filesystem

On nodes, you can mount the host filesystem to `/host`:

```text
kubectl retina shell <node-name> --mount-host-filesystem
```

This mounts the host filesystem (`/`) to `/host` in the debug pod:

```text
root [ / ]# ls /host
NOTICE.txt  bin  boot  dev  etc  home  lib  lib64  libx32  lost+found  media  mnt  opt  proc  root  run  sbin  srv  sys  tmp  usr  var
```

The host filesystem is mounted read-only by default. If you need write access, use the `--allow-host-filesystem-write` flag.

Symlinks between files on the host filesystem may not resolve correctly. If you see "No such file or directory" errors for symlinks, try following the instructions below to `chroot` to the host filesystem.

## Chroot to the host filesystem

`chroot` requires the `SYS_CHROOT` capability:

```bash
kubectl retina shell <node-name> --mount-host-filesystem --capabilities SYS_CHROOT
```

Then you can use `chroot` to switch to start a shell inside the host filesystem:

```text
root [ / ]# chroot /host bash
root@aks-nodepool1-15232018-vmss000002:/# cat /etc/resolv.conf | tail -n 2
nameserver 168.63.129.16
search shncgv2kgepuhm1ls1dwgholsd.cx.internal.cloudapp.net
```

`chroot` allows you to:

* Execute binaries installed on the node.
* Resolve symlinks that point to files in the host filesystem (such as /etc/resolv.conf -> /run/systemd/resolve/resolv.conf)
* Use `sysctl` to view or modify kernel parameters.
* Use `journalctl` to view systemd unit and kernel logs.
* Use `ip netns` to view network namespaces. (However, `ip netns exec` does not work.)

## Systemctl

`systemctl` commands require both `chroot` to the host filesystem and host PID:

```bash
kubectl retina shell <node-name> --mount-host-filesystem --capabilities SYS_CHROOT --host-pid
```

Then `chroot` to the host filesystem and run `systemctl status`:

```text
root [ / ]# chroot /host systemctl status | head -n 2
● aks-nodepool1-15232018-vmss000002
    State: running
```

>NOTE: If `systemctl` shows an error "Failed to connect to bus: No data available", check that the `retina shell` command has `--host-pid` set and that you have chroot'd to /host.

## [pwru](https://github.com/cilium/pwru)

eBPF-based tool for tracing network packets in the Linux kernel with advanced filtering capabilities. It allows fine-grained introspection of kernel state to facilitate debugging network connectivity issues.

Requires the `NET_ADMIN` and `SYS_ADMIN` capabilities.

Capability requirements are based on common eBPF tool practices and not directly from the pwru documentation.

```shell
kubectl retina shell -n kube-system pod/<pod-name> --capabilities=NET_ADMIN,SYS_ADMIN
```

You can then run, for example:

```shell
pwru -h
pwru "tcp and (src port 8080 or dst port 8080)" 
```

## [sysctl](https://man7.org/linux/man-pages/man8/sysctl.8.html)

Tool for viewing and modifying kernel parameters at runtime. `sysctl` is useful for network troubleshooting as it allows you to inspect and tune various kernel networking settings such as IP forwarding, TCP congestion control, buffer sizes, and other network-related parameters.

For viewing kernel parameters, no special capabilities are required:

```shell
kubectl retina shell <node-name>
```

For modifying kernel parameters, you may need the `SYS_ADMIN` capability and/or `chroot` to the host filesystem depending on the parameter:

```shell
kubectl retina shell <node-name> --capabilities=SYS_ADMIN --mount-host-filesystem
```

You can then run, for example:

```shell
# View kernel parameters
sysctl net.ipv4.ip_forward
sysctl -a | grep tcp_congestion
sysctl net.core.rmem_max

# View all networking-related parameters
sysctl -a | grep net

# Modify parameters (may require chroot /host)
sysctl -w net.ipv4.ip_forward=1
```

>NOTE: `sysctl` shows different kernel parameters depending on whether you're running in the container context or the node context. To view/modify the actual node's kernel parameters, use `chroot /host` after mounting the host filesystem. Running `sysctl` without `chroot` shows the container's view, which may have limited or different parameters.

## [bpftool](https://github.com/libbpf/bpftool)

Allows you to list, dump, load BPF programs, etc. Reference utility to quickly inspect and manage BPF objects on your system, to manipulate BPF object files, or to perform various other BPF-related tasks.

Requires the `NET_ADMIN` and `SYS_ADMIN` capabilities.

```shell
kubectl retina shell -n kube-system pod/<pod-name> --capabilities=NET_ADMIN,SYS_ADMIN
```

You can then run for example:

```shell
bpftool -h
bpftool prog show
bpftool map dump id <map_id>
```

## [Inspektor Gadget (ig)](https://inspektor-gadget.io/)

Tools and framework for data collection and system inspection on Kubernetes clusters and Linux hosts using eBPF.

To use `ig`, you need to add the `--mount-host-filesystem`,  `--apparmor-unconfined` and `--seccomp-unconfined` flags, along with the following capabilities:

* `NET_ADMIN`
* `SYS_ADMIN`
* `SYS_RESOURCE`
* `SYSLOG`
* `IPC_LOCK`
* `SYS_PTRACE`
* `NET_RAW`

```shell
kubectl retina shell <node-name> --capabilities=NET_ADMIN,SYS_ADMIN,SYS_RESOURCE,SYSLOG,IPC_LOCK,SYS_PTRACE,NET_RAW  --mount-host-filesystem --apparmor-unconfined --seccomp-unconfined
```

You can then run for example:

```shell
ig -h
ig run trace_dns:latest
```

## [mpstat](https://www.man7.org/linux/man-pages/man1/mpstat.1.html)

Tool for detailed reporting of processor-related statistics. `mpstat` is useful for network troubleshooting because it shows how much CPU time is spent handling SoftIRQs, which are often triggered by network traffic, helping identify interrupt bottlenecks or imbalanced CPU usage. SoftIRQs (Software Interrupt Requests) are a type of deferred interrupt handling mechanism in the Linux kernel used to process time-consuming tasks—like network packet handling or disk I/O—outside the immediate hardware interrupt context, allowing faster and more efficient interrupt processing without blocking the system.

This example usage of `mpstat` monitors CPU usage statistics, specifically focusing on SoftIRQ usage, across all CPU cores, sampled every 1 second, for 5 intervals.

```shell
mpstat -P ALL 1 5 | grep -E '(CPU|%soft|Average)'
```

## Troubleshooting

### Timeouts

If `kubectl retina shell` fails with a timeout error, then:

1. Increase the timeout by setting `--timeout` flag.
2. Check the pod using `kubectl describe pod` to determine why retina shell is failing to start.

Example:

```bash
# increase timeout to 10 minutes
kubectl retina shell --timeout 10m <node-name>
```

### Firewalls and ImagePullBackoff

Some clusters are behind a firewall that blocks pulling the retina-shell image. To workaround this:

1. Replicate the retina-shell images to a container registry accessible from within the cluster.
2. Override the image used by Retina CLI with the environment variable `RETINA_SHELL_IMAGE_REPO`.

Example:

```bash
export RETINA_SHELL_IMAGE_REPO="example.azurecr.io/retina/retina-shell"
# optional, if not set defaults to the Retina CLI version.
export RETINA_SHELL_IMAGE_VERSION=v0.0.1
# this will use the image "example.azurecr.io/retina/retina-shell:v0.0.1"
kubectl retina shell <node-name>
```

## Limitations

* Windows nodes and pods are not yet supported.
* `bpftrace` not yet supported.
* The shell image links `iptables` commands to `iptables-nft`, even if the node itself links to `iptables-legacy`.
* `nsenter` is not supported.
* `ip netns` will not work without `chroot` to the host filesystem.
