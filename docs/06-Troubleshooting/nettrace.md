# Network Trace (nettrace)

>NOTE: `retina nettrace` is an experimental feature. The flags and behavior may change in future versions.

The `retina nettrace` command allows you to trace network issues on a Kubernetes node in real-time using eBPF/bpftrace.

This is useful for debugging connectivity problems such as:
- Packet drops (with reason codes)
- TCP RST events (connection resets)
- Socket errors (ECONNREFUSED, ETIMEDOUT, etc.)
- TCP retransmissions (packet loss indicators)

## Getting Started

Trace network issues on a node:

```shell
# Basic usage - trace all network issues on a node for 30 seconds
kubectl retina nettrace <node-name>

# With custom duration
kubectl retina nettrace <node-name> --duration 60s

# Filter by IP address
kubectl retina nettrace <node-name> --filter-ip 10.224.0.5

# Filter by CIDR
kubectl retina nettrace <node-name> --filter-cidr 10.224.0.0/16

# Output as JSON (for parsing)
kubectl retina nettrace <node-name> -o json

# Specify custom timeout for trace pod operations
kubectl retina nettrace <node-name> --timeout 120s
```

Run `kubectl retina nettrace -h` for full documentation and examples.

## Event Types

The nettrace command captures several types of network events:

### DROP - Packet Drops

Captures packets dropped by the kernel with reason codes. Common reasons include:

| Code | Name | Description |
|------|------|-------------|
| 0 | NOT_SPECIFIED | Unspecified reason |
| 1 | NO_SOCKET | No listening socket |
| 3 | TCP_CSUM | TCP checksum error |
| 6 | NETFILTER_DROP | Dropped by NetworkPolicy/iptables |
| 8 | IP_CSUM | IP checksum error |

The full list of drop reasons is kernel-version specific and is printed at the start of each trace.

### RST_SENT / RST_RECV - TCP Reset Events

Captures TCP RST packets sent or received. These indicate:
- Connection refused (no service listening)
- Connection reset by peer
- Firewall rejecting connections

### SOCK_ERR - Socket Errors

Captures socket-level errors reported to applications:

| Code | Name | Description |
|------|------|-------------|
| 104 | ECONNRESET | Connection reset by peer |
| 110 | ETIMEDOUT | Connection timed out |
| 111 | ECONNREFUSED | Connection refused |
| 113 | EHOSTUNREACH | No route to host |

### RETRANS - TCP Retransmissions

Captures TCP segment retransmissions, which indicate:
- Packet loss in the network
- Network congestion
- Slow or unresponsive peers

The reason code shows the TCP state during retransmission.

## Output Format

### Table Format (default)

```text
TIME         TYPE       REASON             PROBE              SRC -> DST
────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
18:28:12     SOCK_ERR   ECONNREFUSED       inet_sk_error_report 127.0.0.1:35779  ->  127.0.0.1:9999 
18:28:27     DROP       6                  kfree_skb          10.224.0.60:41929  ->  10.224.0.39:80   
18:28:28     RETRANS    2                  tcp_retransmit_skb 10.224.0.60:41929  ->  10.224.0.39:80   
18:28:33     RST_SENT   -                  tcp_send_reset     10.224.0.47:38470  ->  20.161.216.95:443  
```

### JSON Format (`-o json`)

```json
{"time":"18:28:12","type":"SOCK_ERR","reason_code":111,"probe":"inet_sk_error_report","src_ip":"127.0.0.1","src_port":35779,"dst_ip":"127.0.0.1","dst_port":9999}
{"time":"18:28:27","type":"DROP","reason_code":6,"probe":"kfree_skb","src_ip":"10.224.0.60","src_port":41929,"dst_ip":"10.224.0.39","dst_port":80}
```

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--duration` | duration | 30s | Duration to run the trace |
| `--timeout` | duration | 60s | Timeout for trace pod operations |
| `--filter-ip` | string | "" | Filter events by IP address (matches either source or destination) |
| `--filter-cidr` | string | "" | Filter events by CIDR (matches either source or destination, e.g., 10.0.0.0/8) |
| `-o, --output` | string | table | Output format: table or json |
| `--retina-shell-image-repo` | string | (default) | Override the retina-shell image repository |
| `--retina-shell-image-version` | string | (default) | Override the retina-shell image version |

## Example: Debugging NetworkPolicy Drops

When pods can't communicate due to NetworkPolicy:

```shell
# Start tracing on the node where the destination pod runs
kubectl retina nettrace aks-nodepool1-12345678-vmss000000 --duration 60s

# In another terminal, attempt the connection
kubectl exec -it client-pod -- curl http://server-service:80
```

You'll see output like:

```text
18:14:41     DROP       6      kfree_skb          10.224.0.34:33061  ->  10.224.0.49:80   
18:14:42     RETRANS    2      tcp_retransmit_skb 10.224.0.34:33061  ->  10.224.0.49:80   
18:14:42     DROP       6      kfree_skb          10.224.0.34:33061  ->  10.224.0.49:80   
```

The `DROP` with reason code `6` (NETFILTER_DROP) confirms NetworkPolicy is blocking traffic.

## Example: Debugging Connection Refused

When connecting to a service that's not listening:

```shell
kubectl retina nettrace node-name --filter-ip 10.224.0.5
```

```text
18:19:05     RST_RECV   -      tcp_receive_reset  10.224.0.10:35267  ->  10.224.0.5:8080 
18:19:05     SOCK_ERR   ECONNREFUSED inet_sk_error_report 10.224.0.10:35267  ->  10.224.0.5:8080 
```

This shows the TCP RST and corresponding socket error, indicating no service is listening on port 8080.

## Requirements

- Linux nodes (Windows not supported)
- Kernel with BTF support (5.x+ recommended)

## Limitations

- IPv6 filtering not currently supported
- Netfilter table/chain enrichment not available on nftables-based kernels (drop reason code identifies netfilter drops)
