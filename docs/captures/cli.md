# Capture with Retina CLI

Retina capture command allows the user to capture network traffic and metadata for the capture target, and then send the capture file to the location by Output Configuration.

Note: captures can also be performed with a [Capture CRD](../CRDs/Capture.md) after [installing Retina](../installation/setup.md) with capture support.

## Retina capture create

`retina capture create` creates a Capture with underlying Kubernetes jobs.

### No wait(default true)

Do not wait for the long-running capture job to finish.
By default, Retina capture CLI will exit before the jobs are completed. With `--no-wait=false`, the CLI will wait until the jobs are completed and clean up the Kubernetes resources created.

#### Example

- do not wait for the long-running capture job to finish

`kubectl retina capture create --host-path /mnt/capture --node-selectors "kubernetes.io/os=linux" --no-wait=true`

### Namespace(default "default")

Namespace to host capture job and the other k8s resources for a network capture. Please make sure the namespace exists.

#### Example

- deploy the capture job in namespace `capture`

`kubectl retina capture create --host-path /mnt/capture --namespace capture --node-selectors "kubernetes.io/os=linux"`

### Capture Target(required)

Capture target indicates the target on which the network packets capture will be performed, and the user can select either node, by node-selectors or node-names, or Pods, by pod-selector and namespace-selector pair.

#### Examples

- capture network packets on the node selected by node selectors

`kubectl retina capture create --host-path /mnt/capture --namespace capture --node-selectors "kubernetes.io/os=linux"`

- capture network packets on the node selected by node names

`kubectl retina capture create --host-path /mnt/capture --namespace capture --node-names "aks-nodepool1-41844487-vmss000000,aks-nodepool1-41844487-vmss000001"`

- capture network packets on the pod selected by pod-selector and namespace-selector pairs

`kubectl retina capture create --host-path /mnt/capture --namespace capture --pod-selectors="k8s-app=kube-dns" --namespace-selectors="kubernetes.io/metadata.name=kube-system"`

### Stop Capture(optional with default values)

The Capture can be stopped in either way below:

- In a given time, by the `duration` flag, or when the allowed maximum capture file reaches by the `max-size` flag. When both are specified, the capture will stop when either condition first meets.
- On demand by deleting the capture before the specified conditions meets.

The network traffic will be uploaded to the specified output location.

#### Capture Duration

Duration of capturing packets(default 1m)

##### Example

- stop the capture in 2 minutes

`kubectl retina capture create --host-path /mnt/capture --namespace capture --node-selectors "kubernetes.io/os=linux" --duration=2m`

#### Maximum Capture Size

Limit the capture file to MB in size(default 100MB)

##### Example

- stop the capture when the capture file size reaches 50MB

`kubectl retina capture create --host-path /mnt/capture --namespace capture --node-selectors "kubernetes.io/os=linux" --max-size=50`

#### Packet Size(optional)

Limits the each packet to bytes in size and packets longer than PacketSize will be truncated.
This is beneficial when the user wants to reduce the capture file size or hide customer data for security concern.

##### Example

- limit each packet size to 96 bytes

`kubectl retina capture create --host-path /mnt/capture --namespace capture --node-selectors "kubernetes.io/os=linux" --packet-size=96`

### Capture Configuration(optional)

capture configuration indicates the configurations of the network capture.

#### Packet capture filters

Packet capture filters represent a range of filters to be included/excluded in the capture. This is not available on Windows.

##### Example

`kubectl retina capture create --host-path /mnt/capture --namespace capture --node-selectors "kubernetes.io/os=linux" --exclude-filter="10.224.0.26:80,10.224.0.33:8080" --include-filter="10.224.0.42:80,10.224.0.33:8080"`

#### Tcpdump Filter

