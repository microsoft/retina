# Shell TSG

**EXPERIMENTAL: `retina shell` is an experimental feature, so the flags and behavior may change in future versions.**

The `retina shell` command allows you to start an interactive shell on a Kubernetes node or pod. This runs a container image with many common networking tools installed (`ping`, `curl`, etc.).

## Testing connectivity

Start a shell on a node or inside a pod

```bash
# To start a shell in a node (root network namespace):
kubectl retina shell aks-nodepool1-15232018-vmss000001

# To start a shell inside a pod (pod network namespace):
kubectl retina shell -n kube-system pods/coredns-d459997b4-7cpzx
```

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

Check DNS resolution using `dig`:

```text
root [ / ]# dig example.com +short
93.184.215.14
```

The tools `nslookup` and `drill` are also available if you prefer those.

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

### nftables and iptables

Accessing nftables and iptables rules requires `NET_RAW` and `NET_ADMIN` capabilities.

```bash
kubectl retina shell aks-nodepool1-15232018-vmss000002 --capabilities NET_ADMIN,NET_RAW
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

**If you see the error "Operation not permitted (you must be root)", check that your `kubectl retina shell` command sets `--capabilities NET_RAW,NET_ADMIN`.**

`iptables` in the shell image uses `iptables-nft`, which may or may not match the configuration on the node. For example, Azure Linux 2 maps `iptables` to `iptables-legacy`. To use the exact same `iptables` binary as installed on the node, you will need to `chroot` into the host filesystem (see below).

## Accessing the host filesystem

On nodes, you can mount the host filesystem to `/host`:

```bash
kubectl retina shell aks-nodepool1-15232018-vmss000002 --mount-host-filesystem
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
kubectl retina shell aks-nodepool1-15232018-vmss000002 --mount-host-filesystem --capabilities SYS_CHROOT
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
kubectl retina shell aks-nodepool1-15232018-vmss000002 --mount-host-filesystem --capabilities SYS_CHROOT --host-pid
```

Then `chroot` to the host filesystem and run `systemctl status`:

```text
root [ / ]# chroot /host systemctl status | head -n 2
● aks-nodepool1-15232018-vmss000002
    State: running
```

**If `systemctl` shows an error "Failed to connect to bus: No data available", check that the `retina shell` command has `--host-pid` set and that you have chroot'd to /host.**

## Troubleshooting

### Timeouts

If `kubectl retina shell` fails with a timeout error, then:

1. Increase the timeout by setting `--timeout` flag.
2. Check the pod using `kubectl describe pod` to determine why retina shell is failing to start.

Example:

```bash
kubectl retina shell --timeout 10m node001 # increase timeout to 10 minutes
```

### Firewalls and ImagePullBackoff

Some clusters are behind a firewall that blocks pulling the retina-shell image. To workaround this:

1. Replicate the retina-shell images to a container registry accessible from within the cluster.
2. Override the image used by Retina CLI with the environment variable `RETINA_SHELL_IMAGE_REPO`.

Example:

```bash
export RETINA_SHELL_IMAGE_REPO="example.azurecr.io/retina/retina-shell"
export RETINA_SHELL_IMAGE_VERSION=v0.0.1 # optional, if not set defaults to the Retina CLI version.
kubectl retina shell node0001 # this will use the image "example.azurecr.io/retina/retina-shell:v0.0.1"
```

## Limitations

* Windows nodes and pods are not yet supported.
* `bpftool` and `bpftrace` are not supported.
* The shell image links `iptables` commands to `iptables-nft`, even if the node itself links to `iptables-legacy`.
* `nsenter` is not supported.
* `ip netns` will not work without `chroot` to the host filesystem.
