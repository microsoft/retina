# Capture with Retina CLI

The Retina capture command allows users to capture network traffic and metadata for the capture target, and send the data to the location defined by output configuration.

> Note: captures can also be performed with a [Capture CRD](../05-Concepts/CRDs/Capture.md) after [installing Retina](../02-Installation/01-Setup.md) **with capture support**.

## Starting a Capture

`kubectl retina capture create [--flags]` creates a Capture with underlying Kubernetes jobs.

## Flags

### [required] Name

- Syntax: `--name <string>`

A name for the Retina Capture.

**Example:**

`kubectl retina capture create --name capture-test --host-path /mnt/capture --node-selectors "kubernetes.io/os=linux"`

---

### [required] Capture Target

The capture target indicates where the packet capture will be performed. There are three choices here, each of which is its own flag. They are described below.

#### Node Selectors

- Syntax: `--node-selectors "<string>"`

Capture network captures on nodes filtered by the provided by node selectors. Comma-separated.

**Example:**

`kubectl retina capture create --name capture-test --host-path /mnt/capture --node-selectors "kubernetes.io/os=linux"`

#### Node Names

- Syntax: `--node-names "<string>"`

Capture network captures on nodes filtered by the provided node names. Comma-separated.

**Example:**

`kubectl retina capture create --name capture-test --host-path /mnt/capture --node-names "aks-nodepool1-41844487-vmss000000,aks-nodepool1-41844487-vmss000001"`

#### Pod Selectors & Namespace Selectors (Pairs)

- Syntax (Pod Selectors): `--pod-selectors="<string>"`
- Syntax (Namespace Selectors): `--namespace-selectors="<string>"`

Capture network captures on pods filtered by the provided pod-selector and namespace-selector **pairs**. Comma-separated.

**Example:**

`kubectl retina capture create --name capture-test --host-path /mnt/capture --pod-selectors="k8s-app=kube-dns" --namespace-selectors="kubernetes.io/metadata.name=kube-system"`

---

### [required] Output Configuration

The output configuration indicates the location where the capture will be stored. At least one location needs to be specified. This can either be the host path on the node, or a remote storage option.

