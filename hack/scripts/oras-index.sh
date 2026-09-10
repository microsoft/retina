#!/usr/bin/env bash
# Create a multi-arch image index from per-platform image references with oras.
#
# Usage: oras-index.sh <registry>/<repo>:<tag> <registry>/<repo>:<tag>-<platform>...
#
# A drop-in for `docker buildx imagetools create -t <index> <ref>...` on hosts
# without a docker CLI. The caller must be logged in to the registry with oras.
#
# Each platform reference resolves to the newest existing tag among <ref>-1,
# <ref>-2, ... and falls back to <ref> itself. The Azure DevOps pipeline appends
# the job attempt to the tags it pushes, because a retried job cannot push a
# tag that already exists in the registry.
set -euo pipefail
# set -e does not apply inside $(...) unless inherit_errexit is on.
shopt -s inherit_errexit

if [ "$#" -lt 2 ]; then
  echo "usage: $0 <index-ref> <platform-ref>..." >&2
  exit 2
fi

index_ref="$1"
shift

# newest_ref <ref>: the reference with the highest existing attempt suffix,
# or <ref> itself when no attempt tag exists.
newest_ref() {
  local ref="$1" n=1 found=""
  while oras manifest fetch --descriptor "$ref-$n" >/dev/null 2>&1; do
    found="$ref-$n"
    n=$((n + 1))
  done
  printf '%s\n' "${found:-$ref}"
}

# platform_digest <ref>: the digest of the platform image manifest behind <ref>.
# BuildKit pushes each per-platform tag as an OCI index that holds the image
# manifest plus an attestation manifest, and oras does not flatten nested
# indexes, so unwrap that index here.
platform_digest() {
  local ref="$1" manifest media_type
  manifest=$(oras manifest fetch "$ref") || return 1
  media_type=$(jq -er '.mediaType' <<<"$manifest") || return 1
  if [ "$media_type" = "application/vnd.oci.image.index.v1+json" ]; then
    jq -er '[.manifests[] | select(.platform.os != "unknown")][0].digest' <<<"$manifest"
  else
    oras manifest fetch --descriptor "$ref" | jq -er '.digest'
  fi
}

digests=()
for ref in "$@"; do
  ref=$(newest_ref "$ref")
  echo "using $ref"
  digests+=("$(platform_digest "$ref")")
done
oras manifest index create "$index_ref" "${digests[@]}"
