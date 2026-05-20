# Capture

## Overview

The `Capture` CustomResourceDefinition (CRD) defines a custom resource called `Capture`, which represents the settings of a network trace.
This CRD allows users to specify the configurations for capturing network packets and storing the captured data.

To use the `Capture` CRD, [install Retina](../../02-Installation/01-Setup.md) with capture support.

## CRD Specification

The full specification for the `Capture` CRD can be found in the [Capture CRD](https://github.com/microsoft/retina/blob/main/deploy/standard/manifests/controller/helm/retina/crds/retina.sh_captures.yaml) file.

The `Capture` CRD is defined with the following specifications:

- **API Group:** retina.sh
- **API Version:** v1alpha1
- **Kind:** Capture
- **Plural:** captures
- **Singular:** capture
- **Scope:** Namespaced

### Fields

- **spec.captureConfiguration:** Specifies the configuration for capturing network packets. It includes the following properties:
  - `captureOption`: Lists options for the capture, such as:
    - `duration`: Capture duration
    - `maxCaptureSize`: Maximum capture file size in MB
    - `packetSize`: Maximum packet size to capture
    - `interfaces`: Array of network interface names to capture from (e.g., `["eth0", "eth1"]`). If empty, captures from all interfaces.
    - `pcapFilter`: BPF filter expression for packet filtering (e.g., `"host 10.0.0.1"`, `"tcp port 443"`). Does NOT accept flags.
    - Boolean flags for tcpdump capture behavior and display options:
      - `noPromiscuous`: Disable promiscuous mode (tcpdump -p)
      - `packetBuffered`: Enable packet-buffered output (tcpdump -U)
      - `immediateMode`: Enable immediate mode (tcpdump --immediate-mode)
      - `noResolveDNS`: Don't resolve hostnames (tcpdump -n)
      - `noResolvePort`: Don't resolve hostnames or port names (tcpdump -nn)
      - `verbose`, `extraVerbose`, `maxVerbose`: Verbose output levels (tcpdump -v, -vv, -vvv). **Mutually exclusive** - set only one.
      - `printDataHex`, `printDataHexLink`, `printDataASCII`, `printDataASCIILink`: Print packet data in hex (tcpdump -x, -xx) or ASCII (tcpdump -A, -AA). **Mutually exclusive** - set only one print data format.
      - `printLinkHeader`: Print link-level headers (tcpdump -e)
      - `quietOutput`: Quick/quiet output (tcpdump -q)
      - `absoluteSeq`: Print absolute TCP sequence numbers (tcpdump -S)
      - `noTimestamp`, `unformattedTimestamp`, `deltaTimestamp`, `dateTimestamp`, `deltaSinceFirst`: Timestamp options (tcpdump -t, -tt, -ttt, -tttt, -ttttt). **Mutually exclusive** - set only one.
      - `dontVerifyChecksum`: Don't verify TCP checksums (tcpdump -K)
  - `captureTarget`: Defines the target on which the network packets will be captured. It includes namespace, node, and pod selectors, as well as specific pod names.
  - `filters`: Specifies filters for including or excluding network packets based on IP or port.
  - `includeMetadata`: Indicates whether networking metadata should be captured.
  - `tcpdumpFilter`: DEPRECATED. Accepts BPF filter expressions only (no flags). Use `captureOption.pcapFilter` for BPF expressions and `captureOption` boolean flags for display/output options.

- **spec.outputConfiguration:** Indicates where the captured data will be stored. It includes the following properties:
  - `blobUpload`: Specifies a secret containing the blob SAS URL for storing the capture data.
  - `hostPath`: Stores the capture files into the specified host filesystem.
  - `persistentVolumeClaim`: Mounts a PersistentVolumeClaim into the Pod to store capture files.
  - `s3Upload`: Specifies the configuration for uploading capture files to an S3-compatible storage service, including the bucket name, region, and optional custom endpoint.

- **status:** Describes the status of the capture, including the number of active, failed, and completed jobs, completion time, conditions, and more. Check [capture lifecycle](#capture-lifecycle) for more details.

## Usage

### Creating a Capture

To create a `Capture`, create a YAML manifest file with the desired specifications and apply it to the cluster using `kubectl apply`:

```yaml
apiVersion: retina.sh/v1alpha1
kind: Capture
metadata:
  name: example-capture
spec:
  captureConfiguration:
    captureOption:
      duration: "30s"
      maxCaptureSize: 100
      packetSize: 1500
    captureTarget:
      namespaceSelector:
        matchLabels:
          app: target-app
  outputConfiguration:
    hostPath: /captures
    blobUpload: blob-sas-url
    s3Upload:
      bucket: retina-bucket
      region: ap-northeast-2
      path: retina/captures
      secretName: capture-s3-upload-secret
---
apiVersion: v1
kind: Secret
metadata:
  name: capture-s3-upload-secret
data:
  s3-access-key-id: <based-encode-s3-access-key-id>
  s3-secret-access-key: <based-encode-s3-secret-access-key>
```

### Advanced Filtering

#### Capturing on Specific Network Interfaces

To capture packets only on specific network interfaces, use the `captureOption.interfaces` field:

```yaml
apiVersion: retina.sh/v1alpha1
kind: Capture
metadata:
  name: capture-specific-interfaces
spec:
  captureConfiguration:
    captureOption:
      duration: "1m"
      interfaces: ["eth0", "eth1"]  # Capture only on these interfaces
    captureTarget:
      nodeSelector:
        matchLabels:
          kubernetes.io/hostname: node-1
  outputConfiguration:
    hostPath: /tmp/captures
```

#### Using BPF Filters with Display Options

The `captureOption.pcapFilter` field accepts BPF (Berkeley Packet Filter) expressions for packet filtering. Use boolean flags in `captureOption` for display and output formatting.

##### BPF Filters Only

To apply packet filtering using BPF syntax:

```yaml
apiVersion: retina.sh/v1alpha1
kind: Capture
metadata:
  name: capture-with-bpf-filter
spec:
  captureConfiguration:
    captureOption:
      duration: "1m"
      pcapFilter: "tcp and (port 443 or port 80)"  # Only capture HTTP/HTTPS traffic
    captureTarget:
      nodeSelector:
        matchLabels:
          kubernetes.io/hostname: node-1
  outputConfiguration:
    hostPath: /tmp/captures
```

**Valid BPF filter examples:**

- `"host 10.0.0.1"` - Capture packets to/from specific host
- `"tcp port 443"` - Capture HTTPS traffic
- `"net 192.168.0.0/16 and not port 22"` - Capture subnet traffic except SSH
- `"tcp and (port 80 or port 443)"` - Capture HTTP and HTTPS traffic

##### Using Display Options with BPF Filters

Combine BPF filters with display option boolean flags:

```yaml
apiVersion: retina.sh/v1alpha1
kind: Capture
metadata:
  name: capture-with-display-options
spec:
  captureConfiguration:
    captureOption:
      duration: "1m"
      pcapFilter: "tcp port 443"
      noResolveDNS: true     # Don't resolve hostnames (tcpdump -n)
      verbose: true          # Verbose output (tcpdump -v)
      printDataHex: true     # Show hex data (tcpdump -x)
    captureTarget:
      nodeSelector:
        matchLabels:
          kubernetes.io/hostname: node-1
  outputConfiguration:
    hostPath: /tmp/captures
```

**Examples with different display options:**

```yaml
# Don't resolve names or ports, very verbose, capture HTTP
captureOption:
  pcapFilter: "tcp port 80"
  noResolvePort: true    # tcpdump -nn
  extraVerbose: true     # tcpdump -vv
```

```yaml
# Show hex data with timestamps, capture ICMP
captureOption:
  pcapFilter: "icmp"
  printDataHex: true     # tcpdump -x
  dateTimestamp: true    # tcpdump -tttt
```

**Available display option and boolean flags:**

- `noResolveDNS`, `noResolvePort`: Don't resolve hostnames/port names (tcpdump -n, -nn)
- **Verbosity** (mutually exclusive - choose one):
  - `verbose`: Verbose output (tcpdump -v)
  - `extraVerbose`: Extra verbose output (tcpdump -vv)
  - `maxVerbose`: Maximum verbose output (tcpdump -vvv)
- **Print data format** (mutually exclusive - choose one):
  - `printDataHex`: Show packet data in hex (tcpdump -x)
  - `printDataHexLink`: Show packet data in hex with link-level headers (tcpdump -xx)
  - `printDataASCII`: Show packet data in ASCII (tcpdump -A)
  - `printDataASCIILink`: Show packet data in ASCII with link-level headers (tcpdump -AA)
- **Timestamp format** (mutually exclusive - choose one):
  - `noTimestamp`: Don't print timestamps (tcpdump -t)
  - `unformattedTimestamp`: Print timestamps as Unix epoch (tcpdump -tt)
  - `deltaTimestamp`: Print time delta between packets (tcpdump -ttt)
  - `dateTimestamp`: Print timestamps with date (tcpdump -tttt)
  - `deltaSinceFirst`: Print time delta since first packet (tcpdump -ttttt)
- Other options:
  - `printLinkHeader`: Print link-level headers (tcpdump -e)
  - `quietOutput`: Quick/quiet output (tcpdump -q)
  - `absoluteSeq`: Print absolute TCP sequence numbers (tcpdump -S)
  - `dontVerifyChecksum`: Don't verify TCP checksums (tcpdump -K)

> **CLI Users**: When using the `kubectl retina capture create` command, use the enum-based flags (`--verbosity`, `--timestamp-format`, `--print-data`) instead of setting these boolean fields directly. See the [CLI documentation](../../04-Captures/02-cli.md) for details.

**Capture behavior options:**

- `noPromiscuous`: Disable promiscuous mode (tcpdump -p)
- `packetBuffered`: Enable packet-buffered output (tcpdump -U)
- `immediateMode`: Enable immediate mode (tcpdump --immediate-mode)

### Capture Lifecycle

Once a Capture is created, the capture controller inside retina-operator is responsible for managing the lifecycle of the Capture.
A Capture can be turned into error when errors happens like no required selector is specified, or InProgress when created workload are running, or completed when all workloads are completed.
In implementation, the complete status is defined by setting complete status condition to true, and InProgress is defined as a false complete status condition.

#### Examples of Capture status

- No allowed selectors are specified

```yaml
Status:
  Conditions:
    Last Transition Time:  2023-10-23T06:17:39Z
    Message:               Neither NodeSelector nor NamespaceSelector&PodSelector is set.
    Reason:                otherError
    Status:                True
    Type:                  error
```

- Capture is running in progress

```yaml
Status:
  Active:  2
  Conditions:
    Last Transition Time:  2023-10-23T06:33:56Z
    Message:               2/2 Capture jobs are in progress, waiting for completion
    Reason:                JobsInProgress
    Status:                False
    Type:                  complete
```

- Capture is completed

```yaml
Status:
  Completion Time:  2023-10-23T06:34:40Z
  Conditions:
    Last Transition Time:  2023-10-23T06:34:40Z
    Message:               All 2 Capture jobs are completed
    Reason:                JobsCompleted
    Status:                True
    Type:                  complete
  Succeeded:               2
```
