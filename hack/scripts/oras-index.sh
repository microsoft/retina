#!/usr/bin/env bash
# Create a multi-arch image index from per-platform image references with oras.
#
# Usage: oras-index.sh <registry>/<repo>:<tag> <registry>/<repo>:<tag>-<platform>...
#
# A drop-in for `docker buildx imagetools create -t <index> <ref>...` on hosts
# without a docker CLI. The caller must be logged in to the registry with oras.
#
# Each platform reference resolves to the tag with the highest attempt suffix
# among <ref>-1, <ref>-2, ... in the registry, and falls back to <ref> itself
# when no attempt tag exists. The Azure DevOps pipeline appends the job attempt
# to the tags it pushes, because a retried job cannot push a tag that already
# exists in the registry. Attempts are not contiguous: an attempt that failed
# before its push leaves no tag.
set -euo pipefail
# set -e does not apply inside $(...) unless inherit_errexit is on.
shopt -s inherit_errexit

if [ "$#" -lt 2 ]; then
  echo "usage: $0 <index-ref> <platform-ref>..." >&2
  exit 2
fi

index_ref="$1"
shift

# newest_ref <ref>: the reference with the highest attempt suffix in the
# registry, or <ref> itself when no attempt tag exists.
#
# The registry lists tags in lexical order, so the attempt tags form one block.
# `--last <tag>-` starts the listing at that block on registries that honor it;
# awk skips any tags before the block and stops at the first tag after it. oras
# then stops on SIGPIPE (exit 141) instead of listing the rest of the
# repository. Any other oras failure fails the resolution, so a registry error
# cannot select an older attempt.
newest_ref() {
  local ref="$1" repo="${1%:*}" prefix="${1##*:}-" lines status n best=""
  set +o pipefail
  mapfile -t lines < <(
    oras repo tags --last "$prefix" "$repo" \
      | awk -v p="$prefix" 'index($0, p) == 1 { print; found = 1; next } found { exit }'
    echo "${PIPESTATUS[0]}"
  )
  set -o pipefail
  status=${lines[-1]}
  unset 'lines[-1]'
  case "$status" in
    0 | 141) ;;
    *)
      echo "listing the tags of $repo failed with exit code $status" >&2
      return 1
      ;;
  esac
  for n in "${lines[@]#"$prefix"}"; do
    [[ $n =~ ^[0-9]+$ ]] || continue
    if [ -z "$best" ] || [ "$n" -gt "$best" ]; then
      best=$n
    fi
  done
  if [ -n "$best" ]; then
    printf '%s\n' "$ref-$best"
  else
    printf '%s\n' "$ref"
  fi
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
