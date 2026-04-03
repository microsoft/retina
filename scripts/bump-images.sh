#!/usr/bin/env bash
# bump-images.sh — update all Dockerfile base image digests.
#
# Parses "# skopeo inspect docker://IMAGE:TAG ..." comments in Dockerfiles
# and replaces the sha256 digest on the following FROM line with the latest
# digest from the registry. Supports --override-os windows for Windows images.
#
# For Go builder images (golang), also checks for newer minor/patch versions
# and prompts the user to select a version before updating digests.
#
# Usage: make bump-images
# Requires: skopeo, curl, python3

set -euo pipefail

if ! command -v skopeo &>/dev/null; then
  echo "error: skopeo is required but not installed" >&2
  exit 1
fi

REPO_ROOT="$(git rev-parse --show-toplevel)"
DOCKERFILES=$(find "$REPO_ROOT" -name "Dockerfile*" -not -path "*/.git/*" -not -path "*/vendor/*")

# --- Go version bump detection ---
# Scan Dockerfiles for Go builder images and check for newer versions.

go_image_repo=""
current_go_version=""

for dockerfile in $DOCKERFILES; do
  if match=$(grep -oP 'docker://\S+/golang:\K[0-9]+\.[0-9]+\.[0-9]+' "$dockerfile" | head -1); then
    if [[ -n "$match" ]]; then
      current_go_version="$match"
      go_image_repo=$(grep -oP 'docker://\K\S+/golang(?=:)' "$dockerfile" | head -1)
      break
    fi
  fi
done

if [[ -n "$current_go_version" && -n "$go_image_repo" ]]; then
  echo "Current Go builder version: $current_go_version ($go_image_repo)"

  # Parse current version components
  IFS='.' read -r cur_major cur_minor cur_patch <<< "$current_go_version"

  # Fetch available tags from MCR API
  # Convert image repo to registry API path (mcr.microsoft.com/foo/bar → /v2/foo/bar/tags/list)
  registry_host=$(echo "$go_image_repo" | cut -d/ -f1)
  repo_path=$(echo "$go_image_repo" | cut -d/ -f2-)
  tags_json=$(curl -s "https://${registry_host}/v2/${repo_path}/tags/list" 2>/dev/null || true)

  if [[ -n "$tags_json" ]]; then
    # Extract unique Go versions (MAJOR.MINOR.PATCH only, no suffix) that are newer
    newer_versions=$(echo "$tags_json" | python3 -c "
import sys, json

data = json.load(sys.stdin)
tags = data.get('tags', [])

cur_major, cur_minor, cur_patch = ${cur_major}, ${cur_minor}, ${cur_patch}
seen = set()
results = []

for tag in tags:
    # Match tags that are exactly MAJOR.MINOR.PATCH (no suffix)
    parts = tag.split('.')
    if len(parts) != 3:
        continue
    try:
        major, minor, patch = int(parts[0]), int(parts[1]), int(parts[2])
    except ValueError:
        continue

    version = (major, minor, patch)
    if version in seen:
        continue
    seen.add(version)

    # Only include versions newer than current, same major
    if major != cur_major:
        continue
    if (minor, patch) <= (cur_minor, cur_patch):
        continue
    results.append(version)

for v in sorted(results):
    kind = 'minor' if v[1] > cur_minor else 'patch'
    print(f'{v[0]}.{v[1]}.{v[2]} ({kind})')
" 2>/dev/null || true)

    if [[ -n "$newer_versions" ]]; then
      echo ""
      echo "Newer Go versions available:"
      i=1
      versions_array=()
      while IFS= read -r line; do
        echo "  $i) $line"
        # Extract just the version number (before the space)
        versions_array+=("${line%% *}")
        ((i++))
      done <<< "$newer_versions"
      echo "  0) Keep current ($current_go_version)"
      echo ""

      if [[ -t 0 ]]; then
        read -r -p "Select version to bump to [0]: " choice
      else
        echo "Non-interactive mode, keeping current version."
        choice=0
      fi
      choice="${choice:-0}"

      if [[ "$choice" =~ ^[0-9]+$ ]] && (( choice >= 1 && choice <= ${#versions_array[@]} )); then
        new_go_version="${versions_array[$((choice - 1))]}"
        echo ""
        echo "Bumping Go version: $current_go_version → $new_go_version"

        # Update all Dockerfiles: replace old Go version with new in golang image tags
        for dockerfile in $DOCKERFILES; do
          # Update skopeo comment lines and FROM lines referencing the golang image
          if grep -q "golang:${current_go_version}" "$dockerfile"; then
            sed -i "s|golang:${current_go_version}|golang:${new_go_version}|g" "$dockerfile"
            rel_path="${dockerfile#$REPO_ROOT/}"
            echo "  GO    $rel_path: ${current_go_version} → ${new_go_version}"
          fi
        done
        echo ""
      else
        echo "Keeping Go version at $current_go_version."
        echo ""
      fi
    else
      echo "No newer Go versions found."
      echo ""
    fi
  fi
fi

# --- Digest bump loop ---

updated=0
skipped=0

for dockerfile in $DOCKERFILES; do
  # Find all skopeo comment lines
  while IFS= read -r line_num; do
    # Extract the skopeo command from the comment
    comment=$(sed -n "${line_num}p" "$dockerfile")

    # Parse image reference from: # skopeo inspect docker://IMAGE:TAG [--override-os windows] ...
    image_ref=$(echo "$comment" | grep -oP 'docker://\S+' | sed 's/docker:\/\///')
    override_os=$(echo "$comment" | grep -oP '\-\-override-os \S+' || true)

    if [[ -z "$image_ref" ]]; then
      continue
    fi

    # Get current digest from the FROM line (next non-empty line after comment)
    from_line=$((line_num + 1))
    current=$(sed -n "${from_line}p" "$dockerfile" | grep -oP 'sha256:[a-f0-9]+' || true)

    if [[ -z "$current" ]]; then
      continue
    fi

    # Fetch latest digest
    latest=$(skopeo inspect "docker://${image_ref}" $override_os --format "{{.Digest}}" 2>/dev/null || true)

    if [[ -z "$latest" ]]; then
      echo "SKIP  $image_ref (inspect failed)" >&2
      ((skipped++)) || true
      continue
    fi

    if [[ "$current" == "$latest" ]]; then
      ((skipped++)) || true
      continue
    fi

    # Replace old digest with new in the entire file (handles multi-use of same digest)
    sed -i "s|${current}|${latest}|g" "$dockerfile"
    rel_path="${dockerfile#$REPO_ROOT/}"
    echo "BUMP  $rel_path: $image_ref"
    echo "      ${current:0:19}... → ${latest:0:19}..."
    ((updated++)) || true

  done < <(grep -n "skopeo inspect" "$dockerfile" | cut -d: -f1)
done

echo ""
echo "Done: $updated updated, $skipped unchanged"
