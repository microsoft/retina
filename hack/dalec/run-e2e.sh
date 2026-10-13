#!/usr/bin/env bash
set -euo pipefail

: "${RESULTS_DIR:?}"
: "${CLUSTER_NAME:?}"
: "${AZURE_RESOURCE_GROUP:?}"
: "${AZURE_LOCATION:?}"
: "${BUILD_BUILDID:?}"
repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"
manifest="$RESULTS_DIR/candidate-images.json"
RETINA_AGENT_IMAGE="$(jq -er '.images.retinaAgentImage' "$manifest")"
RETINA_WINDOWS_AGENT_IMAGE="$(jq -er '.images.retinaWindowsAgentImage' "$manifest")"
RETINA_INIT_IMAGE="$(jq -er '.images.retinaInitImage' "$manifest")"
RETINA_OPERATOR_IMAGE="$(jq -er '.images.retinaOperatorImage' "$manifest")"
KUBECTL_RETINA_IMAGE="$(jq -er '.images.kubectlRetinaImage' "$manifest")"
KUBECTL_RETINA_SHELL_IMAGE="$(jq -er '.images.kubectlRetinaShellImage' "$manifest")"
TAG="$(jq -er '.imageVersion' "$manifest")"
export RETINA_AGENT_IMAGE RETINA_WINDOWS_AGENT_IMAGE RETINA_INIT_IMAGE RETINA_OPERATOR_IMAGE
export KUBECTL_RETINA_IMAGE KUBECTL_RETINA_SHELL_IMAGE TAG
export IMAGE_REGISTRY=mcr.microsoft.com
export IMAGE_NAMESPACE=containernetworking/v2
AZURE_SUBSCRIPTION_ID="$(az account show --query id -o tsv)"
export AZURE_SUBSCRIPTION_ID
export PATH="$repo_root/hack/dalec:$PATH"
export KUBECONFIG="$repo_root/test/e2e/test.pem"

diagnostics() {
  local status=$?
  trap - EXIT
  if [[ -f "$RESULTS_DIR/e2e.jsonl" ]]; then
    python3 hack/dalec/test-report.py "$RESULTS_DIR/e2e.jsonl" "$RESULTS_DIR/junit.xml" || {
      echo "Failed to generate the test report" >&2
      status=1
    }
  fi
  if [[ -f "$KUBECONFIG" ]]; then
    for resource in nodes pods events daemonsets deployments jobs; do
      kubectl get "$resource" --all-namespaces -o yaml > "$RESULTS_DIR/$resource.yaml" 2>&1 ||
        echo "Failed to collect $resource diagnostics" >&2
    done
    kubectl logs --namespace kube-system --selector k8s-app=retina \
      --all-containers --prefix --tail=2000 > "$RESULTS_DIR/retina.log" 2>&1 ||
      echo "Failed to collect Retina logs" >&2
  fi
  exit "$status"
}
trap diagnostics EXIT

[[ "$(az group exists --name "$AZURE_RESOURCE_GROUP")" == false ]] || {
  echo "Refusing to reuse an existing resource group" >&2
  exit 1
}
az group create --name "$AZURE_RESOURCE_GROUP" --location "$AZURE_LOCATION" \
  --tags "retinaE2ERun=$BUILD_BUILDID" -o none

az network vnet create --resource-group "$AZURE_RESOURCE_GROUP" --name testvnet \
  --address-prefixes 10.128.0.0/9 --subnet-name testsubnet \
  --subnet-prefixes 10.128.0.0/12 -o none
subnet_id="$(az network vnet subnet show --resource-group "$AZURE_RESOURCE_GROUP" \
  --vnet-name testvnet --name testsubnet --query id -o tsv)"
windows_password="Retina!$(openssl rand -hex 20)"
az aks create --resource-group "$AZURE_RESOURCE_GROUP" --name "$CLUSTER_NAME" \
  --location "$AZURE_LOCATION" --enable-managed-identity --generate-ssh-keys \
  --node-count 3 --node-vm-size Standard_D4ds_v4 --os-sku Ubuntu \
  --network-plugin azure --network-policy azure --vnet-subnet-id "$subnet_id" \
  --service-cidr 10.0.0.0/16 --dns-service-ip 10.0.0.10 \
  --windows-admin-username azureuser --windows-admin-password "$windows_password" -o none