Blob-upload requires a Blob Shared Access Signature (SAS) with the write permission to the storage account container, to create SAS tokens in the Azure portal, please read: [Create SAS Tokens in the Azure Portal](https://learn.microsoft.com/en-us/azure/cognitive-services/translator/document-translation/how-to-guides/create-sas-tokens?tabs=Containers#create-sas-tokens-in-the-azure-portal).

#### Host Path

Store the capture file in the node's specified host path. In the below example this is `/mnt/capture`.

**Example:**

`kubectl retina capture create --name capture-test --host-path /mnt/capture --node-selectors "kubernetes.io/os=linux"`

#### PVC

Store the capture file to a PVC, in the example below `mypvc` with access mode ReadWriteMany in namespace `capture`.

**Example:**

`kubectl retina capture create --name capture-test --pvc mypvc --node-selectors "kubernetes.io/os=linux"`

#### Storage Account

Store the capture file to a storage account (blob) through the use of a SAS.

**Example:**

`kubectl retina capture create --name capture-test --blob-upload <Blob SAS URL with write permission> --node-selectors "kubernetes.io/os=linux"`

#### AWS S3

Store the capture file to AWS S3.

**Example:**

`kubectl retina capture create --name capture-test --s3-bucket "your-bucket-name" --s3-region "eu-central-1" --s3-access-key-id "your-access-key-id" --s3-secret-access-key "your-secret-access-key" --node-selectors "kubernetes.io/os=linux"`

#### S3-compatible service

Store the capture file to S3-compatible service (such as MinIO).

**Example:**

`kubectl retina capture create --name capture-test --s3-bucket "your-bucket-name" --s3-endpoint "https://play.min.io:9000" --s3-access-key-id "your-access-key-id" --s3-secret-access-key "your-secret-access-key" --node-selectors "kubernetes.io/os=linux"`

---

### No Wait

- Syntax: `--no-wait=<bool>`
- Default value: `true`
- Optional

By default, Retina capture CLI will exit before the jobs are completed. With `--no-wait=false`, the CLI will wait until the jobs are completed and clean up the Kubernetes resources created.

**Example:**

`kubectl retina capture create --name capture-test --host-path /mnt/capture --node-selectors "kubernetes.io/os=linux" --no-wait=true`

---

### Namespace

- Syntax: `--namespace <string>`
- Default value: `default`
- Optional

Sets the namespace which hosts the capture job and the other Kubernetes resources for a network capture. **Ensure the namespace exists**.

**Example:**

`kubectl retina capture create --name capture-test --host-path /mnt/capture --node-selectors "kubernetes.io/os=linux" --namespace capture`

---

### Include / Exclude Filters

- Syntax (Include): `--include-filter="<string>"`
- Syntax (Exclude): `--exclude-filter="<string>"`
- Default value: `""`
- Optional

A comma-separated list of IP:Port pairs that are included or excluded from capturing network packets. Supported formats are IP:Port, IP, Port, *:Port, IP:*

Only works on Linux.

**Example:**

`kubectl retina capture create --name capture-test --host-path /mnt/capture --namespace capture --node-selectors "kubernetes.io/os=linux" --include-filter="10.224.0.42:80,10.224.0.33:8080" --exclude-filter="10.224.0.26:80,10.224.0.34:8080"`

---

### Tcpdump Filters

- Syntax: `--tcpdump-filter="<string>"`
- Default value: `""`
- Optional

Raw tcpdump flags which only work on Linux. Available tcpdump filters can be found in the [TCPDUMP MAN PAGE](https://www.tcpdump.org/manpages/tcpdump.1.html).

NOTE: this includes only tcpdump flags, for boolean expressions, please use [Packet include/exclude filters](#include--exclude-filters).

**Example:**

`kubectl retina capture create --name capture-test --host-path /mnt/capture --namespace capture --node-selectors "kubernetes.io/os=linux" --tcpdump-filter="udp port 53"`

---

### Include Metadata

- Syntax: `--include-metadata=<bool>`
- Default value: `true`
- Optional

Collect static network metadata into the capture file if true.

**Example:**

`kubectl retina capture create --name capture-test --host-path /mnt/capture --namespace capture --node-selectors "kubernetes.io/os=linux" --include-metadata=false`

---

### Job Number Limit

- Syntax: `--job-num-limit=<int>`
- Default value: `0`
- Optional

The maximum number of jobs which can be created for each capture. The default value 0 indicates no limit. This can be configured by CLI flags for each CLI command, or by a config map consumed by the retina-operator.

When creating a job requires job number exceeds this limit, it will fail with prompt like `Error: the number of capture jobs 3 exceeds the limit 2`.

**Example:**

`kubectl retina capture create --name capture-test --job-num-limit=10 --host-path /mnt/capture --namespace capture --node-selectors "kubernetes.io/os=linux"`

---

### Packet Size Limit

- Syntax: `--packet-size=<int>`
- Default value: `0`
- Optional

Limit the packet size in bytes. Packets longer than the defined maximum size will be truncated. The default value 0 indicates no limit. This is beneficial when the user wants to reduce the capture file size or hide customer data due to security concerns.

Only works on Linux.

**Example:**

`kubectl retina capture create --name capture-test --host-path /mnt/capture --namespace capture --node-selectors "kubernetes.io/os=linux" --packet-size=96`

---

## Stopping a Capture

The Capture can be stopped in a number of ways:

- In a given time, by the `duration` flag, or when the file reaches the maximum allowed file size defined by the `max-size` flag. When both are specified, the capture will stop whenever either condition is first met.
- On demand by deleting the capture before the specified conditions meets.

The network traffic will be uploaded to the specified output location.

### Capture Duration

- Syntax: `--duration=<string>`
- Default value: `1m0s`
- Optional

Maximum duration of the packet capture - in minutes / seconds.

**Example:**

`kubectl retina capture create --name capture-test --host-path /mnt/capture --namespace capture --node-selectors "kubernetes.io/os=linux" --duration=2m`

### Capture Size

- Syntax: `--max-size=<int>`
- Default value: `100`
- Optional

Maximum size of the capture file - in MB.

Only works on Linux.

**Example:**

`kubectl retina capture create --name capture-test --host-path /mnt/capture --namespace capture --node-selectors "kubernetes.io/os=linux" --max-size=50`

### Capture Delete

Deleting the capture job before either of the terminating conditions have been met will stop the capture.

`kubectl retina capture delete --name <string>` deletes a Kubernetes Jobs with the specified Capture name.

**Example:**

`kubectl retina capture delete --name retina-capture-zlx5v`

#### Get Capture List

To get a list of the captures you can run `kubectl retina capture list` to get the captures in a specific namespace or in all namespaces.

**Example (namespace):**

`kubectl retina capture list --namespace capture`

**Example (all namespaces):**

`kubectl retina capture list --all-namespaces`

---

## Obtaining the output

After downloading or copying the tarball from the location specified, extract the tarball through the `tar` command in either Linux shell or Windows Powershell, for example,

```shell
tar -xvf retina-capture-aks-nodepool1-41844487-vmss000000-20230320013600UTC.tar.gz
```

### Name pattern of the tarball

the tarball take such name pattern, `$(capturename)-$(hostname)-$(date +%Y%m%d%H%M%S%Z).tar.gz`, for example, `retina-capture-aks-nodepool1-41844487-vmss000000-20230313101436UTC.tar.gz`.

### File and directory structure inside the tarball

- Linux

```text
├── ip-resources.txt
├── iptables-rules.txt
├── retina-capture-aks-nodepool1-41844487-vmss000000-20230320013600UTC.pcap
├── proc-net
│   ├── anycast6
│   ├── arp
... ...
├── proc-sys-net
│   ├── bridge
... ...
|-- socket-stats.txt
`-- tcpdump.log
```

- Windows

```text
│   retina-capture-akswin000002-20230322010252UTC.etl
│   retina-capture-akswin000002-20230322010252UTC.pcap
│   netsh.log
│
└───metadata
    │   arp.txt
        ... ...
    │
    ├───adapters
    │       6to4 Adapter_int.txt
            ... ...
    │
    ├───logs
    │   ├───cbs
    │   │       CBS.log
    │   │
    │   ├───dism
    │   │       dism.log
    │   │
    │   └───NetSetup
    │           service.0.etl
                ... ...
    │
    ├───wfp
    │       filters.xml
    │       netevents.xml
    │       wfpstate.xml
    │
    └───winevt
            Application.evtx
            ... ...
```

### Network metadata

- Linux
  - IP address configuration (ip -d -j addr show)
  - IP neighbor status (ip -d -j neighbor show)
  - IPtables rule dumps
    - iptables-save
    - iptables -vnx -L
    - iptables -vnx -L -t nat
    - iptables -vnx -L -t mangle
  - Network statistics information
    - ss -s (summary)
    - ss -tapionume (socket information)
  - networking stats (/proc/net)
  - kernel networking configuration (/proc/sys/net)

- Windows
  - reference: [Microsoft SDN Debug tool](https://github.com/microsoft/SDN/blob/master/Kubernetes/windows/debug/collectlogs.ps1)

## Debug mode

With debug mode, when `--debug` is specified, you can overwrite the capture job's pod image.

**Example:**

Use `ghcr.io` image in default debug mode.

`kubectl retina capture create --name capture-test --host-path /mnt/test --namespace capture --node-selectors "kubernetes.io/os=linux" --debug`

**Example:**

Use custom retina-agent image by specifying it in the `RETINA_AGENT_IMAGE` environment variable.

`RETINA_AGENT_IMAGE=<YOUR RETINA AGENT IMAGE> kubectl retina capture create --name capture-test --host-path /mnt/test --namespace capture --node-selectors "kubernetes.io/os=linux" --debug`

## Cleanup

When creating a capture, you can specify `--no-wait` to clean up the jobs after the Capture is completed.

Otherwise, after creating a Capture, a random Capture name is returned, with which you can delete the jobs by running the `kubectl retina capture delete` command.
