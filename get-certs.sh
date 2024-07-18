#!/usr/bin/env bash

set -euo pipefail
set -x

# Directory where certificates will be stored
CERT_DIR="$(pwd)/.certs"
mkdir -p "$CERT_DIR"

declare -A CERT_FILES=(
  ["tls.crt"]="tls-client-cert-file"
  ["tls.key"]="tls-client-key-file"
  ["ca.crt"]="tls-ca-cert-files"
)

for FILE in "${!CERT_FILES[@]}"; do
  KEY="${CERT_FILES[$FILE]}"
  JSONPATH="{.data['${FILE//./\\.}']}"

  # Retrieve the secret and decode it
  kubectl get secret hubble-relay-client-certs -n kube-system \
    -o jsonpath="${JSONPATH}" | \
    base64 -d > "$CERT_DIR/$FILE"

  # Set the appropriate hubble CLI config
  hubble config set "$KEY" "$CERT_DIR/$FILE"
done

hubble config set tls true
hubble config set tls-server-name instance.hubble-relay.cilium.io
