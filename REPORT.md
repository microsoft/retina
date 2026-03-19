# Security Audit Report - Retina

**Date:** January 6, 2026  
**Auditor:** Red Team Security Analysis  
**Repository:** microsoft/retina  
**Scope:** Comprehensive security and stability analysis

---

## Executive Summary

This report documents security vulnerabilities, stability concerns, and potential attack vectors identified in the Retina network observability platform. The findings range from **CRITICAL** to **LOW** severity and include issues related to privilege escalation, command injection, insecure defaults, and potential denial-of-service vectors.

**Total Findings: 56**
- Critical: 4
- High: 9 (including 3 user-facing endpoint issues)
- Medium: 18 (including 3 eBPF-specific, 3 user-facing, 5 supply chain)
- Low: 20 (including 5 eBPF-specific, 3 user-facing, 4 supply chain)
- Stability: 5

The analysis covers:
- Go codebase security review
- eBPF/C kernel-level program analysis
- User-facing endpoint and API security
- CRD validation and admission control gaps
- Supply chain and infrastructure security
- Windows-specific code paths and PowerShell scripts
- Configuration parsing and environment variable handling
- Test code security patterns

---

## Table of Contents

1. [Critical Findings](#critical-findings)
2. [High Severity Findings](#high-severity-findings)
3. [Medium Severity Findings](#medium-severity-findings)
4. [Low Severity Findings](#low-severity-findings)
5. [Stability Concerns](#stability-concerns)
6. [eBPF/C Security Findings](#ebpfc-security-findings)
7. [User-Facing Endpoint Security Analysis](#user-facing-endpoint-security-analysis)
8. [Supply Chain & Infrastructure Security Findings](#supply-chain--infrastructure-security-findings)
9. [Additional Findings from Deep Code Review](#additional-findings-from-deep-code-review)
10. [Recommendations](#recommendations)

---

## Critical Findings

### 1. Shell Component Allows Privileged Container Creation with Host Access

**Location:** [shell/manifests.go](shell/manifests.go#L27-L100)

**Description:** The `retina shell` command can create pods with:
- `HostNetwork: true`
- `HostPID: true` (configurable)
- Host filesystem mounted at `/host` with optional write access
- `AppArmorProfile: Unconfined` (configurable)
- `SeccompProfile: Unconfined` (configurable)
- Universal tolerations allowing scheduling on any node

**Risk:** An attacker with access to the CLI and cluster credentials could create a fully privileged pod that can:
- Access all host network traffic
- View all processes on the host
- Read/write the entire host filesystem
- Escape container isolation

**Code:**
```go
pod.Spec.Volumes = append(pod.Spec.Volumes,
    v1.Volume{
        Name: "host-filesystem",
        VolumeSource: v1.VolumeSource{
            HostPath: &v1.HostPathVolumeSource{
                Path: "/",
            },
        },
    },
```

**Recommendation:**
- Implement RBAC checks before allowing shell creation
- Add audit logging for shell command usage
- Consider requiring explicit cluster-admin confirmation
- Document security implications prominently

---

### 2. Capture Jobs Run with Elevated Privileges

**Location:** [pkg/capture/crd_to_job.go](pkg/capture/crd_to_job.go#L138-L148)

**Description:** Capture workload pods are created with:
- `HostNetwork: true`
- `HostIPC: true`
- `NET_ADMIN` and `SYS_ADMIN` capabilities
- Universal tolerations

**Code:**
```go
Spec: corev1.PodSpec{
    HostNetwork:                   true,
    HostIPC:                       true,
    ...
    SecurityContext: &corev1.SecurityContext{
        Capabilities: &corev1.Capabilities{
            Add: []corev1.Capability{
                "NET_ADMIN", "SYS_ADMIN",
            },
        },
    },
```

**Risk:** These elevated privileges are necessary for packet capture but could be exploited if an attacker gains control of the capture job image or the Capture CRD.

**Recommendation:**
- Implement admission control to validate capture images
- Use Pod Security Standards/Policies to restrict which namespaces can run captures
- Add image signature verification

---

### 3. Remote Script Download in Windows Metadata Collection

**Location:** [pkg/capture/provider/network_capture_win.go](pkg/capture/provider/network_capture_win.go#L224-L248)

**Description:** The Windows network capture downloads and executes a PowerShell script from GitHub without integrity verification:

```go
url := "https://raw.githubusercontent.com/microsoft/SDN/master/Kubernetes/windows/debug/collectlogs.ps1"
resp, err := http.Get(url)
...
_, err = io.Copy(out, resp.Body)
```

**Risk:**
- Man-in-the-middle attack could inject malicious script
- Repository compromise would affect all Retina deployments
- Script uses `master` branch (mutable reference)

**Recommendation:**
- Pin to a specific commit hash or release tag
- Implement checksum verification
- Bundle the script in the container image
- Use HTTPS with certificate pinning

---

### 4. pprof Endpoints Exposed Without Authentication

**Location:** 
- [pkg/server/server.go](pkg/server/server.go#L43-L52)
- [operator/cmd/standard/deployment.go](operator/cmd/standard/deployment.go#L248-L258)

**Description:** pprof debugging endpoints are exposed on the metrics server without authentication:

```go
rt.mux.HandleFunc("/debug/pprof/", pprof.Index)
rt.mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
rt.mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
```

**Risk:**
- Information disclosure (goroutine dumps, heap profiles)
- Denial of service via CPU profiling requests
- Potential memory exhaustion via heap dumps

**Recommendation:**
- Disable pprof in production by default
- Require authentication for pprof endpoints
- Bind to localhost only or use network policies

---

## High Severity Findings

### 5. Insecure HTTP Server Configuration

**Location:** [operator/cmd/standard/deployment.go](operator/cmd/standard/deployment.go#L256-L258)

**Description:** The pprof HTTP server lacks timeout configuration:

```go
if err := http.ListenAndServe(":8082", pprofmux); err != nil {
    panic(err)
}
```

**Risk:** Susceptible to Slowloris and similar denial-of-service attacks.

**Recommendation:**
```go
srv := &http.Server{
    Addr:         ":8082",
    Handler:      pprofmux,
    ReadTimeout:  10 * time.Second,
    WriteTimeout: 10 * time.Second,
}
```

---

### 6. Potential Command Injection in tcpdump Filter

**Location:** [pkg/capture/provider/network_capture_unix.go](pkg/capture/provider/network_capture_unix.go#L75-L98)

**Description:** The tcpdump filter and raw filter are constructed from environment variables and appended directly to command arguments:

```go
if tcpdumpRawFilter := os.Getenv(captureConstants.TcpdumpRawFilterEnvKey); len(tcpdumpRawFilter) != 0 {
    tcpdumpRawFilterSlice := strings.Split(tcpdumpRawFilter, " ")
    captureStartCmd.Args = append(captureStartCmd.Args, tcpdumpRawFilterSlice...)
}

if len(filter) != 0 {
    captureStartCmd.Args = append(
        captureStartCmd.Args,
        filter,
    )
}
```

**Risk:** While the environment variables are set by the operator, a compromised Capture CRD could inject malicious arguments. The filter content is not sanitized.

**Recommendation:**
- Implement strict filter validation/sanitization
- Use allowlist of permitted filter patterns
- Consider using a tcpdump wrapper with limited options

---

### 7. Similar Command Injection Risk in Windows netsh

**Location:** [pkg/capture/provider/network_capture_win.go](pkg/capture/provider/network_capture_win.go#L79-L92)

**Description:** Similar to the Unix implementation, Windows netsh filter arguments are split and appended without validation:

```go
if len(filter) != 0 {
    netshFilterSlice := strings.Split(filter, " ")
    captureStartCmd.Args = append(captureStartCmd.Args, netshFilterSlice...)
}
```

**Recommendation:** Same as above - validate filter inputs.

---

### 8. Weak Blob SAS URL Validation

**Location:** [pkg/capture/outputlocation/blob.go](pkg/capture/outputlocation/blob.go#L117-L127)

**Description:** The blob SAS URL validation only checks that the URL has a path:

```go
func validateBlobSASURL(blobSASURL string) error {
    u, err := url.Parse(blobSASURL)
    if err != nil {
        return err
    }
    path := strings.TrimPrefix(u.Path, "/")
    if path == "" {
        return fmt.Errorf("invalid blob SAS URL")
    }
    return nil
}
```

**Risk:** 
- Attacker could exfiltrate capture data to their own storage account
- No validation of the storage account domain

**Recommendation:**
- Validate the hostname matches expected Azure Blob domains
- Consider allowlisting specific storage accounts
- Add audit logging for capture uploads

---

### 9. Downloaded File Permissions Too Permissive

**Location:** [cli/cmd/capture/download.go](cli/cmd/capture/download.go#L69)

**Description:** Downloaded capture files are written with 0644 permissions:

```go
err = os.WriteFile(v.Name, blobData, 0o644)
```

**Risk:** Capture data may contain sensitive network traffic and should have restricted permissions.

**Recommendation:** Use 0600 permissions for downloaded capture files.

---

## Medium Severity Findings

### 10. BPF Filesystem Mount Permissions

**Location:** [pkg/bpf/setup_linux.go](pkg/bpf/setup_linux.go#L22-L25)

**Description:** The BPF map path is created with 0755 permissions:

```go
err = os.MkdirAll(plugincommon.MapPath, 0o755)
```

**Risk:** Other processes on the node could potentially read BPF maps.

**Recommendation:** Use more restrictive permissions (0700) for the BPF map directory.

---

### 11. Panic Recovery Could Leak Sensitive Information

**Location:** [pkg/telemetry/telemetry.go](pkg/telemetry/telemetry.go#L117-L137)

**Description:** The panic recovery function sends the full stack trace to Application Insights:

```go
func TrackPanic() {
    if r := recover(); r != nil {
        message := fmt.Sprintf("Panic caused by: %v , Stacktrace %s", r, string(debug.Stack()))
        trace := appinsights.NewTraceTelemetry(message, appinsights.Critical)
        ...
        client.Track(trace)
```

**Risk:** Stack traces may contain sensitive information (IP addresses, hostnames, internal paths).

**Recommendation:**
- Sanitize stack traces before sending to telemetry
- Ensure Application Insights ID is kept confidential
- Document telemetry data collection clearly

---

### 12. Environment Variable Leakage in Telemetry

**Location:** [pkg/telemetry/telemetry.go](pkg/telemetry/telemetry.go#L143)

**Description:** Environment properties are collected and could potentially include sensitive values.

**Recommendation:** Explicitly filter environment properties before sending to telemetry.

---

### 13. gRPC Insecure Credentials in pktmon Plugin

**Location:** [pkg/plugin/pktmon/pktmon_windows.go](pkg/plugin/pktmon/pktmon_windows.go#L23)

**Description:** The pktmon plugin uses insecure gRPC credentials:

```go
import "google.golang.org/grpc/credentials/insecure"
```

**Risk:** Local socket communication could be intercepted.

**Recommendation:** Use Unix socket permissions or add authentication for local gRPC communication.

---

### 14. Missing Input Validation in CLI Shell Command

**Location:** [cli/cmd/shell.go](cli/cmd/shell.go#L147-L158)

**Description:** The shell image repo and version can be set via environment variables without validation:

```go
if envRepo := os.Getenv("RETINA_SHELL_IMAGE_REPO"); envRepo != "" {
    retinaShellImageRepo = envRepo
}
if envVersion := os.Getenv("RETINA_SHELL_IMAGE_VERSION"); envVersion != "" {
    retinaShellImageVersion = envVersion
}
```

**Risk:** An attacker who can set environment variables could inject a malicious image.

**Recommendation:**
- Validate image repository against an allowlist
- Add image signature verification
- Warn users when using non-default images

---

### 15. Kubernetes Service Account Token Handling in Windows

**Location:** [windows/setkubeconfigpath.ps1](windows/setkubeconfigpath.ps1)

**Description:** The PowerShell script reads and embeds service account tokens into a kubeconfig file:

```powershell
$token = Get-Content -Path $env:CONTAINER_SANDBOX_MOUNT_POINT\var\run\secrets\kubernetes.io\serviceaccount\token
```

**Risk:** Token is written to disk and persisted in the kubeconfig file.

**Recommendation:**
- Ensure kubeconfig file has restricted permissions
- Consider using in-memory token handling
- Clear token from memory after use

---

## Low Severity Findings

### 16. Default Metrics Port Binding

**Location:** [cmd/root.go](cmd/root.go#L39)

**Description:** Metrics endpoint binds to all interfaces by default:

```go
rootCmd.Flags().StringVar(&metricsAddr, "metrics-bind-address", ":18080", ...)
```

**Recommendation:** Consider defaulting to localhost and requiring explicit configuration for external access.

---

### 17. Hardcoded Timeouts

**Location:** Multiple files

**Description:** Various hardcoded timeouts that could lead to resource exhaustion:
- [pkg/capture/crd_to_job.go](pkg/capture/crd_to_job.go#L118): 30-minute termination grace period
- [shell/shell.go](shell/shell.go): Default 30-second timeout

**Recommendation:** Make timeouts configurable via configuration file.

---

### 18. Missing Rate Limiting

**Location:** [pkg/server/server.go](pkg/server/server.go)

**Description:** The HTTP server lacks rate limiting, potentially allowing DoS attacks.

**Recommendation:** Add middleware for rate limiting on endpoints.

---

### 19. Hubble Download Without Certificate Verification

**Location:** [controller/Dockerfile](controller/Dockerfile#L112-L115)

**Description:** Hubble binary is downloaded with checksum verification, but `--no-check-certificate` is used:

```dockerfile
RUN wget --no-check-certificate https://github.com/cilium/hubble/releases/download/$HUBBLE_VERSION/hubble-linux-${HUBBLE_ARCH}.tar.gz
```

**Risk:** While checksum is verified, the initial download could be intercepted.

**Recommendation:** Remove `--no-check-certificate` flag or use a base image with proper CA certificates.

---

### 20. RBAC Over-Permissions

**Location:** [deploy/standard/manifests/controller/helm/retina/templates/rbac.yaml](deploy/standard/manifests/controller/helm/retina/templates/rbac.yaml)

**Description:** The ClusterRole has broad read access across the cluster:

```yaml
rules:
  - apiGroups: [""]
    resources: ["pods", "services", "replicationcontrollers", "nodes", "namespaces"]
    verbs: ["get", "watch", "list"]
```

**Risk:** Compromised agent could enumerate all cluster resources.

**Recommendation:** Apply principle of least privilege - only request access to necessary resources.

---

## Stability Concerns

### 21. Panic in EnablePProf Function

**Location:** [operator/cmd/standard/deployment.go](operator/cmd/standard/deployment.go#L257)

**Description:** `EnablePProf()` panics on error instead of graceful degradation:

```go
if err := http.ListenAndServe(":8082", pprofmux); err != nil {
    panic(err)
}
```

**Impact:** Could crash the operator if port 8082 is in use.

---

### 22. Missing Error Handling in BPF Cleanup

**Location:** [cmd/standard/daemon.go](cmd/standard/daemon.go#L249)

**Description:** Filter manager stop is best-effort:

```go
defer fm.Stop() //nolint:errcheck // best effort
```

**Impact:** Resources may not be properly cleaned up on shutdown.

---

### 23. Race Condition in Filter Map Singleton

**Location:** [pkg/plugin/filter/filter_map_linux.go](pkg/plugin/filter/filter_map_linux.go#L36-L41)

**Description:** The singleton pattern uses `sync.Once` but the object's state check is not thread-safe:

```go
func Init() (*FilterMap, error) {
    once.Do(func() {
        f = &FilterMap{}
    })
    if f.l == nil {
        f.l = log.Logger().Named("filter-map")
    }
    if f.obj != nil {  // Not thread-safe
        return f, nil
    }
```

**Impact:** Potential race condition during concurrent initialization.

---

### 24. Unbounded Channel Buffers

**Location:** [operator/cmd/standard/deployment.go](operator/cmd/standard/deployment.go#L43-L46)

**Description:** Fixed buffer sizes could lead to blocking:

```go
MaxPodChannelBuffer                  = 250
MaxMetricsConfigurationChannelBuffer = 50
```

**Impact:** In high-scale clusters, channel blocking could cause controller delays.

---

### 25. Memory Leak Potential in Perf Readers

**Location:** Various eBPF plugin implementations

**Description:** Perf buffer readers allocate per-CPU buffers that may not be properly released on error paths.

**Recommendation:** Ensure proper cleanup in all error paths and implement finalizers.

---

## eBPF/C Security Findings

### 26. Pinned BPF Maps Accessible to Other Processes

**Location:** 
- [pkg/plugin/conntrack/_cprog/conntrack.c](pkg/plugin/conntrack/_cprog/conntrack.c#L95)
- [pkg/plugin/filter/_cprog/retina_filter.c](pkg/plugin/filter/_cprog/retina_filter.c#L20)

**Severity:** Medium

**Description:** BPF maps are pinned to `/sys/fs/bpf` with `LIBBPF_PIN_BY_NAME`, making them accessible to any process with appropriate permissions:

```c
__uint(pinning, LIBBPF_PIN_BY_NAME); // needs pinning so this can be access from other processes .i.e debug cli
```

The BPF map directory is created with 0755 permissions ([pkg/bpf/setup_linux.go](pkg/bpf/setup_linux.go#L25)), allowing other processes on the node to potentially:
- Read connection tracking data
- Enumerate IP filter entries
- Infer network policies and traffic patterns

**Risk:** Information disclosure of network observability data to other processes on the same node.

**Recommendation:**
- Use 0700 permissions for the BPF map directory
- Consider using unique names or namespacing for BPF maps
- Implement access controls if BPF map sharing is intentional

---

### 27. Large BPF Map Sizes Could Enable Resource Exhaustion

**Location:** 
- [pkg/plugin/conntrack/_cprog/conntrack.h](pkg/plugin/conntrack/_cprog/conntrack.h#L30): `CT_MAP_SIZE = 262144`
- [pkg/plugin/dropreason/_cprog/drop_reason.c](pkg/plugin/dropreason/_cprog/drop_reason.c#L60-L93): Multiple maps with `max_entries = 16384`

**Severity:** Low

**Description:** Large BPF map allocations could consume significant kernel memory:

```c
#define CT_MAP_SIZE 262144  // 262K entries in conntrack map

__uint(max_entries, 16384);  // Per-event arrays
```

**Risk:** 
- Memory pressure on the node kernel
- Potential for OOM conditions in memory-constrained environments
- No apparent limits on map entry creation rate

**Recommendation:**
- Make map sizes configurable based on deployment environment
- Implement monitoring for BPF map utilization
- Consider using LRU maps more aggressively for auto-eviction

---

### 28. Use of Legacy bpf_probe_read Without Bounds Validation

**Location:** [pkg/plugin/dropreason/_cprog/drop_reason.c](pkg/plugin/dropreason/_cprog/drop_reason.c#L107)

**Severity:** Low

**Description:** The `member_read` macro uses `bpf_probe_read` without explicit bounds checking:

```c
#define member_read(destination, source_struct, source_member) \
    do                                                         \
    {                                                          \
        bpf_probe_read(                                        \
            destination,                                       \
            sizeof(source_struct->source_member),              \
            member_address(source_struct, source_member));     \
    } while (0)
```

While the BPF verifier provides safety guarantees, this pattern relies on correct usage of the macro. Modern best practices recommend using `BPF_CORE_READ` or `bpf_probe_read_kernel` for kernel memory access.

**Risk:** Potential for subtle memory access bugs if the macro is misused.

**Recommendation:**
- Migrate to CO-RE (Compile Once, Run Everywhere) patterns using `BPF_CORE_READ` consistently
- Add comments documenting the expected types for the macro parameters

---

### 29. TCP Options Parsing Loop Without Verifier Bounds

**Location:** [pkg/plugin/packetparser/_cprog/packetparser.c](pkg/plugin/packetparser/_cprog/packetparser.c#L40-L101)

**Severity:** Low

**Description:** The TCP timestamp parsing function uses a bounded loop (`#pragma unroll`) to iterate through TCP options:

```c
#pragma unroll
for (i = 0; i < MAX_TCP_OPTIONS_LEN; i++) {
    if (tcp_options_cur_ptr + 1 > (__u8 *)tcp_opt_end_ptr || tcp_options_cur_ptr + 1 > (__u8 *)data_end) {
        return -1;
    }
    opt_kind = *tcp_options_cur_ptr;
    ...
```

The code includes appropriate bounds checks, but the loop unrolling with `MAX_TCP_OPTIONS_LEN = 40` iterations creates a large instruction count in the BPF program.

**Risk:**
- Increased BPF program complexity approaching verifier limits
- May fail verification on older kernels with stricter instruction limits

**Recommendation:**
- Consider reducing the unroll count if TCP timestamp parsing is optional
- Add complexity budget monitoring to CI/CD

---

### 30. Missing NULL Checks in Some BPF Helper Usages

**Location:** [pkg/plugin/dropreason/_cprog/drop_reason.c](pkg/plugin/dropreason/_cprog/drop_reason.c#L270-L283)

**Severity:** Low

**Description:** Some kprobe handlers check for NULL `skb` but don't validate internal pointers before reading:

```c
SEC("kprobe/nf_hook_slow")
int BPF_KPROBE(nf_hook_slow, struct sk_buff *skb, struct nf_hook_state *state)
{
    if (!skb)
        return 0;

    __u16 eth_proto;
    member_read(&eth_proto, skb, protocol);  // skb->head, skb->protocol could be invalid
```

While the BPF verifier ensures memory safety, reading from a freed or corrupted skb could lead to unexpected values.

**Risk:** Incorrect metric attribution if skb is in an unexpected state.

**Recommendation:** Add validation that critical skb fields are non-zero before processing.

---

### 31. Conntrack Entry Manipulation Race Conditions

**Location:** [pkg/plugin/conntrack/_cprog/conntrack.c](pkg/plugin/conntrack/_cprog/conntrack.c#L280-L320)

**Severity:** Medium

**Description:** The connection tracking logic uses `READ_ONCE` and `WRITE_ONCE` for individual field access but performs non-atomic check-then-act sequences:

```c
__u32 eviction_time = READ_ONCE(entry->eviction_time);
if (now >= eviction_time) {
    bpf_map_delete_elem(&retina_conntrack, key);  // Race: entry could be updated between check and delete
    return true;
}
...
if (flags != seen_flags || now - last_report >= CT_REPORT_INTERVAL) {
    // Update multiple fields - not atomic
    if (direction == CT_PACKET_DIR_TX) {
        WRITE_ONCE(entry->flags_seen_tx_dir, flags);
        WRITE_ONCE(entry->last_report_tx_dir, now);
    }
```

**Risk:** 
- Premature connection eviction during high packet rates
- Inconsistent state between tx and rx direction fields
- Potential for missed or duplicate packet reports

**Recommendation:**
- Document the expected behavior under race conditions
- Consider using per-CPU maps for high-frequency counters
- Implement proper locking or use BPF spin locks if consistency is critical

---

### 32. Integer Overflow Check in Connection Timeout

**Location:** [pkg/plugin/conntrack/_cprog/conntrack.c](pkg/plugin/conntrack/_cprog/conntrack.c#L140-L142)

**Severity:** Low

**Description:** The code checks for integer overflow when calculating eviction time:

```c
if (CT_SYN_TIMEOUT > UINT32_MAX - now) {
    return false;
}
new_value.eviction_time = now + CT_SYN_TIMEOUT;
```

This is good practice, but the `bpf_mono_now()` function returns `__u64`, and the eviction_time is stored as `__u32`. This could lead to wrap-around issues after ~136 years of uptime (when the lower 32 bits of the monotonic clock overflow).

**Risk:** Minimal in practice, but could cause connection tracking issues on extremely long-running systems.

**Recommendation:** Consider using `__u64` for eviction times, or document the limitation.

---

### 33. Filter Map Limited to 255 Entries

**Location:** [pkg/plugin/filter/_cprog/retina_filter.c](pkg/plugin/filter/_cprog/retina_filter.c#L19)

**Severity:** Low

**Description:** The IP filter map is limited to 255 entries:

```c
__uint(max_entries, 255);
```

**Risk:** In large clusters with many pods of interest, this limit could cause silent filtering failures where IPs are not tracked.

**Recommendation:**
- Make this configurable or increase the default limit
- Add monitoring for filter map capacity
- Log warnings when approaching the limit

---

### 34. No BPF Program Signing or Verification

**Location:** 
- [pkg/plugin/packetparser/packetparser_linux.go](pkg/plugin/packetparser/packetparser_linux.go#L150-L164)
- [pkg/plugin/dropreason/dropreason_linux.go](pkg/plugin/dropreason/dropreason_linux.go#L156)

**Severity:** Medium

**Description:** eBPF programs are compiled at runtime from C source files and loaded without signature verification:

```go
err = loader.CompileEbpf(ctx, "-target", "bpf", "-Wall", targetArch, "-g", "-O2", "-c", bpfSourceFile, "-o", bpfOutputFile, ...)
...
spec, err := ebpf.LoadCollectionSpec(bpfOutputFile)
```

**Risk:** 
- If an attacker can modify the C source files in the container, they can inject malicious BPF code
- The compiled BPF object files are not verified before loading

**Recommendation:**
- Pre-compile BPF programs and embed them in the container image (which is already done for some architectures)
- Implement integrity verification for BPF object files
- Consider using BPF token-based authorization in future kernels

---

## User-Facing Endpoint Security Analysis

This section analyzes security from the perspective of how users interact with Retina's exposed endpoints and interfaces.

### 35. Capture CRD Lacks Admission Validation

**Location:** [crd/api/v1alpha1/capture_types.go](crd/api/v1alpha1/capture_types.go)

**Severity:** High

**Description:** The Capture CRD accepts user-provided filters without server-side validation:

```go
// TcpdumpFilter is a raw tcpdump filter string.
// +optional
TcpdumpFilter *string `json:"tcpdumpFilter,omitempty"`
```

While MetricsConfiguration has validation in [crd/api/v1alpha1/validations/](crd/api/v1alpha1/validations/), the Capture CRD has **no corresponding validation**. A malicious user with CRD create permissions could:

1. Inject shell metacharacters in the tcpdumpFilter field
2. Specify malicious include/exclude IP filters
3. Create captures that exfiltrate data to attacker-controlled storage

**Attack Vector:** A user with `create` permissions on Capture CRDs (but not cluster-admin) could leverage this to:
- Execute arbitrary commands on capture pods
- Capture sensitive network traffic from other tenants
- Upload captured data to external storage

**Recommendation:**
- Implement ValidatingWebhook for Capture CRDs
- Validate tcpdumpFilter against a strict regex pattern
- Restrict output destinations to pre-approved storage accounts

---

### 36. Blob Download Uses User-Provided Filename Without Sanitization

**Location:** [cli/cmd/capture/download.go](cli/cmd/capture/download.go#L56-L73)

**Severity:** Medium

**Description:** The capture download command writes files using blob names directly from the Azure API:

```go
for _, v := range blobList.Blobs {
    ...
    err = os.WriteFile(v.Name, blobData, 0o644)
```

The blob `v.Name` comes from the Azure storage listing and could contain path traversal sequences like `../../../etc/cron.d/malicious` if an attacker can control blob names.

**Attack Vector:** An attacker who can upload malicious blob names to the capture storage could cause the CLI user to write files outside the intended directory.

**Recommendation:**
- Sanitize blob names using `filepath.Base()` or validate against path traversal
- Reject blob names containing `..` or absolute paths
- Use a dedicated download directory

---

### 37. Metrics Endpoint Exposes Potentially Sensitive Network Topology

**Location:** 
- [pkg/server/server.go](pkg/server/server.go#L59) - `/metrics` endpoint
- [docs/03-Metrics/02-hubble_metrics.md](docs/03-Metrics/02-hubble_metrics.md)

**Severity:** Medium

**Description:** The Prometheus metrics endpoint exposes detailed network topology including:
- Source/destination pod names and namespaces
- IP addresses and ports
- DNS queries and responses
- TCP connection states and latencies

Any entity with network access to the metrics port (18080) can enumerate:
- All active network connections
- Pod-to-pod communication patterns
- External services being accessed
- DNS resolution patterns

**Attack Vector:** An attacker with access to the metrics endpoint could:
- Map the entire cluster network topology
- Identify sensitive services and their connections
- Monitor traffic patterns for reconnaissance
- Identify pods communicating with external services

**Recommendation:**
- Require authentication for metrics endpoint
- Use network policies to restrict access to monitoring infrastructure
- Consider aggregating sensitive labels (e.g., hash pod names)
- Document privacy implications of metric exposure

---

### 38. CLI Config File Parsed Without Validation

**Location:** [cli/cmd/root.go](cli/cmd/root.go#L30-L34)

**Severity:** Low

**Description:** The CLI reads and parses a JSON config file without validation:

```go
file, _ := os.ReadFile(ClientConfigPath)
_ = json.Unmarshal([]byte(file), &config)
RetinaClient = client.NewRetinaClient(config.RetinaEndpoint)
```

Errors are silently ignored. The `RetinaEndpoint` is used directly without URL validation.

**Attack Vector:** An attacker who can modify `.retinactl.json` could:
- Redirect CLI requests to a malicious server (SSRF-like)
- Cause the CLI to send credentials to an attacker-controlled endpoint

**Recommendation:**
- Validate the RetinaEndpoint URL
- Handle and log parsing errors
- Restrict config file permissions to user-only (0600)

---

### 39. HTTP Clients Without Explicit Timeouts in Test/E2E Code

**Location:** [test/e2e/framework/prometheus/prometheus.go](test/e2e/framework/prometheus/prometheus.go#L128)

**Severity:** Low

**Description:** Several HTTP clients are created without explicit timeouts:

```go
client := http.Client{}
```

While this is in test code, if patterns are copied to production, it could lead to resource exhaustion.

**Recommendation:** Always use explicit timeouts: `http.Client{Timeout: 30 * time.Second}`

---

### 40. Capture Output to HostPath Without Path Validation

**Location:** [pkg/capture/outputlocation/hostpath.go](pkg/capture/outputlocation/hostpath.go#L43-L63)

**Severity:** High

**Description:** The HostPath output location reads the destination from an environment variable and writes there without path validation:

```go
hostPath := os.Getenv(string(captureConstants.CaptureOutputLocationEnvKeyHostPath))
...
fileName := filepath.Base(srcFilePath)
fileHostPath := filepath.Join(hostPath, fileName)
destFile, err := os.Create(fileHostPath)
```

While `filepath.Base` is used for the filename, if `hostPath` is controlled by an attacker (via CRD -> Job env var), they could write to arbitrary host locations.

**Attack Vector:** A user creates a Capture CRD with `hostPath: "/etc/cron.d"` - the capture pod writes files to this location with 0644 permissions, potentially allowing privilege escalation on the node.

**Recommendation:**
- Validate hostPath against an allowlist of permitted directories
- Ensure hostPath is under a controlled parent directory
- Consider using a dedicated, restricted base path

---

### 41. S3 Upload Accepts Arbitrary Endpoints

**Location:** [crd/api/v1alpha1/capture_types.go](crd/api/v1alpha1/capture_types.go#L144-L157)

**Severity:** Medium

**Description:** The S3Upload configuration accepts arbitrary endpoints:

```go
type S3Upload struct {
    // Endpoint of S3 compatible storage service.
    // +optional
    Endpoint string `json:"endpoint,omitempty"`
    ...
}
```

An attacker could specify an endpoint pointing to their own S3-compatible server, causing capture data to be exfiltrated.

**Recommendation:**
- Validate endpoints against an allowlist
- Require TLS for S3 endpoints
- Add audit logging for all capture uploads

---

### 42. MetricsConfiguration SourceLabels Can Enumerate All IPs

**Location:** [crd/api/v1alpha1/metricsconfiguration_types.go](crd/api/v1alpha1/metricsconfiguration_types.go#L31-L37)

**Severity:** Low

**Description:** MetricsConfiguration allows specifying `SourceLabels` and `DestinationLabels` including IP addresses:

```go
// SourceLabels represents the source context of the metrics collected
// Such as IP, pod, port
SourceLabels []string `json:"sourceLabels,omitempty"`
```

A user with MetricsConfiguration create permissions could configure metrics to expose all IP addresses in the cluster.

**Recommendation:**
- Consider RBAC restrictions on MetricsConfiguration
- Document the privacy implications
- Add namespace-scoping options

---

### 43. Shell Command Allows Arbitrary Capabilities via Flags

**Location:** [shell/manifests.go](shell/manifests.go) (Referenced in Finding #1)

**Severity:** High (Expansion of Finding #1)

**Description:** The shell command accepts user-provided flags for:
- `--mount-hostfs` - Mount entire host filesystem
- `--allow-hostfs-write` - Enable write access to host
- `--host-pid` - Share host PID namespace
- `--capabilities` - Add specific capabilities

A user who has been granted permission to run `kubectl retina shell` could escalate these options beyond what was intended.

**Attack Scenario:**
1. Admin grants user permission to use `kubectl retina shell` for debugging
2. User runs `kubectl retina shell --node mynode --mount-hostfs --allow-hostfs-write --host-pid`
3. User gains root access to the node

**Recommendation:**
- Implement a configuration file or RBAC to restrict allowed shell options
- Log all shell command invocations with their parameters
- Consider a "safe mode" that restricts dangerous options

---

## Recommendations

### Immediate Actions (Critical/High)

1. **Implement authentication for pprof endpoints** or disable them in production
2. **Add input validation for capture filters** to prevent command injection
3. **Bundle the Windows collectlogs.ps1 script** in the container image with integrity verification
4. **Add audit logging** for privileged operations (shell, capture creation)
5. **Set proper HTTP server timeouts** to prevent DoS
6. **Implement ValidatingWebhook for Capture CRDs** to validate filters and output destinations
7. **Validate hostPath output locations** against an allowlist

### Short-term Actions (Medium)

1. Implement rate limiting on HTTP endpoints
2. Add image signature verification for shell and capture workload images
3. Restrict file permissions for sensitive data (captures, BPF maps)
4. Sanitize telemetry data before transmission
5. Implement allowlist validation for blob upload destinations
6. **Sanitize blob names during download** to prevent path traversal
7. **Add authentication to metrics endpoint** or document exposure risks
8. **Validate S3 endpoints** against known providers

### Long-term Actions (Low/Architectural)

1. Consider implementing a security-focused admission controller for Capture CRDs
2. Add optional mTLS for internal component communication
3. Implement comprehensive audit logging throughout the codebase
4. Create a security-hardened deployment mode with minimal privileges
5. Regular dependency scanning and updates
6. **Implement RBAC-based shell option restrictions**
7. **Add network policy templates** to restrict metrics access

### Supply Chain Security Actions

1. **Sign all container images** including merge queue builds, not just releases
2. **Move Application Insights key to runtime environment** instead of build-time embedding
3. **Scope operator secret access** to specific namespaces/resources
4. **Enable TLS certificate verification** for all build-time downloads
5. **Review ignored Dependabot dependencies** periodically for security updates
6. **Add SBOM generation** for all container images
7. **Implement image provenance verification** in deployment pipelines
8. **Add NetworkPolicy templates** to Helm chart for production deployments

---

## Supply Chain & Infrastructure Security Findings

This section covers findings related to container images, CI/CD pipelines, dependencies, and Kubernetes deployment configurations.

### 44. Application Insights Key Embedded in Container Images

**Location:** 
- [controller/Dockerfile](controller/Dockerfile#L56-L57)
- [.github/workflows/images.yaml](.github/workflows/images.yaml#L57)

**Severity:** Medium

**Description:** The Application Insights instrumentation key is baked into container images at build time:

```dockerfile
-ldflags "-X github.com/microsoft/retina/internal/buildinfo.ApplicationInsightsID="$APP_INSIGHTS_ID"
```

In the CI workflow:
```yaml
APP_INSIGHTS_ID=${{ secrets.AZURE_APP_INSIGHTS_KEY }} \
```

**Risk:**
- Anyone with access to the container image can extract the Application Insights key
- The key could be used to inject fake telemetry or enumerate deployed instances
- Telemetry data might leak sensitive information to shared Application Insights instances

**Recommendation:**
- Pass Application Insights key as an environment variable at runtime instead of embedding
- Use Azure Managed Identity for Application Insights authentication where possible
- Rotate the key periodically

---

### 45. Init Container Runs as Privileged

**Location:** [deploy/standard/manifests/controller/helm/retina/templates/daemonset.yaml](deploy/standard/manifests/controller/helm/retina/templates/daemonset.yaml#L40-L41)

**Severity:** Medium

**Description:** The init container runs with `privileged: true`:

```yaml
securityContext:
  privileged: true
```

While this is required for BPF filesystem setup, it creates a window during pod initialization where the container has full host access.

**Risk:**
- If the init container image is compromised, full node access is possible
- Container escape vulnerabilities have a larger impact window

**Recommendation:**
- Minimize the privileged init container to the absolute minimum operations
- Consider using specific capabilities instead of full privileged mode if possible
- Implement image signing and verification for init images

---

### 46. DaemonSet Requires SYS_ADMIN Capability

**Location:** [deploy/standard/manifests/controller/helm/retina/values.yaml](deploy/standard/manifests/controller/helm/retina/values.yaml#L121-L128)

**Severity:** Low (Documented/Expected)

**Description:** The agent requires elevated capabilities:

```yaml
securityContext:
  privileged: false
  capabilities:
    add:
      - SYS_ADMIN
      - NET_ADMIN
      - IPC_LOCK
```

**Risk:** These capabilities are necessary for eBPF but grant significant privileges:
- `SYS_ADMIN`: Almost equivalent to root, allows BPF loading
- `NET_ADMIN`: Can modify network configuration
- `IPC_LOCK`: Can lock memory (required for perf buffers)

**Recommendation:**
- Document the security implications clearly in deployment documentation
- Provide a "minimal" profile for environments where full observability isn't needed
- Consider using BPF token-based authorization in future kernel versions

---

### 47. Operator Has Broad Secret Access

**Location:** [deploy/standard/manifests/controller/helm/retina/templates/operator.yaml](deploy/standard/manifests/controller/helm/retina/templates/operator.yaml#L204-L215)

**Severity:** Medium

**Description:** The operator has full CRUD access to secrets across the cluster:

```yaml
- apiGroups:
    - ""
  resources:
  - secrets
  verbs:
  - create
  - delete
  - get
  - list
  - patch
  - update
  - watch
```

**Risk:**
- A compromised operator pod can read all secrets in the cluster
- This includes service account tokens, TLS certificates, and application secrets
- Violates the principle of least privilege

**Recommendation:**
- Scope secret access to specific namespaces where captures are allowed
- Use `resourceNames` to limit access to specific secret names
- Consider a separate service account with minimal permissions for capture jobs

---

### 48. No Network Policies Deployed by Default

**Location:** [deploy/standard/manifests/controller/helm/retina/templates/](deploy/standard/manifests/controller/helm/retina/templates/)

**Severity:** Low

**Description:** The Helm chart does not include NetworkPolicy resources to restrict traffic to/from Retina components. By default:
- The metrics port (18080) is accessible from any pod
- The pprof port (8082) is accessible from any pod
- The operator webhook port is accessible from any pod

**Risk:**
- Any pod in the cluster can scrape metrics and gather network topology
- Malicious pods could exploit pprof endpoints for resource exhaustion
- Lateral movement from compromised pods to Retina is unrestricted

**Recommendation:**
- Add optional NetworkPolicy templates to the Helm chart
- Restrict metrics access to Prometheus ServiceAccount
- Restrict pprof access to specific debugging pods/namespaces

---

### 49. External Binary Downloaded at Build Time Without Integrity Verification

**Location:** [controller/Dockerfile](controller/Dockerfile#L47-L51)

**Severity:** Medium

**Description:** The eBPF Windows binary is downloaded from NuGet at build time without signature verification:

```dockerfile
curl -L -o eBPFRetina.zip https://www.nuget.org/api/v2/package/Microsoft.Wcn.Observability.eBPF.Retina.x64/0.1.0-prerelease.11
```

Note: The Hubble download without certificate verification is covered in Finding #19.

**Risk:**
- NuGet packages could be replaced with malicious versions
- No signature verification of the downloaded package
- Version pinning exists but no integrity verification

**Recommendation:**
- Verify package signatures from NuGet
- Consider vendoring the binary or using Azure Artifacts with integrity checks
- Add SHA256 verification for the downloaded package

---

### 50. Dependabot Ignores inspektor-gadget Updates

**Location:** [.github/dependabot.yaml](.github/dependabot.yaml#L30-L31)

**Severity:** Low

**Description:** Dependabot is configured to ignore updates for `inspektor-gadget`:

```yaml
ignore:
  - dependency-name: "github.com/inspektor-gadget/inspektor-gadget"
```

**Risk:**
- Security vulnerabilities in inspektor-gadget won't be automatically flagged
- The project might fall behind on critical security patches for this dependency

**Recommendation:**
- Document why this dependency is excluded
- Implement manual review process for inspektor-gadget updates
- Consider periodic security review of ignored dependencies

---

### 51. Container Images Not Signed in All Workflows

**Location:** [.github/workflows/images.yaml](.github/workflows/images.yaml)

**Severity:** Medium

**Description:** While the release workflow includes Cosign signing:

```yaml
- name: Install Cosign
  uses: sigstore/cosign-installer@v3.8.2
...
- name: Sign container image
  run: cosign sign --yes ${IMAGE_PATH}@${DIGEST}
```

The regular build workflow (`images.yaml`) does not sign images pushed during merge queue builds.

**Risk:**
- Images pushed from merge queue are not cryptographically signed
- Supply chain attacks could target these unsigned intermediate images
- No way to verify image provenance for non-release builds

**Recommendation:**
- Sign all pushed images, not just releases
- Implement a deployment policy requiring signed images
- Add SBOM generation for all container images

---

### 52. GitHub Actions Permissions Could Be More Restrictive

**Location:** Multiple workflow files

**Severity:** Low

**Description:** Workflows request `id-token: write` permission which is necessary for OIDC authentication with Azure, but some workflows also request broader permissions than needed.

**Risk:**
- Compromised workflow could have more access than required
- id-token permission allows federation with external identity providers

**Recommendation:**
- Review each workflow and apply minimal permissions
- Use `permissions` at job level rather than workflow level where possible

---

## Additional Findings from Deep Code Review

### 53. HTTP Client Missing Timeouts in Windows Capture (HIGH)

**Location:** [pkg/capture/provider/network_capture_win.go](pkg/capture/provider/network_capture_win.go#L236)

**Description:** The `http.Get()` call in the Windows capture provider (which downloads scripts as described in Finding #3) uses the default HTTP client without any timeout:

```go
resp, err := http.Get(url)
```

**Risk:**
- Denial of Service: A slow or unresponsive server could hang the capture operation indefinitely
- Resource exhaustion: Connections may never be released

**Recommendation:**
```go
client := &http.Client{Timeout: 30 * time.Second}
resp, err := client.Get(url)
```

---

### 54. Console Logging May Leak Sensitive Information (MEDIUM)

**Location:** Multiple files

**Description:** Various files use `fmt.Printf`, `fmt.Println`, or `log.Printf` for output, which may expose sensitive runtime information:

- [captureworkload/main.go](captureworkload/main.go) - Logs capture configuration
- [cmd/legacy/daemon_linux.go](cmd/legacy/daemon_linux.go) - Logs configuration
- [cli/cmd/*.go](cli/cmd/) - Various CLI output
- [hack/tools/kapinger/*.go](hack/tools/kapinger/) - Test tool logging

**Risk:**
- Sensitive information could be logged to stdout/stderr and captured in container logs
- API keys, tokens, or internal paths could be exposed

**Recommendation:**
- Use structured logging (zap) consistently across all components
- Implement log sanitization for sensitive fields
- Avoid logging full configuration objects

---

### 55. Gosec Bypass Annotations Masking Security Issues (MEDIUM)

**Location:** Multiple files

**Description:** Several files contain `//nolint:gosec` annotations that bypass security scanning:

- [operator/cmd/standard/deployment.go](operator/cmd/standard/deployment.go#L256): `//nolint:gosec // TODO replace with secure server` - pprof endpoint without TLS
- [cli/cmd/capture/download.go](cli/cmd/capture/download.go#L73): `//nolint:gosec,gomnd // intentionally permissive bitmask` - File permissions
- [cli/cmd/config.go](cli/cmd/config.go#L29): `//nolint:gosec,gomnd // no sensitive data` - Config file permissions

**Risk:**
- Security issues are knowingly bypassed but may never be addressed
- TODO comments suggest these are known issues awaiting fixes
- Creates technical security debt
f
**Recommendation:**
- Create tracking issues for each gosec bypass
- Implement secure alternatives as noted in the TODO comments
- Add periodic review of nolint annotations

---

### 56. Test Code Fetches Sensitive Output from Terraform (LOW)

**Location:** [test/multicloud/test/integration/*.go](test/multicloud/test/integration/)

**Description:** Integration tests fetch sensitive credentials from Terraform outputs:

```go
token := utils.FetchSensitiveOutput(t, opts, "access_token")
restConfig := utils.CreateRESTConfigWithBearer(caCertString, token, host)
```

**Risk:**
- Test logs may capture access tokens if tests fail
- Sensitive tokens handled in test code without proper cleanup

**Recommendation:**
- Ensure test cleanup properly destroys credentials
- Use short-lived tokens where possible
- Mask sensitive output in test logs

---

## Appendix: Files Reviewed

- cmd/root.go, cmd/standard/daemon.go, cmd/hubble/daemon_linux.go
- controller/main.go, controller/Dockerfile*
- operator/cmd/root.go, operator/cmd/standard/deployment.go
- cli/cmd/shell.go, cli/cmd/capture/*.go, cli/cmd/config.go
- shell/shell.go, shell/attach.go, shell/manifests.go, shell/validation.go
- pkg/capture/*.go, pkg/capture/provider/*.go, pkg/capture/outputlocation/*.go
- pkg/plugin/dropreason/*.go, pkg/plugin/packetparser/*.go, pkg/plugin/dns/*.go
- pkg/plugin/ebpfwindows/*.go, pkg/plugin/hnsstats/*.go, pkg/plugin/pktmon/*.go
- pkg/plugin/filter/*.go, pkg/plugin/conntrack/*.go, pkg/plugin/packetforward/*.go
- pkg/server/server.go
- pkg/bpf/setup_linux.go
- pkg/telemetry/*.go
- pkg/config/config.go
- pkg/loader/compile.go, pkg/loader/generate.go
- pkg/utils/*.go
- pkg/enricher/enricher.go
- pkg/controllers/operator/capture/controller.go
- pkg/client/client.go
- init/retina/main_linux.go
- captureworkload/main.go
- crd/api/v1alpha1/validations/*.go
- deploy/standard/manifests/controller/helm/retina/templates/*.yaml
- deploy/standard/manifests/controller/helm/retina/values.yaml
- deploy/standard/registercrd.go
- scripts/windows.ps1
- windows/setkubeconfigpath.ps1
- hack/tools/kapinger/*.go
- test/multicloud/test/integration/*.go
- test/e2e/framework/**/*.go
- Makefile
- go.mod, go.sum
- Various Dockerfiles (controller, operator, shell, cli)

**eBPF/C Source Files Reviewed:**
- pkg/plugin/dropreason/_cprog/drop_reason.c, drop_reason.h, dynamic.h
- pkg/plugin/packetparser/_cprog/packetparser.c, packetparser.h, dynamic.h
- pkg/plugin/conntrack/_cprog/conntrack.c, conntrack.h, dynamic.h
- pkg/plugin/filter/_cprog/retina_filter.c
- pkg/plugin/packetforward/_cprog/packetforward.c, packetforward.h
- test/e2e/tools/event-writer/bpf_event_writer.c

**CI/CD and Supply Chain Files Reviewed:**
- .github/workflows/images.yaml
- .github/workflows/release-images.yaml
- .github/workflows/trivy.yaml
- .github/workflows/e2e.yaml
- .github/dependabot.yaml

---

*Report generated by automated security analysis. All findings should be validated and prioritized based on deployment context and threat model.*
