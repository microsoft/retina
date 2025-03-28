# Multi Cloud Retina

This project leverages [OpenTofu](https://opentofu.org/docs/intro/) Infrastructure as Code (IaC) to create Kubernetes infrastructure on multi-cloud and deploy [microsoft/retina](https://github.com/microsoft/retina) via Helm provider.

![Architecture Diagram](./diagrams/diagram.svg)

## Modules available

* [aks](./modules/aks/): Deploy Azure Kubernetes Service cluster.
* [gke](./modules/gke/): Deploy Google Kubernetes Engine cluster.
* [eks](./modules/eks/): Deploy Elastic Kubernetes Service cluster.
* [kind](./modules/kind/): Deploy KIND cluster.
* [helm-release](./modules/helm-release/): Deploy a Helm Chart, used to deploy Retina and Prometheus.
* [grafana](./modules/grafana/): Set up multiple Prometheus data sources including PDC networks in Grafana Cloud.
* [grafana-pdc-agent](./modules/grafana-pdc-agent/): Deploy PDC agent in each cluster.

## Network Observability

Retina supports [Hubble](https://github.com/cilium/hubble) as a [control plane](https://retina.sh/docs/Introduction/architecture#hubble-control-plane), which comes with CLI and UI tools to enhance BPF-powerd network observability. Below is an example Hubble UI visualization on GKE dataplane v1 (no Cilium). [See GKE network overview doc](https://cloud.google.com/kubernetes-engine/docs/concepts/network-overview).

![Hubble UI on GKE v1 dataplane (no Cilium)](./diagrams/mc-gke-hubble-ui.png)

Below is another example with Hubble CLI observing traffic on the `default` Kubernetes namespace for a GKE cluster. The instruction used in this example is `hubble observe --follow --namespace default`.

![Hubble CLI on GKE v1 dataplane (no Cilium)](./diagrams/mc-gke-hubble.png)

In addition to Hubble, Retina provides a number of Grafana dashboards which are also deployed as part of this multicloud sub-project. Below is an example of Retina DNS dashboard visualization for an EKS cluster.

![Grafana Retina DNS dashboard for EKS](./diagrams/mc-eks-grafana.png)

## Prerequisites

* OpenTofu: [installation guide](https://opentofu.org/docs/intro/install/)

* AKS:
    1. Create an Azure account.
    2. [Install az](https://learn.microsoft.com/en-us/cli/azure/install-azure-cli).

    To deploy an AKS cluster and install retina, create file `live/retina-aks/terraform.tfvars` with the Azure TenantID and SubscriptionID.

    ```sh
    # example values
    subscription_id     = "d6050d84-e4dd-463d-afc7-a6ab3dc33ab7"
    tenant_id           = "ac8a4ccd-35f1-4f95-a688-f68e3d89adfc"
    ```

* GKE:
    1. create a gcloud account, project and enable billing.
    2. create a service account and service account key.
    3. Enable Kubernetes Engine API and Identity and Access Management (IAM) API.
    4. [Install gcloud](https://cloud.google.com/sdk/docs/install).

    To deploy a GKE cluster export `GOOGLE_APPLICATION_CREDENTIALS` env variable to point to the path where your [service account key](https://cloud.google.com/iam/docs/keys-create-delete) is located.

    ```sh
    # example
    export GOOGLE_APPLICATION_CREDENTIALS=/Users/srodi/src/retina/test/multicloud/live/retina-gke/service-key.json
    ```

* EKS:
    1. Create an AWS account
    2. Create a user and assign required policies to create VPC, Subnets, Security Groups, IAM roles, EKS and workers
    3. [Install AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html)
    4. Create required `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` for the new user

    To deploy an EKS cluster export `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` as env variables.

    ```sh
    export AWS_ACCESS_KEY_ID="..."
    export AWS_SECRET_ACCESS_KEY="..."
    ```

* Grafana:
    1. Set up a [Grafana Cloud free account](https://grafana.com/pricing/) and start an instance.
    2. Create a [Service Account](https://grafana.com/docs/grafana/latest/administration/service-accounts/#create-a-service-account-in-grafana).
    3. Export `GRAFANA_AUTH` environmnet variable containing the service account token.

    ```sh
    # example
    export GRAFANA_AUTH=glsa_s0MeRan0mS7r1ng_1ab2c345
    ```

* Kind:
    1. Docker installed on the host machine

## Quickstart

The following Make targets can be used to manage each stack lifecycle.

### Create

Format code, initialize OpenTofu, plan and apply the stack to create infra and deploy retina

* AKS:

    ```sh
    make aks
    ```

* GKE:

    ```sh
    make gke
    ```

* EKS:

    ```sh
    make eks
    ```

* Kind:

    ```sh
    make kind
    ```

### Clean up

To destroy the cluster specify the `STACK_NAME` and run `make destroy`.

```sh
# destroy AKS and cleanup local state files
# set a different stack as needed (i.e. retina-gke, retina-kind)
export STACK_NAME=retina-aks
make destroy
```

### Test

The test framework is levergaing Go and [Terratest](https://terratest.gruntwork.io/docs/). To run tests:

```sh
make test
```

## Providers references

Resources documentation:

* [GKE](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/container_cluster)
* [AKS](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/kubernetes_cluster)
* [EKS](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/eks_cluster)
* [Kind](https://registry.terraform.io/providers/tehcyx/kind/latest/docs/resources/cluster)
* [Helm Release](https://registry.terraform.io/providers/hashicorp/helm/latest/docs/resources/release)
* [Kubernetes Deployment](https://registry.terraform.io/providers/hashicorp/kubernetes/latest/docs/resources/deployment)
* [Grafana Data Source](https://registry.terraform.io/providers/grafana/grafana/latest/docs/resources/data_source)

## Troubleshooting

In case the test fails due to timeout, validate the resource was created by the provider, and if it is, you can import into OpenTofu state.

Here is an example on how to import resources for `modules/gke`:

```sh
# move to the stack directory
# i.e. examples/gke
tofu import module.gke.google_container_cluster.gke europe-west2/test-gke-cluster
tofu import module.gke.google_service_account.default projects/mc-retina/serviceAccounts/test-gke-service-account@mc-retina.iam.gserviceaccount.com

# i.e. examples/eks
tofu import module.eks.aws_eks_node_group.node_group mc-test-aks:mc-test-node-group
tofu import module.eks.aws_iam_role.eks_node_group_role mc-test-eks-node-group-role
tofu import module.eks.aws_iam_role_policy_attachment.eks_node_group_AmazonEKS_CNI_Policy "mc-test-eks-node-group-role/arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy"
tofu import module.eks.aws_iam_role_policy_attachment.eks_node_group_AmazonEKSWorkerNodePolicy "mc-test-eks-node-group-role/arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy"
```

>Note: each resource documentation contains a section on how to import resources into the State. [Example for google_container_cluster resource](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/container_cluster#import).

## Multi-Cloud

The [live/](./live/) directory contains multi-cloud / multi-cluster stacks to deploy cloud infrastructure, install Retina, install Prometheus, deploy Private Datasource Connection agent in each cluster, and configure a Grafana Cloud instance to consume Prometheus data sources to visualize Retina metrics from multiple clusters in a single Grafana dashboard.

![Architecture Diagram](./diagrams/diagram-mc.svg)

In the next section we provide a multi-cloud demo to show some of Retina's capabilities when deployed in managed Kubernetes clusters on different cloud providers.

### Demo Prerequisites

Create all required resources in `Azure`, `Google Cloud` and `Amazon Web services`.

```sh
make aks
make gke
make eks
```

### Demo Utilities

The Makefile provides several utilities to help with multi-cloud demo preparation and testing:

#### Setting up Environments

```bash
# Define STACK_NAME
# One of "retina-aks", "retina-gke" or "retina-eks"
export STACK_NAME="retina-gke"

# Set the kubeconfig for the current STACK_NAME
make set-kubeconfig

# Deploy demo client and server pods
make create-pods

# Delete demo client and server pods
make delete-pods

# Observe traffic to the client pod using Hubble
make observe-client

# Restart CoreDNS pods
make restart-coredns

# Test DNS resolution for various domains
make test-dns
```

### Demo Scenarios

The demo aims to show retina capabilities on different cloud:

1. Packets drop on AKS
2. DNS failures on EKS
3. Retina captures on GKE

#### Scenario 1: Packets Dropped by iptables (AKS)

Demonstrates packet drop issues in Azure Kubernetes Service. In this scenario we manipulate network traffic in a Kubernetes environment by adding an `iptables` `DROP` rule within the client pod to block incoming communication from server.

```bash
# Initialize the AKS environment
make drop-init

# Add iptables rule to deny traffic from server to client
make drop-add

# Remove the blocking iptables rule
make drop-rm
```

#### Scenario 2: DNS Resolution Failure (EKS)

Demonstrates DNS issues in Amazon EKS. In this scenario we intentionally creates DNS resolution failures and how Retina can diagnose different types of DNS resolution failures in a controlled environment. As part of this scenario we apply three custom DNS response templates within the EKS CoreDNS configuration. Each template creates a specific DNS behavior for different domains.

```bash
# Initialize the EKS environment
make dns-init

# Configure CoreDNS to fail resolving specific domains
make dns-add

# Restore the original CoreDNS configuration
make dns-rm
```

#### Scenario 3: Packet Capture with Retina (GKE)

Demonstrates Retina's packet capture capabilities in Google Kubernetes Engine. In this scenario we demonstarte `retina capture` to capture network traffic from the client pod for troubleshooting or analysis purposes. The `capture-copy-file` target can be used afterward to extract the capture file for analysis with tools like Wireshark.

```bash
# Initialize the GKE environment
make capture-init

# Run a packet capture on the client pod
make capture-run

# Copy capture files to your local machine for analysis
make capture-copy-file
```
