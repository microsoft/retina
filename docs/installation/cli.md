# CLI Setup

Currently, Retina CLI supports Linux, Windows, and MacOS on x86_64 and ARM64 platforms.

For CLI usage, see [Capture with Retina CLI](../captures/cli.md).

## Option 1: Download from Release

Download `kubectl-retina` from the latest [Retina release](https://github.com/microsoft/retina/releases).
Feel free to move the binary to `/usr/local/bin/`, or add it to your `PATH` otherwise.

## Option 2: Build from source

Building the CLI requires go1.21 or greater.

To build the CLI simply with Go:

```bash
go build -o bin/kubectl-retina cli/main.go
```

To cross-compile for all supported platforms, use [GoReleaser](https://goreleaser.com/):

```bash
goreleaser build --snapshot --clean
```