Raw tcpdump flags which works only for Linux. Available tcpdump filters can be found in [TCPDUMP MAN PAGE](https://www.tcpdump.org/manpages/tcpdump.1.html)
NOTE: this includes only tcpdump flags, for expression part, please user [Packet include/exclude filters
](#packet-capture-filters).

##### Example

- filter DNS query

`kubectl retina capture create --host-path /mnt/capture --namespace capture --node-selectors "kubernetes.io/os=linux" --tcpdump-filter="udp port 53"`

#### Include Metadata

If true, collect static network metadata into the capture file(default true)

##### Example

- disable collecting network metadata

`kubectl retina capture create --host-path /mnt/capture --namespace capture --node-selectors "kubernetes.io/os=linux" --include-metadata=false`

### Job Number Limit(optional)

The maximum number of jobs can be created for each capture. The default value 0 indicates no limit.
This can be configured by CLI flags for each CLI command, or config map consumed by retina-operator.
When creating a job requires job number exceeds this limit, it will fail with prompt like
`Error: the number of capture jobs 3 exceeds the limit 2`

#### Example

`kubectl retina capture create --job-num-limit=10 --host-path /mnt/capture --namespace capture --node-selectors "kubernetes.io/os=linux"`

### Output Configuration(required)

OutputConfiguration indicates the location capture will be stored, and at least one location should be specified.

Blob-upload requires a Blob Shared Access Signature with the write permission to the storage account container, to create SAS tokens in the Azure portal, please read [Create SAS Tokens in the Azure Portal](https://learn.microsoft.com/en-us/azure/cognitive-services/translator/document-translation/how-to-guides/create-sas-tokens?tabs=Containers#create-sas-tokens-in-the-azure-portal)

#### Examples

- store the capture file in the node host path `/mnt/capture`

`kubectl retina capture create --host-path /mnt/capture --namespace capture --node-selectors "kubernetes.io/os=linux"`

- store the capture file to a PVC `mypvc` with access mode ReadWriteMany in namespace `capture`

`kubectl retina capture create --pvc mypvc --namespace capture --node-selectors "kubernetes.io/os=linux"`

- store the capture file to a storage account

`kubectl retina capture create --blob-upload <Blob SAS URL with write permission> --node-selectors "kubernetes.io/os=linux"`

- store the capture file to AWS S3

`kubectl retina capture create --s3-bucket "your-bucket-name" --s3-region "eu-central-1" --s3-access-key-id "your-access-key-id" --s3-secret-access-key "your-secret-access-key" --node-selectors "kubernetes.io/os=linux"`

- store the capture file to S3-compatible service (like MinIO)

`kubectl retina capture create --s3-bucket "your-bucket-name" --s3-endpoint "https://play.min.io:9000" --s3-access-key-id "your-access-key-id" --s3-secret-access-key "your-secret-access-key" --node-selectors "kubernetes.io/os=linux"`

### Debug mode

With debug mode, when `--debug` is specified, we can overwrite the capture job Pod image from the default official `GHCR` one.

#### Examples

- use `ghcr.io` image in default debug mode

`kubectl retina capture create --host-path /mnt/test --namespace capture --node-selectors "kubernetes.io/os=linux" --debug`

- use customized retina-agent image

`RETINA_AGENT_IMAGE=<YOUR RETINA AGENT IMAGE> kubectl retina capture create --host-path /mnt/test --namespace capture --node-selectors "kubernetes.io/os=linux" --debug`

## Retina capture delete

`retina capture delete` deletes a Kubernetes Jobs with the specified Capture name.

### Examples

`kubectl retina capture delete --name retina-capture-zlx5v`

## Retina capture list

`retina capture list` lists Captures in a namespace or all namespaces.

### Examples

- list Captures under namespace `capture`

`kubectl retina capture list --namespace capture`

- list Captures under in all namespaces

`kubectl retina capture list --all-namespaces`

## Obtain the output

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
  - reference: [Microsoft SDN Debug tool
](https://github.com/microsoft/SDN/blob/master/Kubernetes/windows/debug/collectlogs.ps1)

## Cleanup

When creating a capture, we can specify `--no-wait` to clean up the jobs after the Capture is completed. Otherwise, after creating a Capture, a random Capture name is returned, with which we can delete the jobs by `kubectl retina capture delete` command.
