#!/bin/bash

# Add kind-registry in daemon.json
sudo cp .devcontainer/daemon.json /etc/docker/daemon.json && sudo pkill -SIGHUP dockerd

# Check if the registry container already exists
if [ "$(docker ps -aq -f name=kind-registry)" ]; then
    # Remove the existing registry container if it exists
    docker rm -f kind-registry
fi

# Create a local Docker registry without TLS
docker run -d -p 5000:5000 --name kind-registry registry:2

# Get the inet IP address of docker0
REGISTRY_IP=$(ip addr show docker0 | grep 'inet ' | awk '{print $2}' | cut -d'/' -f1)

# Check if a kind cluster named "kind" already exists
if kind get clusters | grep -q "^kind$"; then
    # Delete the existing kind cluster
    kind delete cluster --name kind
fi

# Create a kind cluster configuration file
cat <<EOF | kind create cluster --name kind --verbosity 4 --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."$REGISTRY_IP:5000"]
    endpoint = ["http://$REGISTRY_IP:5000"]
nodes:
- role: control-plane
EOF

# Connect the registry to the kind cluster network
docker network connect "kind" "kind-registry"

# Wait for the kind cluster to be ready
echo "Waiting for kind cluster to be ready..."
kubectl wait --for=condition=Ready nodes --all --timeout=300s
