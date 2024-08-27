# Capture TSG

Retina Capture create Kubernetes Jobs and translate the flags into the specification of the Pod. Locating the Pods by the Capture is essential to troubleshooting Retina Capture issues.

## Find the Pod created for each Capture

Run the following command to list captures:

```shell
kubectl retina capture list -n <capture namespace>
```

or get jobs through the capture name:

```shell
kubectl get job --selector  capture-name=<capture name> -n <capture namespace>
```

and then get the Pod through the job name by:

```shell
kubectl get pod -n <capture namespace> --seletor job-name=<capture job name>
```

## Capture Pod ImagePullBackOff

By default, kubectl retina plugin will eventually create Capture Pods from the [GHCR](https://github.com/microsoft/retina) image with the same version as the kubectl plugin. If the kubectl plugin is built from a local environment, the GHCR image cannot be found. Check [capture CLI Debug Mode](](../04-Captures/02-CLI.md#Debug_mode) for local development and testing.

## Windows node allows only one capture job running at one time

Retina Capture utilizes [`netsh`](https://learn.microsoft.com/en-us/windows-server/networking/technologies/netsh/netsh-contexts) to capture network traffic on Windows, and as only one netsh trace session is allowed on one Windows node, when a Windows node contains a pending or running Capture Pod, creating a Capture targeting the node will raise an error.
To continue creating a capture on this windows node, you may delete the Capture per

- List Capture Pods running on the Windows node

```shell
kubectl get pod -A -o wide |grep <Windows node name>
```

- Get Capture name from the Pod

```shell
kubectl describe pod <capture pod name> -n <capture pod namespace>  |grep capture-name| awk -F "=" '{print $2}' 
```

- Delete the Capture

```shelll
kubectl retina capture delete --namespace <capture pod namespace> --name <capture name>
```

## Network packets are not correctly captured

In case you want to confirm how the network capture is performed, please check tcpdump.log/netsh.log in Capture bundle, or get the pod container log if the Capture is not deleted.
The pcap file name includes UTC time, which records the network capture start time, which might also be helpful.
