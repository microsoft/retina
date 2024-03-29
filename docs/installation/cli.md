# CLI Setup

Currently, Retina CLI supports Linux, Windows, and MacOS on x86_64 and ARM64 platforms.

For CLI usage, see [Capture with Retina CLI](../captures/cli.md).

## Option 1: Install using Krew

Retina CLI is available as a [Krew plugin](https://krew.sigs.k8s.io/)! To install it, run:

```bash
kubectl krew install retina
```

## Option 2: Download the binary from the latest release on GitHub

Download the `kubectl-retina` package for your platform from the latest [Retina release](https://github.com/microsoft/retina/releases/latest).

## Option 2: Build from source

Building the CLI requires go1.21 or greater.

To build the CLI simply and quickly with Go:

```bash
go build -o bin/kubectl-retina cli/main.go
```

The release pipeline uses [GoReleaser](https://goreleaser.com/), which is the recommended way to build the CLI.

To build the CLI with GoReleaser:

```bash
goreleaser build --snapshot --clean [--single-target]
```

Note: --single-target is optional and can be used to build for the current platform only. Omit it to cross-compile for all supported platforms.
