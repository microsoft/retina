#!/bin/bash
set -euo pipefail

LLVM_VERSION=16

# Install the required tools and dependencies
sudo apt-get update && sudo apt-get install -y \
  lsb-release \
  wget \
  software-properties-common \
  gnupg \
  man-db

# Install LLVM/Clang (version must match project requirements)
curl -fsSL https://apt.llvm.org/llvm.sh -o /tmp/llvm.sh
chmod +x /tmp/llvm.sh
sudo /tmp/llvm.sh "$LLVM_VERSION"
rm /tmp/llvm.sh

# Create unversioned symlinks so the build system finds clang and llvm-strip
sudo ln -sf "/usr/bin/clang-${LLVM_VERSION}" /usr/bin/clang
sudo ln -sf "/usr/bin/llvm-strip-${LLVM_VERSION}" /usr/bin/llvm-strip

# Install gofumpt (used by make fmt)
go install mvdan.cc/gofumpt@latest