unset windows_password
az aks nodepool add --resource-group "$AZURE_RESOURCE_GROUP" --cluster-name "$CLUSTER_NAME" \
  --name azlinux --node-count 1 --node-vm-size Standard_D4ds_v4 \
  --os-sku AzureLinux --enable-fips-image -o none
az aks nodepool add --resource-group "$AZURE_RESOURCE_GROUP" --cluster-name "$CLUSTER_NAME" \
  --name arm64 --node-count 2 --node-vm-size Standard_D4pds_v5 --os-sku Ubuntu -o none
az aks nodepool add --resource-group "$AZURE_RESOURCE_GROUP" --cluster-name "$CLUSTER_NAME" \
  --name ws22 --node-count 1 --node-vm-size Standard_D4ds_v4 \
  --os-type Windows --os-sku Windows2022 -o none

while IFS= read -r registry; do
  if [[ "$registry" == *.azurecr.io ]]; then
    registry_id="$(az acr show --name "${registry%.azurecr.io}" --query id -o tsv)"
    az aks update --resource-group "$AZURE_RESOURCE_GROUP" --name "$CLUSTER_NAME" \
      --attach-acr "$registry_id" -o none
    kubelet_id="$(az aks show --resource-group "$AZURE_RESOURCE_GROUP" --name "$CLUSTER_NAME" \
      --query identityProfile.kubeletidentity.objectId -o tsv)"
    [[ -n "$kubelet_id" ]] || { echo "AKS kubelet identity is missing" >&2; exit 1; }
    az role assignment list --assignee "$kubelet_id" --scope "$registry_id" --role AcrPull \
      --query '[].id' -o tsv >> "$RESULTS_DIR/registry-role-assignments.txt"
    az acr login --name "${registry%.azurecr.io}"
  fi
done < <(jq -r '.images[] | split("/")[0]' "$manifest" | sort -u)
az aks get-credentials --resource-group "$AZURE_RESOURCE_GROUP" --name "$CLUSTER_NAME" \
  --admin --file "$KUBECONFIG" --overwrite-existing
kubectl wait --for=condition=Ready nodes --all --timeout=10m
for platform in linux/amd64 linux/arm64 windows/amd64; do
  count="$(kubectl get nodes --selector "kubernetes.io/os=${platform%/*},kubernetes.io/arch=${platform#*/}" \
    -o json | jq '.items | length')"
  [[ "$count" -gt 0 ]] || { echo "No nodes available for $platform" >&2; exit 1; }
done

for arch in amd64 arm64; do
  for component in kubectlRetinaImage kubectlRetinaShellImage; do
    image="$(jq -er --arg key "$component" '.images[$key]' "$manifest")"
    job="cli-${component,,}-$arch"
    command='["/kubectl-retina","version"]'
    if [[ "$component" == kubectlRetinaShellImage ]]; then
      command='["/bin/sh","-ec","test -x /bin/kubectl-retina; /bin/kubectl-retina version"]'
    fi
    jq -n --arg job "$job" --arg image "$image" --arg arch "$arch" --argjson command "$command" \
      '{apiVersion:"batch/v1",kind:"Job",metadata:{name:$job},
        spec:{backoffLimit:0,template:{spec:{restartPolicy:"Never",
          nodeSelector:{"kubernetes.io/os":"linux","kubernetes.io/arch":$arch},
          containers:[{name:"cli",image:$image,command:$command}]}}}}' | kubectl apply -f -
    kubectl wait --for=condition=complete "job/$job" --timeout=5m
    version="$(kubectl logs "job/$job")"
    printf '%s\n' "$version" > "$RESULTS_DIR/$job.log"
    [[ "${version#v}" == "${TAG#v}" ]] || {
      echo "$component on $arch reported $version instead of $TAG" >&2
      exit 1
    }
  done
done

go test -json ./test/e2e/ -tags=e2e -run '^TestE2ERetina$' -count=1 -timeout=120m \
  -args -image-tag="$TAG" -image-registry="$IMAGE_REGISTRY" \
  -image-namespace="$IMAGE_NAMESPACE" -kubeConfig="$KUBECONFIG" \
  -create-infra=false -delete-infra=false \
  | tee "$RESULTS_DIR/e2e.jsonl"
