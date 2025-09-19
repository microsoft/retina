#!/bin/bash
# Script to validate that all directories with Dockerfiles are covered by dependabot configuration

set -e

REPO_ROOT=$(git rev-parse --show-toplevel)
cd "$REPO_ROOT"

echo "🔍 Validating dependabot Docker coverage..."

# Find all directories containing Dockerfiles
echo "📁 Directories with Dockerfiles:"
dockerfile_dirs=$(find . -name "Dockerfile*" -exec dirname {} \; | sort -u | sed 's|^\.|/|' | sed 's|^//|/|')
echo "$dockerfile_dirs"

echo ""

# Extract directories tracked by dependabot for Docker
echo "📋 Directories tracked in dependabot.yaml:"
dependabot_dirs=$(awk '/package-ecosystem.*docker/{getline; print}' .github/dependabot.yaml | sed 's/.*directory: "//' | sed 's/".*//' | sort)
echo "$dependabot_dirs"

echo ""

# Compare the two lists
missing_dirs=""
for dir in $dockerfile_dirs; do
    if ! echo "$dependabot_dirs" | grep -q "^$dir$"; then
        missing_dirs="$missing_dirs $dir"
    fi
done

if [ -n "$missing_dirs" ]; then
    echo "❌ VALIDATION FAILED: The following directories contain Dockerfiles but are not tracked by dependabot:"
    for dir in $missing_dirs; do
        echo "   - $dir"
    done
    echo ""
    echo "Please add these directories to .github/dependabot.yaml"
    exit 1
else
    echo "✅ VALIDATION PASSED: All directories with Dockerfiles are covered by dependabot configuration"
    echo ""
    echo "Total directories tracked: $(echo "$dependabot_dirs" | wc -l)"
fi