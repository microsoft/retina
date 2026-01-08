# Performance Troubleshooting

This guide helps diagnose and address potential performance issues when running Retina, particularly the `packetparser` plugin on high-core-count systems.

## Background

Community users have reported performance considerations when running the `packetparser` plugin (used in Advanced metrics mode) on systems with high CPU core counts under sustained network load. For detailed background, see the [`packetparser` performance considerations](../03-Metrics/plugins/Linux/packetparser.md#performance-considerations).

## Symptoms to Monitor

Watch for these indicators after deploying Retina:

- **Decreased network throughput** compared to baseline
- **High CPU usage** by Retina agent pods
- **Elevated context switches** on nodes running Retina
- **Increased latency** in network-intensive applications

## Diagnostic Steps

### Step 1: Identify Your Configuration

Check which plugins are enabled:

```bash
kubectl get configmap retina-config -n kube-system -o yaml | grep enabledPlugin
```

If `packetparser` is enabled, you're running Advanced metrics mode which is more resource-intensive.

### Step 2: Check Node Specifications

```bash
# Check core count on nodes
kubectl get nodes -o custom-columns=NAME:.metadata.name,CPU:.status.capacity.cpu

# Identify nodes with high core counts (32+)
kubectl get nodes -o json | jq '.items[] | select((.status.capacity.cpu | tonumber) >= 32) | {name: .metadata.name, cpu: .status.capacity.cpu}'
```

### Step 3: Monitor Retina Resource Usage

```bash
# Check CPU and memory usage of Retina pods
kubectl top pods -n kube-system -l app=retina

# For more detailed analysis, check specific pod on a node
RETINA_POD=$(kubectl get pods -n kube-system -l app=retina -o jsonpath='{.items[0].metadata.name}')
kubectl top pod $RETINA_POD -n kube-system
```

### Step 4: Establish Performance Baseline

Before and after Retina deployment, measure:

- Network throughput (using your application's metrics or tools like iperf3)
- Application response times
- CPU utilization on nodes

## Mitigation Options

If you observe performance impact, consider these approaches:

### Option 1: Use Basic Metrics Mode (Recommended)

Basic metrics mode provides node-level observability without the `packetparser` plugin:

```bash
# Reinstall or upgrade Retina without packetparser
helm upgrade retina oci://ghcr.io/microsoft/retina/charts/retina \
    --set enabledPlugin_linux="\[dropreason\,packetforward\,linuxutil\,dns\]" \
    --reuse-values
```

**Trade-off:** You'll have node-level metrics only, not pod-level metrics.

### Option 2: Enable Data Sampling

Reduce event volume by sampling packets:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: retina-config
  namespace: kube-system
data:
  config.yaml: |
    dataSamplingRate: 10  # Sample 1 out of every 10 packets
```

**Trade-off:** Reduced data granularity, but lower overhead.

### Option 3: Use High Data Aggregation Level

Reduce events at the eBPF level:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: retina-config
  namespace: kube-system
data:
  config.yaml: |
    dataAggregationLevel: "high"
```

**Trade-off:** Disables host interface monitoring; API server latency metrics may be less reliable.

### Option 4: Selective Deployment

Deploy Retina only on nodes where you need detailed observability:

```yaml
# Use node selectors or taints/tolerations
apiVersion: apps/v1
kind: DaemonSet
spec:
  template:
    spec:
      nodeSelector:
        retina-enabled: "true"
```

## Advanced Diagnostics

### Inspecting eBPF Maps

To see what data structures Retina is using:

```bash
# Access the node
kubectl debug node/<node-name> -it --image=ubuntu

# In the debug container, enter the host namespace
chroot /host

# List BPF maps (requires bpftool)
bpftool map list | grep retina

# Check the packetparser map type
bpftool map show name retina_packetparser_events
```

Currently, `packetparser` uses `BPF_MAP_TYPE_PERF_EVENT_ARRAY`.

### Monitoring Event Rates (Advanced)

If you have bpftrace available on nodes:

```bash
# Monitor perf_event activity
sudo bpftrace -e '
  kprobe:perf_event_output { @events = count(); }
  interval:s:5 { print(@events); clear(@events); }
'
```

High event rates may correlate with increased CPU usage.

## Reporting Issues

If you experience performance issues, please report them with:

1. **Node specifications**: CPU count, memory, kernel version
2. **Retina configuration**: Version, enabled plugins, configuration settings
3. **Workload characteristics**: Network throughput, number of pods, traffic patterns
4. **Performance metrics**: CPU usage, network throughput before/after, specific observations

Open an issue at: <https://github.com/microsoft/retina/issues>

## Further Resources

- [Packetparser Performance Considerations](../03-Metrics/plugins/Linux/packetparser.md#performance-considerations)
- [Data Aggregation Levels](../05-Concepts/data-aggregation.md)
- [Configuration Options](../02-Installation/03-Config.md)
