#!/bin/bash

# Change to the retina directory
cd "$BUILD_SOURCESDIRECTORY/retina" || exit 1

# Tidy and download Go modules
go mod tidy
go mod download

# Set output path and version
OUTPUT_PATH="$BUILD_SOURCESDIRECTORY/output/retina_windows_amd64.exe"
SOURCE_FILE="./controller/main.go"
VERSION="$CDP_DEFINITION_BUILD_COUNT"

# Build the Go binary
go build -v \
  -o "$OUTPUT_PATH" \
  -gcflags="-dwarflocationlists=true" \
  -ldflags "-X github.com/microsoft/retina/internal/buildinfo.Version=$VERSION \
