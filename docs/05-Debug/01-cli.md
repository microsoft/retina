# Debug with Retina CLI

This page describes how to use the Retina CLI debug commands for real-time network troubleshooting.

The debug commands provide live monitoring capabilities that complement Retina's capture functionality.

## Prerequisites

- [Install Retina CLI](../02-Installation/02-CLI.md)
- Linux environment with eBPF support
- Sufficient privileges (may require sudo for eBPF operations)

## Commands

### Debug Drop Events

`kubectl retina debug drop [--flags]` monitors packet drop events in real-time using eBPF.

This command uses the Retina dropreason plugin to capture and display information about dropped network packets, including:

- Drop reason
- Source and destination IP addresses
- Protocol information
- Timestamps
- Packet details

#### Basic Usage

```bash
# Monitor drop events for 30 seconds (default)
kubectl retina debug drop

# Monitor for a specific duration
kubectl retina debug drop --duration=60s

# Save output to a file
kubectl retina debug drop --output=drops.log

# Monitor specific IP addresses
kubectl retina debug drop --ips=10.0.0.1,10.0.0.2

# Skip confirmation prompts
kubectl retina debug drop --confirm=false
```

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `confirm` | bool | true | Confirm before performing invasive operations like port-forwarding |
| `duration` | duration | 30s | Duration to watch for drop events |
| `ips` | strings | | IP addresses to filter for (optional) |
| `metrics-port` | int | 10093 | Metrics port for Retina |
| `namespace` | string | kube-system | Namespace where Retina pods are running |
| `output` | string | | Output file to write drop events (optional) |
| `pod-name` | string | | Specific pod name to monitor (optional) |
| `port-forward` | bool | false | Enable port forwarding for remote monitoring |
| `verbose` | bool | false | Enable verbose output |
| `width` | int | 0 | Console width for formatting (auto-detected if 0) |

#### Output Format

The command displays drop events in a tabular format:

```
TIMESTAMP            SRC_IP          DST_IP          PROTO      DROP_REASON          DETAILS
21:30:15.123         10.0.0.1        10.0.0.2        TCP        DROP(42)             Connection refused
21:30:15.456         10.0.0.3        10.0.0.4        UDP        DROP(13)             No route to host
```

#### Requirements and Limitations

- **eBPF Support**: Requires a Linux environment with eBPF capabilities
- **Privileges**: May require root or elevated privileges for eBPF map creation
- **Kernel Version**: Requires a recent Linux kernel (typically 4.9+)
- **Memory Limits**: May require increasing MEMLOCK limits (`ulimit -l`)

#### Troubleshooting

**Error: "operation not permitted"**
```bash
# Try running with sudo
sudo kubectl retina debug drop

# Or increase memory lock limits
ulimit -l unlimited
```

**Error: "MEMLOCK may be too low"**
```bash
# Increase memory lock limit
echo "* soft memlock unlimited" >> /etc/security/limits.conf
echo "* hard memlock unlimited" >> /etc/security/limits.conf
```

**No events appearing**
- Ensure there is actual network traffic and drops occurring
- Check that the specified IP filters (if any) match actual traffic
- Verify eBPF programs are loaded correctly with verbose output

#### Examples

**Monitor all drop events for 2 minutes:**
```bash
kubectl retina debug drop --duration=2m
```

**Monitor drops for specific IPs and save to file:**
```bash
kubectl retina debug drop --ips=192.168.1.10,192.168.1.20 --output=network-drops.log
```

**Monitor with custom console width:**
```bash
kubectl retina debug drop --width=120
```

**Enable verbose logging for troubleshooting:**
```bash
kubectl retina debug drop --verbose --duration=10s
```