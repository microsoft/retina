#!/usr/bin/env bash
# Preserve Windows os.version when composing an index from single-platform tags.
set -euo pipefail

if [ "$#" -lt 2 ]; then
  echo "usage: $0 <index-ref> <platform-ref>..." >&2
  exit 2
fi

index_ref="$1"
shift
index=$(docker buildx imagetools create --dry-run "$@")
jq -e '.manifests | type == "array"' <<<"$index" > /dev/null

if ! jq -e 'any(.manifests[]; .platform.os == "windows")' <<<"$index" > /dev/null; then
  exec docker buildx imagetools create -t "$index_ref" "$@"
fi

directory=$(mktemp -d)
files=()
cleanup() {
  if [ "${#files[@]}" -gt 0 ]; then
    rm -f "${files[@]}"
  fi
  rmdir "$directory"
}
trap cleanup EXIT

mapfile -t descriptors < <(jq -c '.manifests[]' <<<"$index")
args=()
for descriptor in "${descriptors[@]}"; do
  if [ "$(jq -r '.platform.os' <<<"$descriptor")" = "windows" ]; then
    digest=$(jq -er '.digest' <<<"$descriptor")
    config=$(docker buildx imagetools inspect "${index_ref%:*}@$digest" --format '{{json .Image}}')
    if ! platform=$(jq -e '
      {os, architecture, "os.version": .["os.version"]} |
      select(.os == "windows" and (.architecture | type == "string") and
             (.["os.version"] | type == "string" and length > 0))
    ' <<<"$config"); then
      echo "Windows image $digest is missing valid platform or os.version metadata" >&2
      exit 1
    fi
    descriptor=$(jq --argjson platform "$platform" '.platform += $platform' <<<"$descriptor")
  fi
  file="$directory/descriptor-${#files[@]}.json"
  files+=("$file")
  printf '%s\n' "$descriptor" > "$file"
  args+=(-f "$file")
done

docker buildx imagetools create -t "$index_ref" "${args[@]}"
