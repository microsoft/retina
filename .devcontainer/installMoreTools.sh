#!/bin/bash

# Install the required tools and dependencies
sudo apt-get update && sudo apt-get install lsb-release wget software-properties-common gnupg -y

# Install LLVM 14
export LLVM_VERSION=14
curl -sL https://apt.llvm.org/llvm.sh | sudo bash -s "$LLVM_VERSION"
