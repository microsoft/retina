# Retina - eBPF Network Observability Platform

Retina is a cloud-agnostic, open-source Kubernetes network observability platform that provides centralized monitoring for application health, network health, and security. Built with Go, eBPF, and container technologies for Linux and Windows environments.

Always reference these instructions first and fallback to search or bash commands only when you encounter unexpected information that does not match the info here.

## Working Effectively

### Environment Setup
- Install required dependencies:
  - `sudo apt update && sudo apt install -y clang llvm-strip-18 jq`
  - `sudo ln -sf /usr/bin/llvm-strip-18 /usr/bin/llvm-strip` (if needed)
- Go 1.24.6+ required (check with `go version`)
- Docker and Helm required for container operations
- `clang` and `llvm-strip` are CRITICAL for eBPF compilation

### Core Build Commands
- `make retina` -- builds retina binary -- takes ~1 minute (includes eBPF generation). NEVER CANCEL. Set timeout to 10+ minutes.
- `make retina-capture-workload` -- builds capture workload binary -- takes ~2 seconds
- CLI: `cd cli && go build -o ../output/linux_amd64/retina/kubectl-retina .` -- takes ~6 seconds
- `make clean` -- clean build artifacts -- takes < 1 second

### Testing and Validation
- `make test` -- runs full test suite -- takes 10+ minutes. NEVER CANCEL. Set timeout to 20+ minutes.
- Basic package tests: `go test -timeout 10m -tags=unit ./pkg/...` -- takes ~6 minutes with some expected failures in cross-platform code
- `make fmt` -- format code -- takes ~1 second
- `make lint` -- runs linting -- takes ~2-3 minutes. May show some issues in generated mock files that can be ignored.

### BPF Generation and Plugins
- BPF generation for current architecture works: generates `.o` and `.go` files for eBPF programs
- Cross-compilation (ARM64) may fail in development environment - this is expected
- Plugin test example: `cd test/plugin && make test-packetforward` -- builds and runs plugin tests with sudo
- Individual plugins can be tested but require network traffic to show meaningful data

## Validation Scenarios

### Basic Functionality Validation
Always test these core workflows after making changes:

1. **Build and run agent**:
   ```bash
   make retina
   ./output/linux_amd64/retina/retina --help
   ```

2. **Build and test CLI**:
   ```bash
   cd cli && go build -o ../output/linux_amd64/retina/kubectl-retina .
   ./output/linux_amd64/retina/kubectl-retina --help
   ./output/linux_amd64/retina/kubectl-retina version
   ```

3. **Test plugin functionality**:
   ```bash
   cd test/plugin && make test-packetforward
   # This will run until Ctrl+C - expect to see "Start collecting packet forward metrics"
   ```

## Critical Build Timing and Warnings

### NEVER CANCEL - Build Time Expectations
- **Main binary build (`make retina`)**: Takes 1-2 minutes including eBPF generation. NEVER CANCEL. Use timeout 10+ minutes.
- **Full test suite (`make test`)**: Takes 10-15 minutes. NEVER CANCEL. Use timeout 20+ minutes.
- **Linting (`make lint`)**: Takes 2-3 minutes. NEVER CANCEL. Use timeout 5+ minutes.
- **Plugin tests**: Can run indefinitely waiting for network traffic - this is expected behavior.

### Expected Build Issues
- Cross-compilation for ARM64 may fail with "exec format error" - this is expected in development environment
- Some unit tests fail due to missing kubebuilder/etcd dependencies - this is expected
- Lint warnings about generated mock files are expected and can be ignored
- BPF compilation warnings about operator precedence are expected and harmless

## Project Structure

### Key Directories
- `/pkg/plugin/` - eBPF plugins for network observability (conntrack, dropreason, packetforward, etc.)
- `/controller/` - Main retina agent controller code
- `/cli/` - kubectl-retina CLI implementation  
- `/captureworkload/` - Network capture workload implementation
- `/operator/` - Kubernetes operator code
- `/test/plugin/` - Individual plugin test utilities
- `/docs/08-Contributing/02-development.md` - Detailed development guide

### Important Files
- `Makefile` - Primary build system with comprehensive targets
- `go.mod` - Go 1.24.6, extensive Kubernetes and eBPF dependencies
- `.devcontainer/` - GitHub Codespaces configuration with required tools
- `.github/workflows/` - CI/CD pipelines for testing and building

## Common Development Tasks

### Before Committing Changes
Always run these commands before pushing:
1. `make fmt` - Format code (required for CI)
2. `make lint` - Check code quality (may show expected warnings)
3. `make retina` - Ensure main binary builds correctly
4. Test CLI: `cd cli && go build .` - Ensure CLI builds

### Working with eBPF Code
- eBPF source files are in `pkg/plugin/*/\_cprog/` directories
- Generated files: `*_bpfel_x86.o` and `*_bpfel_x86.go` 
- BPF generation happens automatically during build
- Some compiler warnings about operator precedence are expected and harmless

### Container and Helm Operations
- `make retina-image` - Build container image (requires registry access)
- Images are published to GHCR (GitHub Container Registry)
- Helm charts are available in the repository for deployment
- Full deployment requires Kubernetes cluster with appropriate permissions

## Debugging and Troubleshooting

### Common Issues
- **"exec format error"**: Usually cross-compilation issue, rebuild for current arch
- **Missing kubebuilder**: Some tests require Kubernetes test environment
- **Plugin tests hanging**: Normal behavior - plugins wait for network traffic
- **Lint failures**: Check if they're in generated mock files (can be ignored)
- **BPF compilation warnings**: Expected and generally harmless

### Log Analysis
- Agent logs provide detailed eBPF and networking information
- CLI operations are verbose and show capture job creation details
- Plugin tests show real-time network metrics when traffic is available

This is a complex, enterprise-grade networking platform. Take time to understand the eBPF integration and Kubernetes-native architecture before making changes.