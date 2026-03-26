#!/usr/bin/env bash
#
# version-docs.sh — Snapshot current docs for a new release version.
#
# Usage:
#   ./site/scripts/version-docs.sh v1.2.0
#
# This script:
#   1. Creates a Docusaurus versioned snapshot of docs/
#   2. Updates docusaurus.config.ts to make the new version the stable default
#   3. Prunes old versions beyond the retention limit
#   4. Verifies the site builds successfully
#
# After running, commit the changes and open a PR.

set -euo pipefail

# Number of versioned snapshots to keep (not counting "current"/Latest)
MAX_VERSIONS=3

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SITE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_ROOT="$(cd "$SITE_DIR/.." && pwd)"
CONFIG_FILE="$SITE_DIR/docusaurus.config.ts"
VERSIONS_FILE="$SITE_DIR/versions.json"

usage() {
  echo "Usage: $0 <version>"
  echo "  version: semver tag, e.g. v1.2.0"
  echo ""
  echo "Keeps the $MAX_VERSIONS most recent versioned snapshots."
  exit 1
}

if [[ $# -ne 1 ]]; then
  usage
fi

VERSION="$1"

# Validate version format
if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Error: version must match vX.Y.Z (got: $VERSION)"
  exit 1
fi

# Read current stable version from config
CURRENT_VERSION=$(grep -oP "lastVersion: '[^']+'" "$CONFIG_FILE" | grep -oP "'[^']+'" | tr -d "'")
if [[ -z "$CURRENT_VERSION" ]]; then
  echo "Error: could not find lastVersion in $CONFIG_FILE"
  exit 1
fi

if [[ "$CURRENT_VERSION" == "$VERSION" ]]; then
  echo "Error: $VERSION is already the current stable version"
  exit 1
fi

echo "==> Versioning docs: $CURRENT_VERSION -> $VERSION"

# Install deps if needed
if [[ ! -d "$SITE_DIR/node_modules" ]]; then
  echo "==> Installing npm dependencies..."
  (cd "$SITE_DIR" && npm ci)
fi

# Create the versioned docs snapshot
echo "==> Creating docs snapshot for $VERSION..."
(cd "$SITE_DIR" && npx docusaurus docs:version "$VERSION")

# Prune old versions beyond the retention limit
VERSIONS=($(python3 -c "import json; vs=json.load(open('$VERSIONS_FILE')); [print(v) for v in vs]"))
NUM_VERSIONS=${#VERSIONS[@]}

if [[ $NUM_VERSIONS -gt $MAX_VERSIONS ]]; then
  echo "==> Pruning old versions (keeping $MAX_VERSIONS)..."
  for (( i=MAX_VERSIONS; i<NUM_VERSIONS; i++ )); do
    OLD="${VERSIONS[$i]}"
    echo "    Removing $OLD..."
    rm -rf "$SITE_DIR/versioned_docs/version-$OLD"
    rm -f "$SITE_DIR/versioned_sidebars/version-$OLD-sidebars.json"
  done

  # Rewrite versions.json with only the kept versions
  python3 -c "
import json
versions = json.load(open('$VERSIONS_FILE'))
kept = versions[:$MAX_VERSIONS]
json.dump(kept, open('$VERSIONS_FILE', 'w'), indent=2)
print('    versions.json:', kept)
"
fi

# Update docusaurus.config.ts — replace old version references with new
echo "==> Updating docusaurus.config.ts..."
sed -i "s/lastVersion: '$CURRENT_VERSION'/lastVersion: '$VERSION'/" "$CONFIG_FILE"
sed -i "s/'$CURRENT_VERSION': {/'$VERSION': {/" "$CONFIG_FILE"
sed -i "s/label: '$CURRENT_VERSION (stable)'/label: '$VERSION (stable)'/" "$CONFIG_FILE"

# Verify the build
echo "==> Building site to verify..."
(cd "$SITE_DIR" && npm run build)

echo ""
echo "Done! Docs versioned as $VERSION."
echo ""
echo "Versions now available:"
cat "$VERSIONS_FILE"
echo ""
echo "Changed files:"
(cd "$REPO_ROOT" && git status --short site/ | head -20)
echo ""
echo "Next steps:"
echo "  1. Review the changes"
echo "  2. Commit and open a PR"
