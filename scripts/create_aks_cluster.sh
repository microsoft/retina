set -o xtrace
set -e

regions=(eastus eastus2 centralus southcentralus westcentralus northcentralus westus westus2)
rand_index=$((RANDOM % ${#regions[@]}))

export LOCATION=${regions[$rand_index]}
export BASE_NAME=ret-$3

#  upgrade az cli to the latest version
az upgrade -y

# Select the oldest patch version in the latest minor version from AKS support version list.
K8S_VERSION=$(az aks get-versions -l $LOCATION -o json | jq '.values |sort_by(.version) | first.patchVersions ' | jq 'keys|.[0]' --sort-keys | tr -d '"')
echo "$K8S_VERSION is selected from region $LOCATION"


echo "creating AKS cluster with linux amd64 node pool"
echo "Image tag: $1"

RESOURCE_GROUP=$BASE_NAME-$1-$2
RESOURCE_GROUP=${RESOURCE_GROUP//./-}

CLUSTER_NAME=$BASE_NAME-$1-$2
CLUSTER_NAME=${CLUSTER_NAME//./-}

# Update aks-preview to the latest version
az extension add --name aks-preview
az extension update --name aks-preview

az provider register -n Microsoft.ContainerService

az group create --name $RESOURCE_GROUP --location $LOCATION

az aks create \
    --resource-group $RESOURCE_GROUP \
    --name $CLUSTER_NAME \
    --generate-ssh-keys \
    --kubernetes-version $K8S_VERSION \
    --network-plugin azure \
    --vm-set-type VirtualMachineScaleSets \
    --node-count 1 \
    --network-policy azure


sleep 60s

# Function to add node pools to aks cluster previously create
function add_node_pool() {
    local name=$1
    local vm_size=$2
    local os_type=$3
    local os_sku=$4
    
    echo "Adding $name $os_type $os_sku node pool"

    if [ -n "$vm_size" ]; then
        set +e # Ignore error on arm64 node pool
        az aks nodepool add \
            --resource-group $RESOURCE_GROUP \
            --cluster-name $CLUSTER_NAME \
            --name $name \
            --node-count 1 \
            --os-type $os_type \
            --os-sku $os_sku \
            --node-vm-size $vm_size
        set -e
    else
        az aks nodepool add \
            --resource-group $RESOURCE_GROUP \
            --cluster-name $CLUSTER_NAME \
            --name $name \
            --node-count 1 \
            --os-type $os_type \
            --os-sku $os_sku
    fi
}

add_node_pool mariner Standard_D4pds_v5 Linux AzureLinux
add_node_pool arm64 Standard_D4pds_v5 Linux Ubuntu
add_node_pool nwin22 "" Windows Windows2022
add_node_pool nwin19 "" Windows Windows2019
