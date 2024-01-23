#!/bin/bash

set -o xtrace
export SUBSCRIPTION=9b8218f9-902a-4d20-a65c-e98acec5362f
export LOCATION=eastus
export VERSION=4
export BASE_NAME=$USER-aks
SSH_KEY_FILE_PATH="~/.ssh/$USER-ssh.id_rsa.pub" # Depends on you. Or you can use --generate-ssh-keys
WINDOWS_USERNAME="azureuser" # Recommend azureuser
export PASSWORD=randOmPassword123@

#  upgrade az cli to the latest version
az upgrade -y

# Select the oldest patch version in the latest minor version from AKS support version list.
K8S_VERSION=$(az aks get-versions -l $LOCATION -o json | jq '.values |sort_by(.version) | first.patchVersions ' | jq 'keys|.[0]' --sort-keys | tr -d '"')
echo "$K8S_VERSION is selected from region $LOCATION"

echo "creating azmon+grafana workspace"

# deploy azure monitor workspace
AZMON_NAME=$BASE_NAME-azmon-$VERSION
AZMON_RESOURCE_GROUP=$AZMON_NAME
az group create --location $LOCATION --name $AZMON_RESOURCE_GROUP
az resource create --resource-group $AZMON_RESOURCE_GROUP --namespace microsoft.monitor --resource-type accounts --name $AZMON_NAME --location $LOCATION --properties {}

GRAFANA_NAME=$BASE_NAME-grafana-$VERSION
az grafana create --name $GRAFANA_NAME --resource-group $AZMON_RESOURCE_GROUP

echo "creating ws22 cluster"

# Windows Server 2022
NAME=$BASE_NAME-grafana-$VERSION-retina-ws22
RESOURCE_GROUP=$NAME
az group create --location $LOCATION --name $RESOURCE_GROUP

# Update aks-preview to the latest version
az extension add --name aks-preview
az extension update --name aks-preview

# Enable Microsoft.ContainerService/AKSWindows2022Preview
az feature register --namespace Microsoft.ContainerService --name AKSWindows2022Preview
az provider register -n Microsoft.ContainerService

az group create --name $RESOURCE_GROUP --location $LOCATION

az aks create \
    --resource-group $RESOURCE_GROUP \
    --name $NAME \
     --generate-ssh-keys \
    --windows-admin-username $WINDOWS_USERNAME \
    --windows-admin-password $PASSWORD \
    --kubernetes-version $K8S_VERSION \
    --network-plugin azure \
    --vm-set-type VirtualMachineScaleSets \
    --node-count 1

# Set variables for Windows 2022 node pool
myWindowsNodePool="nwin22" # Length <= 6
az aks nodepool add \
    --resource-group $RESOURCE_GROUP \
    --cluster-name $NAME \
    --name $myWindowsNodePool \
    --os-type Windows \
    --os-sku Windows2022 \
    --node-count 1

# Set variables for Windows 2019 node pool
myWindowsNodePool="nwin19" # Length <= 6
az aks nodepool add \
    --resource-group $RESOURCE_GROUP \
    --cluster-name $NAME \
    --name $myWindowsNodePool \
    --os-type Windows \
    --os-sku Windows2019 \
    --node-count 1

az aks get-credentials -g $RESOURCE_GROUP -n $NAME --overwrite-existing

kubectl apply -f ama-metrics-settings-configmap.yaml

az aks update --enable-azuremonitormetrics -n $NAME -g $RESOURCE_GROUP --azure-monitor-workspace-resource-id /subscriptions/$SUBSCRIPTION/resourcegroups/$AZMON_RESOURCE_GROUP/providers/microsoft.monitor/accounts/$AZMON_NAME --grafana-resource-id  /subscriptions/9b8218f9-902a-4d20-a65c-e98acec5362f/resourceGroups/$AZMON_RESOURCE_GROUP/providers/Microsoft.Dashboard/grafana/$GRAFANA_NAME

echo "creating azcni overlay cluster"

# create azcni overlay cluster
NAME=$BASE_NAME-grafana-$VERSION-retina-linux
RESOURCE_GROUP=$NAME
az group create --location $LOCATION --name $RESOURCE_GROUP

# Create a VNet with a subnet for nodes and a subnet for pods
az network vnet create -g $RESOURCE_GROUP --location $LOCATION --name $NAME-vnet --address-prefixes 10.0.0.0/8 -o none 
az network vnet subnet create -g $RESOURCE_GROUP --vnet-name $NAME-vnet --name nodesubnet --address-prefixes 10.240.0.0/16 -o none

az aks create -n $NAME -g $RESOURCE_GROUP -l $LOCATION \
  --max-pods 250 \
  --node-count 2 \
  --network-plugin azure \
  --network-plugin-mode overlay \
  --kubernetes-version $K8S_VERSION \
  --pod-cidr 192.168.0.0/16 \
  --vnet-subnet-id /subscriptions/$SUBSCRIPTION/resourceGroups/$RESOURCE_GROUP/providers/Microsoft.Network/virtualNetworks/$NAME-vnet/subnets/nodesubnet 

az aks get-credentials -n $NAME -g $RESOURCE_GROUP --subscription $SUBSCRIPTION

kubectl apply -f ama-metrics-settings-configmap.yaml

az aks update --enable-azuremonitormetrics -n $NAME -g $RESOURCE_GROUP --azure-monitor-workspace-resource-id /subscriptions/$SUBSCRIPTION/resourcegroups/$AZMON_RESOURCE_GROUP/providers/microsoft.monitor/accounts/$AZMON_NAME --grafana-resource-id  /subscriptions/9b8218f9-902a-4d20-a65c-e98acec5362f/resourceGroups/$AZMON_RESOURCE_GROUP/providers/Microsoft.Dashboard/grafana/$GRAFANA_NAME

echo "complete, todo: install unified retina linux/windows helm chart, and follow windows installation steps in /windows/readme.md"
