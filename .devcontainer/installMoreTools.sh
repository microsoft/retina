#!/bin/bash

# Install the required tools and dependencies
sudo apt-get update && sudo apt-get install -y lsb-release wget software-properties-common gnupg clang-14 lldb-14 lld-14 clangd-14 man-db
export PATH=$PATH:/usr/lib/llvm-14/bin

# Install LLVM 14
export LLVM_VERSION=14
curl -sL https://apt.llvm.org/llvm.sh | sudo bash -s "$LLVM_VERSION"
