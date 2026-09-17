# Drop Statistics Comparison

This document compares Retina's drop statistics with other observability tools, helping you understand the differences, advantages, and use cases for each approach.

## Overview

Network packet drops can occur at various layers in the network stack, and different tools provide visibility into these drops at different levels of granularity and with different methodologies.

## Retina's Drop Statistics

Retina provides comprehensive drop statistics through two main plugins:

### 1. eBPF-based Drops (`dropreason` plugin)

Retina's `dropreason` plugin uses eBPF to capture packet drops with specific context and reasons:

**Key Features:**
- **Granular drop reasons**: Specific reasons like `IPTABLE_RULE_DROP`, `IPTABLE_NAT_DROP`, `TCP_CONNECT_BASIC`, etc.
- **Direction awareness**: Distinguishes between ingress and egress drops
- **Pod-level context**: In Advanced mode, provides pod, namespace, and workload context
- **Real-time capture**: Uses eBPF hooks to capture drops as they happen

**Metrics provided:**
- `drop_count`: Packet count with reason and direction labels
- `drop_bytes`: Byte count with reason and direction labels
- `adv_drop_count`: Advanced mode with pod-level context
- `adv_drop_bytes`: Advanced mode byte count with pod-level context

**eBPF Hook Points:**
| Reason | Hook Point | Description |
|--------|------------|-------------|
| `IPTABLE_RULE_DROP` | `kprobe/nf_hook_slow` | Packets dropped by iptables rules |
| `IPTABLE_NAT_DROP` | `kretprobe/nf_nat_inet_fn` | Packets dropped by iptables NAT rules |
| `TCP_CONNECT_BASIC` | `kretprobe/tcp_v4_connect` | TCP connection failures |
| `TCP_ACCEPT_BASIC` | `kretprobe/inet_csk_accept` | TCP accept failures |
| `CONNTRACK_ADD_DROP` | `kretprobe/__nf_conntrack_confirm` | Connection tracking failures |

### 2. Interface-level Drops (`linuxutil` plugin)

Retina's `linuxutil` plugin provides traditional interface statistics similar to `ethtool` and `netstat`:

**Data Sources:**
- **ethtool**: Interface hardware statistics including drops
- **netstat**: TCP/UDP connection statistics and drops

**Drop-related metrics:**
- Interface RX/TX drop counters via ethtool
- TCP connection drops from `/proc/net/netstat` (curated list):
  - `ListenDrops`: Dropped connections due to full listen queue
  - `TCPBacklogDrop`: TCP backlog queue drops
  - `TCPRcvQDrop`: TCP receive queue drops
  - `TCPZeroWindowDrop`: TCP zero window drops
  - `TCPDeferAcceptDrop`: TCP defer accept drops
  - `TCPMinTTLDrop`: TCP minimum TTL drops
  - `PFMemallocDrop`: Packet buffer allocation drops
  - `LockDroppedIcmps`: ICMP drops due to locking
  - `InCsumErrors`: Input checksum errors
  - Plus additional MPTCP drops: `AddAddrDrop`, `RmAddrDrop`

## Comparison with Other Tools

### Node Exporter (Prometheus)

**Node Exporter's Approach:**
- Uses `/proc/net/dev` or netlink for interface statistics
- Provides `node_network_receive_drop_total` and `node_network_transmit_drop_total`
- Simple counter metrics without drop reasons
- Note: The node_exporter team has a TODO comment asking "Find out if those drops ever happen on modern switched networks" - highlighting uncertainty about interface-level drop relevance

**Node Exporter Example Metrics:**
```promql
# Total receive drops per interface
node_network_receive_drop_total{device="eth0"}

# Total transmit drops per interface  
node_network_transmit_drop_total{device="eth0"}

# Rate of drops (from node_exporter mixin rules)
rate(node_network_receive_drop_total[5m])
```

**Comparison Table:**

| Feature | Retina eBPF | Retina LinuxUtil | Node Exporter |
|---------|-------------|------------------|---------------|
| **Data Source** | eBPF hooks | ethtool/netstat | /proc/net/dev |
| **Drop Reasons** | ✅ Specific reasons | ✅ TCP-specific | ❌ Generic only |
| **Direction** | ✅ Ingress/Egress | ✅ RX/TX | ✅ RX/TX |
| **Pod Context** | ✅ Advanced mode | ❌ | ❌ |
| **Real-time** | ✅ | ❌ Polling | ❌ Polling |
| **Overhead** | Low | Very Low | Very Low |
| **Kernel Requirements** | Modern eBPF | Standard | Standard |

### Other Observability Tools

#### Cilium Hubble
- Similar eBPF-based approach to Retina
- Flow-based drops with L3/L4 context
- Network policy drops

#### SNMP-based Monitoring
- Uses SNMP MIBs for interface statistics
- Similar to node_exporter but for network devices
- Interface drops without application context

#### eBPF Tools (bcc/bpftrace)
- Ad-hoc drop analysis scripts
- Custom eBPF programs for specific drop scenarios
- Requires manual scripting

## When to Use Each Approach

### Use Retina eBPF Drops When:
- You need to understand **why** packets are being dropped
- You require pod-level visibility in Kubernetes
- You want real-time drop detection
- You're troubleshooting specific network policies or iptables rules
- You need to correlate drops with application context

### Use Retina LinuxUtil Drops When:
- You want traditional interface statistics
- You're monitoring TCP connection health
- You need compatibility with existing monitoring practices
- You want minimal overhead monitoring

### Use Node Exporter When:
- You only need basic interface drop counters
- You're using a general-purpose monitoring stack
- You don't need drop reasons or application context
- You're monitoring non-Kubernetes environments

## Example Use Cases

### Debugging Network Policy Drops
```promql
# Retina: See specific iptables rule drops by pod
networkobservability_drop_count{reason="IPTABLE_RULE_DROP", source_namespace="production"}

# Node Exporter: Only see total interface drops
rate(node_network_receive_drop_total[5m])
```

### Monitoring TCP Connection Health
```promql
# Retina: Specific TCP drops with reasons
networkobservability_tcp_connection_stats{statistic_name="TCPBacklogDrop"}

# Generic: Not available in interface statistics
```

### Interface-level Monitoring
```promql
# Both provide similar interface-level drops
networkobservability_interface_stats{statistic_name="rx_dropped"}
node_network_receive_drop_total
```

## Migration Considerations

If you're migrating from node_exporter to Retina:

1. **Interface drops**: Use `linuxutil` plugin for compatibility
2. **Enhanced visibility**: Add `dropreason` plugin for detailed analysis
3. **Alerting**: Update alert rules to take advantage of drop reasons
4. **Dashboards**: Enhance with pod-level context in Advanced mode

## Best Practices

1. **Use both approaches**: Combine interface-level and eBPF-based drops for comprehensive visibility
2. **Start with interface drops**: Begin monitoring with `linuxutil` for baseline
3. **Add eBPF drops for troubleshooting**: Enable `dropreason` when investigating issues
4. **Correlate with application metrics**: Combine drop statistics with application performance metrics
5. **Set appropriate retention**: eBPF drops can generate more data points

## Conclusion

Retina's approach to drop statistics provides both traditional interface-level monitoring (compatible with tools like node_exporter) and advanced eBPF-based drop analysis with specific reasons and Kubernetes context. This dual approach allows for both broad monitoring and deep troubleshooting capabilities that aren't available in traditional monitoring tools.

The choice between approaches depends on your monitoring requirements, troubleshooting needs, and operational preferences. Many users find value in using both interface-level drops for general monitoring and eBPF drops for detailed analysis.