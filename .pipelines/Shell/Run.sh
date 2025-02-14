#!/bin/bash
# Azure Container Registry URL
ACR_URL="$PROD_CONTAINER_REGISTRY/$IMAGE_NAMESPACE"
IMAGE_NAME="multi_arch_image.tar"

echo "Target ACR_URL=${ACR_URL}"

tdnf upgrade -y && tdnf install -y wget gzip

echo "setting XDG_RUNTIME_DIR"
export XDG_RUNTIME_DIR=/run/user/$(id -u)
echo $XDG_RUNTIME_DIR

#config all the necessary access
echo "Login cli using managed identity"
az login --identity
echo "Getting ACR credentials"
TOKEN_QUERY_RES=$(az acr login -n $PROD_CONTAINER_REGISTRY -t)
TOKEN=$(echo "$TOKEN_QUERY_RES" | jq -r '.accessToken')
DESTINATION_ACR=$(echo "$TOKEN_QUERY_RES" | jq -r '.loginServer')
echo "DESTINATION_ACR: $DESTINATION_ACR"
crane auth login "$DESTINATION_ACR" -u "00000000-0000-0000-0000-000000000000" -p "$TOKEN"
if [ $? -ne 0 ]; then
    echo "Failed to authenticate with crane."
    exit 1
fi

TMP_FOLDER=$(mktemp -d)
cd $TMP_FOLDER

# get the tar file 
echo "Downloading docker tarball image from $PROD_CONTAINER_REGISTRY."
wget -O $IMAGE_NAME "$TARDIRECTORY"
if [ $? -ne 0 ]; then
    echo "Failed to download docker tarball image."
    exit 1
fi

#reomove all the uncessary .tar files to resolve conflict
tar -xvf $IMAGE_NAME -C $TMP_FOLDER
rm -f $IMAGE_NAME

# create manifest directory
cd ./multi_arch_image
mkdir -p manifests

# Function to extract image name and tag from file name
extract_image_details() {
    local file_name="$1"
    local name_tag="${file_name%.tar}"
    local name="${name_tag%-*-*}"
    local tag="${name_tag#*-*-}"
    
    if [[ $name == *"-"* ]]; then
        name="${name%-*}"
    fi
    
    # Handle the case for Windows image names/tags
    if [[ $name_tag == *"windows"* ]]; then
        name="${name_tag%-*-*-*-*}"
        tag="${name_tag#*-*-}"
    fi

    echo "$name:$tag"
}

# Traverse the directory and process each tarball
find . -type f -name "*.tar" | while read -r tarball; do
    # Extract image name and tag
    image_details=$(extract_image_details "$(basename "$tarball")")

    # Use crane to push image to the Azure Container Registry
    crane push "$tarball" "$ACR_URL/$image_details"
    
    # Extract the image name and git sha tag
    image_name="${image_details%:*}"
    
    # Check if a manifest for this image name already exists
    if [[ ! -f "./manifests/$image_name.txt" ]]; then
        touch "./manifests/$image_name.txt"
    fi
    
    # Add the image details to the manifest file
    echo "$ACR_URL/$image_details" >> "./manifests/$image_name.txt"
done

find ./manifests -type f -name "*.txt" | while read -r manifest_file; do
    # Read the image details from the manifest file into an array
    mapfile -t image_details_array < "$manifest_file"
    
    # Extract the git SHA tag by removing architecture
    git_sha_tag="${image_details_array[0]#*:}"
    git_sha_tag="${git_sha_tag%-*-*}" 

    # Extract the image name from the manifest file name
    image_name=$(basename "$manifest_file" .txt)

    # Define the full destination image name (without architecture)
    DEST_IMAGE_FULL_NAME="$ACR_URL/$image_name:$git_sha_tag"

    # Construct the `crane index append` command dynamically
    crane index append --docker-empty-base \
        $(for image_detail in "${image_details_array[@]}"; do 
            echo "-m $image_detail"
        done) \
        -t "$DEST_IMAGE_FULL_NAME"
done